package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/atotto/clipboard"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const clipboardClearDelay = 60 * time.Second
const maxBackupsPerVault = 10

// App is the root application object exposed to the frontend.
type App struct {
	ctx       context.Context
	vaultMu   sync.RWMutex
	vault     *Vault
	vaultFile string
	vaultName string
	clipMu    sync.Mutex
	clipStop  chan struct{}
	qaMu      sync.Mutex
	qaOpen    bool
	qaPrevW   int
	qaPrevH   int
	qaHotkey  bool
	syncMu    sync.Mutex
	syncStatus SyncStatus
	localAPIMu sync.Mutex
	localAPI   *localAPIServer
}

var errQuickAccessUnsupported = errors.New("quick access disponível apenas no Windows")

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// Global hotkey is ON by default; OpenVault re-checks the vault setting.
	a.qaMu.Lock()
	_ = a.enableQuickAccessHotkey()
	a.qaMu.Unlock()
	// Background sync ticker (60s): no-ops when locked or unconfigured.
	go a.syncScheduler()
}

// ---------- Sync ----------

// buildSyncProvider constructs the configured provider for the current vault.
func (a *App) buildSyncProvider(v *Vault) (SyncProvider, string, error) {
	provider, err := v.getSetting("sync_provider")
	if err != nil {
		return nil, "", err
	}
	remote, err := v.getSetting("sync_remote")
	if err != nil {
		return nil, "", err
	}
	switch provider {
	case "local":
		if remote == "" {
			return nil, "", errors.New("pasta de sincronização não configurada")
		}
		return &localProvider{dir: remote}, "", nil
	default:
		return nil, "", errors.New("sincronização não configurada")
	}
}

// GetSyncConfig returns the current vault's sync provider and remote.
func (a *App) GetSyncConfig() (map[string]string, error) {
	v := a.currentVault()
	if v == nil {
		return nil, ErrVaultLocked
	}
	provider, _ := v.getSetting("sync_provider")
	remote, _ := v.getSetting("sync_remote")
	return map[string]string{"provider": provider, "remote": remote}, nil
}

// SetSyncConfig configures sync for the current vault. Provider "" disables.
// For "local", remote is a folder path.
func (a *App) SetSyncConfig(provider, remote string) error {
	v := a.currentVault()
	if v == nil {
		return ErrVaultLocked
	}
	if provider != "" && provider != "local" {
		return errors.New("provedor desconhecido")
	}
	if provider == "local" {
		info, err := os.Stat(remote)
		if err != nil || !info.IsDir() {
			return errors.New("pasta local inválida")
		}
	}
	if err := v.setSetting("sync_provider", provider); err != nil {
		return err
	}
	if err := v.setSetting("sync_remote", remote); err != nil {
		return err
	}
	// reset state so the next run does a full compare
	_ = os.Remove(syncStatePath(v.path))
	a.setSyncStatus(SyncStatus{Configured: provider != "", Provider: provider, Remote: remote, State: "idle"})
	return nil
}

// DisconnectSync disables sync for the current vault.
func (a *App) DisconnectSync() error {
	return a.SetSyncConfig("", "")
}

// GetSyncStatus returns the last known sync status.
func (a *App) GetSyncStatus() SyncStatus {
	a.syncMu.Lock()
	defer a.syncMu.Unlock()
	return a.syncStatus
}

func (a *App) setSyncStatus(s SyncStatus) {
	a.syncMu.Lock()
	defer a.syncMu.Unlock()
	a.syncStatus = s
}

// SyncNow runs one sync cycle for the current vault.
func (a *App) SyncNow() (string, error) {
	v := a.currentVault()
	if v == nil {
		return "", ErrVaultLocked
	}
	a.vaultMu.RLock()
	file := a.vaultFile
	a.vaultMu.RUnlock()
	provider, remote, err := a.buildSyncProvider(v)
	if err != nil {
		return "", err
	}
	if remote == "" {
		remote = file
	}
	localPath := filepath.Join(a.VaultDir(), file)

	// copy the key for a possible reload after download-swap (always wiped)
	key := append([]byte{}, v.vaultKey...)
	defer wipe(key)

	engine := &SyncEngine{provider: provider, remote: remote}
	result, err := engine.syncFile(localPath,
		func() (syncState, error) { return loadSyncState(localPath) },
		func(s syncState) error { return saveSyncState(localPath, s) },
		func() error { return a.reloadVaultAfterSync(file, key) },
	)
	status := SyncStatus{Configured: true, Provider: provider.Name(), State: "ok", LastSync: time.Now().UnixMilli()}
	if err != nil {
		status.State = "error"
		status.Detail = err.Error()
	} else {
		status.Detail = result
		if strings.HasPrefix(result, "conflict") {
			status.State = "conflict"
			status.Conflict = result
		}
	}
	// refresh cached status fields
	a.setSyncStatus(status)
	if err != nil {
		return "", err
	}
	return result, nil
}

// reloadVaultAfterSync reopens the swapped vault file with the in-memory key.
func (a *App) reloadVaultAfterSync(file string, key []byte) error {
	defer wipe(key)
	a.vaultMu.Lock()
	defer a.vaultMu.Unlock()
	if a.vault != nil {
		a.vault.close()
	}
	v, err := openVaultWithKey(filepath.Join(a.VaultDir(), file), key)
	if err != nil {
		a.vault, a.vaultFile, a.vaultName = nil, "", ""
		return err
	}
	a.vault = v
	return nil
}

// syncScheduler ticks every 60s and syncs when unlocked + configured.
func (a *App) syncScheduler() {
	t := time.NewTicker(60 * time.Second)
	defer t.Stop()
	for range t.C {
		v := a.currentVault()
		if v == nil {
			continue
		}
		provider, _ := v.getSetting("sync_provider")
		if provider == "" {
			continue
		}
		if _, err := a.SyncNow(); err != nil {
			// status already recorded; keep ticking
			continue
		}
	}
}

// ---------- Quick Access (global hotkey popup) ----------

// enableQuickAccessHotkey registers Ctrl+Shift+Space once and starts the
// listener loop. Caller must hold qaMu.
func (a *App) enableQuickAccessHotkey() error {
	if a.qaHotkey {
		return nil
	}
	if err := registerQuickAccessHotkey(); err != nil {
		return err
	}
	a.qaHotkey = true
	go quickAccessLoop(func() {
		a.openQuickAccess()
	})
	return nil
}

// disableQuickAccessHotkey unregisters the global hotkey.
// Caller must hold qaMu.
func (a *App) disableQuickAccessHotkey() {
	if !a.qaHotkey {
		return
	}
	unregisterQuickAccessHotkey()
	a.qaHotkey = false
}

// SetQuickAccess enables/disables the global Ctrl+Shift+Space popup.
func (a *App) SetQuickAccess(enabled bool) error {
	v := a.currentVault()
	if v == nil {
		return ErrVaultLocked
	}
	val := "1"
	if !enabled {
		val = "0"
	}
	if err := v.setSetting("quick_access", val); err != nil {
		return err
	}
	a.qaMu.Lock()
	defer a.qaMu.Unlock()
	if enabled {
		return a.enableQuickAccessHotkey()
	}
	a.disableQuickAccessHotkey()
	return nil
}

// openQuickAccess summons the mini search popup (or just shows the window
// when locked). Called from the hotkey thread.
func (a *App) openQuickAccess() {
	if a.ctx == nil {
		return
	}
	if !a.IsUnlocked() {
		runtime.WindowShow(a.ctx)
		runtime.WindowUnminimise(a.ctx)
		return
	}
	a.qaMu.Lock()
	if a.qaOpen {
		a.qaMu.Unlock()
		runtime.WindowShow(a.ctx)
		return
	}
	w, h := runtime.WindowGetSize(a.ctx)
	a.qaPrevW, a.qaPrevH = w, h
	a.qaOpen = true
	a.qaMu.Unlock()

	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
	runtime.WindowSetAlwaysOnTop(a.ctx, true)
	runtime.WindowSetSize(a.ctx, 560, 460)
	runtime.WindowCenter(a.ctx)
	runtime.EventsEmit(a.ctx, "quick-access-open")
}

// CloseQuickAccess restores the main window after the popup closes.
func (a *App) CloseQuickAccess() {
	a.qaMu.Lock()
	open := a.qaOpen
	w, h := a.qaPrevW, a.qaPrevH
	a.qaOpen = false
	a.qaMu.Unlock()
	if a.ctx == nil {
		return
	}
	runtime.WindowSetAlwaysOnTop(a.ctx, false)
	if open && w > 0 && h > 0 {
		runtime.WindowSetSize(a.ctx, w, h)
		runtime.WindowCenter(a.ctx)
	}
	runtime.EventsEmit(a.ctx, "quick-access-close")
}

// currentVault returns the unlocked vault or nil (safe for goroutines).
func (a *App) currentVault() *Vault {
	a.vaultMu.RLock()
	defer a.vaultMu.RUnlock()
	if a.vault == nil || a.vault.vaultKey == nil {
		return nil
	}
	return a.vault
}

// validVaultFile prevents path traversal: only plain ".passapp" filenames.
func validVaultFile(file string) bool {
	return file != "" && filepath.Base(file) == file && strings.HasSuffix(file, ".passapp")
}

// VaultDir returns the directory that stores all vault files.
func (a *App) VaultDir() string {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(".", "ciphersync")
	}
	return filepath.Join(cfg, "LockSync")
}

// ListVaults returns the available vaults, most recently used first.
func (a *App) ListVaults() ([]VaultInfo, error) {
	return listVaultsIn(a.VaultDir())
}

// IsUnlocked reports whether a vault is currently unlocked.
func (a *App) IsUnlocked() bool {
	return a.currentVault() != nil
}

// CreateVault creates a new named vault with the given master password and unlocks it.
func (a *App) CreateVault(name, password, confirm string) error {
	if a.IsUnlocked() {
		return errors.New("a vault is already unlocked")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("give the vault a name")
	}
	if len(password) < 8 {
		return errors.New("master password must be at least 8 characters")
	}
	if password != confirm {
		return errors.New("passwords do not match")
	}
	if err := os.MkdirAll(a.VaultDir(), 0o700); err != nil {
		return err
	}
	base := slugify(name)
	file := base + ".passapp"
	path := filepath.Join(a.VaultDir(), file)
	for i := 2; fileExists(path); i++ {
		file = base + "-" + strconv.Itoa(i) + ".passapp"
		path = filepath.Join(a.VaultDir(), file)
	}
	v, err := createVault(path, password)
	if err != nil {
		return err
	}
	if err := v.setSetting("vault_name", name); err != nil {
		v.close()
		return err
	}
	a.vaultMu.Lock()
	a.vault = v
	a.vaultFile = file
	a.vaultName = name
	a.vaultMu.Unlock()
	_ = a.startLocalAPI()
	return nil
}

// OpenVault unlocks an existing vault file with the given master password.
func (a *App) OpenVault(file, password string) error {
	if a.IsUnlocked() {
		return errors.New("a vault is already unlocked")
	}
	if !validVaultFile(file) {
		return errors.New("invalid vault file")
	}
	if len(password) == 0 {
		return errors.New("enter your master password")
	}
	path := filepath.Join(a.VaultDir(), file)
	v, err := openVault(path, password)
	if err != nil {
		return err
	}
	a.vaultMu.Lock()
	a.vault = v
	a.vaultFile = file
	if name, err := readVaultName(path); err == nil && name != "" {
		a.vaultName = name
	} else {
		a.vaultName = strings.TrimSuffix(file, ".passapp")
	}
	a.vaultMu.Unlock()
	// daily automatic backup
	go func() {
		_, _ = a.autoBackup(v, file)
	}()
	// local API for the browser-extension host (only while unlocked)
	_ = a.startLocalAPI()
	// re-check the quick-access preference now that we can read settings
	a.qaMu.Lock()
	defer a.qaMu.Unlock()
	if val, err := v.getSetting("quick_access"); err == nil && val == "0" {
		a.disableQuickAccessHotkey()
	} else {
		_ = a.enableQuickAccessHotkey()
	}
	return nil
}

// GetCurrentVaultName returns the display name of the unlocked vault.
func (a *App) GetCurrentVaultName() string {
	a.vaultMu.RLock()
	defer a.vaultMu.RUnlock()
	if a.vaultName == "" && a.vaultFile != "" {
		return strings.TrimSuffix(a.vaultFile, ".passapp")
	}
	return a.vaultName
}

// DeleteVault removes a vault file from disk.
func (a *App) DeleteVault(file string) error {
	if a.IsUnlocked() {
		a.Lock()
	}
	if !validVaultFile(file) {
		return errors.New("invalid vault file")
	}
	path := filepath.Join(a.VaultDir(), file)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// DeleteAccount wipes every vault and all app data, resetting to first-run.
func (a *App) DeleteAccount() error {
	a.Lock()
	err := os.RemoveAll(a.VaultDir())
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ChangeMasterPassword rotates the master password of the unlocked vault.
func (a *App) ChangeMasterPassword(oldPassword, newPassword, confirm string) error {
	v := a.currentVault()
	if v == nil {
		return ErrVaultLocked
	}
	if len(newPassword) < 8 {
		return errors.New("master password must be at least 8 characters")
	}
	if newPassword != confirm {
		return errors.New("new passwords do not match")
	}
	return v.changeMasterPassword(oldPassword, newPassword)
}

// Lock locks the currently unlocked vault and wipes keys from memory.
func (a *App) Lock() {
	a.vaultMu.Lock()
	v := a.vault
	a.vault = nil
	a.vaultFile = ""
	a.vaultName = ""
	a.vaultMu.Unlock()
	if v != nil {
		v.close()
	}
	a.stopLocalAPI()
}

// GetItems returns all active (non-trashed) items of the unlocked vault.
func (a *App) GetItems() ([]Item, error) {
	v := a.currentVault()
	if v == nil {
		return nil, ErrVaultLocked
	}
	return v.list(), nil
}

// CreateItem adds a new item to the vault.
func (a *App) CreateItem(input Item) (Item, error) {
	v := a.currentVault()
	if v == nil {
		return Item{}, ErrVaultLocked
	}
	return v.create(input)
}

// UpdateItem edits an existing item in the vault.
func (a *App) UpdateItem(input Item) error {
	v := a.currentVault()
	if v == nil {
		return ErrVaultLocked
	}
	return v.update(input)
}

// DeleteItem moves an item to the trash (soft delete).
func (a *App) DeleteItem(id string) error {
	v := a.currentVault()
	if v == nil {
		return ErrVaultLocked
	}
	return v.trash(id)
}

// DeleteItems moves multiple items to the trash (soft delete).
func (a *App) DeleteItems(ids []string) error {
	v := a.currentVault()
	if v == nil {
		return ErrVaultLocked
	}
	return v.trashItems(ids)
}

// ---------- Trash ----------

// ListTrashed returns the items currently in the trash.
func (a *App) ListTrashed() ([]Item, error) {
	v := a.currentVault()
	if v == nil {
		return nil, ErrVaultLocked
	}
	return v.listTrashed(), nil
}

// RestoreTrashed brings a trashed item back to the active list.
func (a *App) RestoreTrashed(id string) error {
	v := a.currentVault()
	if v == nil {
		return ErrVaultLocked
	}
	return v.restoreTrashed(id)
}

// PurgeTrashed permanently removes the given items (and their versions/attachments).
func (a *App) PurgeTrashed(ids []string) error {
	v := a.currentVault()
	if v == nil {
		return ErrVaultLocked
	}
	return v.purgeItems(ids)
}

// SetTrashDays configures the trash retention in days.
func (a *App) SetTrashDays(days int) error {
	v := a.currentVault()
	if v == nil {
		return ErrVaultLocked
	}
	if days < 0 {
		return errors.New("invalid retention")
	}
	return v.setSetting("trash_days", strconv.Itoa(days))
}

// ---------- Batch operations ----------

// SetCategoryBatch sets the category of multiple items.
func (a *App) SetCategoryBatch(ids []string, category string) error {
	v := a.currentVault()
	if v == nil {
		return ErrVaultLocked
	}
	return v.setCategoryBatch(ids, category)
}

// AddTagBatch adds a tag to multiple items.
func (a *App) AddTagBatch(ids []string, tag string) error {
	v := a.currentVault()
	if v == nil {
		return ErrVaultLocked
	}
	return v.addTagBatch(ids, tag)
}

// SetFavoriteBatch marks multiple items as favorite or not.
func (a *App) SetFavoriteBatch(ids []string, favorite bool) error {
	v := a.currentVault()
	if v == nil {
		return ErrVaultLocked
	}
	return v.setFavoriteBatch(ids, favorite)
}

// ExportSelectedCSV returns selected items as CSV text.
func (a *App) ExportSelectedCSV(ids []string) (string, error) {
	v := a.currentVault()
	if v == nil {
		return "", ErrVaultLocked
	}
	return exportCSV(v.itemsByIDs(ids)), nil
}

// ExportSelectedJSON returns selected items as JSON text.
func (a *App) ExportSelectedJSON(ids []string) (string, error) {
	v := a.currentVault()
	if v == nil {
		return "", ErrVaultLocked
	}
	return exportJSON(v.itemsByIDs(ids))
}

// ---------- Attachments ----------

// AddAttachment stores an encrypted attachment (base64 data) on an item.
func (a *App) AddAttachment(itemID, name, dataB64 string) (Attachment, error) {
	v := a.currentVault()
	if v == nil {
		return Attachment{}, ErrVaultLocked
	}
	data, err := base64.StdEncoding.DecodeString(dataB64)
	if err != nil {
		return Attachment{}, errors.New("invalid file data")
	}
	return v.addAttachment(itemID, name, data)
}

// ListAttachments returns the metadata of an item's attachments.
func (a *App) ListAttachments(itemID string) ([]Attachment, error) {
	v := a.currentVault()
	if v == nil {
		return nil, ErrVaultLocked
	}
	return v.listAttachments(itemID)
}

// GetAttachment returns an attachment's name and base64 data.
func (a *App) GetAttachment(id string) (AttachmentPayload, error) {
	v := a.currentVault()
	if v == nil {
		return AttachmentPayload{}, ErrVaultLocked
	}
	_, name, data, err := v.getAttachment(id)
	if err != nil {
		return AttachmentPayload{}, err
	}
	return AttachmentPayload{Name: name, Data: base64.StdEncoding.EncodeToString(data)}, nil
}

// DeleteAttachment removes an attachment permanently.
func (a *App) DeleteAttachment(id string) error {
	v := a.currentVault()
	if v == nil {
		return ErrVaultLocked
	}
	return v.deleteAttachment(id)
}

// ---------- Backups ----------

// BackupNow creates a consistent snapshot of the vault and returns its path.
func (a *App) BackupNow() (string, error) {
	a.vaultMu.RLock()
	v := a.vault
	file := a.vaultFile
	a.vaultMu.RUnlock()
	if v == nil || v.vaultKey == nil || !validVaultFile(file) {
		return "", ErrVaultLocked
	}
	return a.backupVaultFile(v, file)
}

// autoBackup backs up only if no backup exists for today.
func (a *App) autoBackup(v *Vault, file string) (string, error) {
	backups := filepath.Join(a.VaultDir(), "backups")
	base := strings.TrimSuffix(file, ".passapp")
	today := time.Now().Format("20060102")
	entries, err := os.ReadDir(backups)
	if err == nil {
		for _, e := range entries {
			name := e.Name()
			if strings.HasPrefix(name, base+"-"+today) && strings.HasSuffix(name, ".passapp") {
				return "", nil // already backed up today
			}
		}
	}
	return a.backupVaultFile(v, file)
}

func (a *App) backupVaultFile(v *Vault, file string) (string, error) {
	backups := filepath.Join(a.VaultDir(), "backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		return "", err
	}
	base := strings.TrimSuffix(file, ".passapp")
	dest := filepath.Join(backups, fmt.Sprintf("%s-%s.passapp", base, time.Now().Format("20060102-150405")))
	if err := v.backupTo(dest); err != nil {
		return "", err
	}
	a.pruneBackups(backups, base)
	return dest, nil
}

func (a *App) pruneBackups(dir, base string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var files []string
	prefix := base + "-"
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() && strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".passapp") {
			files = append(files, name)
		}
	}
	sort.Strings(files)
	for len(files) > maxBackupsPerVault {
		_ = os.Remove(filepath.Join(dir, files[0]))
		files = files[1:]
	}
}

// ---------- Generator ----------

// GeneratePassword creates a random password according to the given options.
func (a *App) GeneratePassword(opts PasswordOptions) (string, error) {
	return generatePassword(opts)
}

// GeneratePassphrase creates a word-based passphrase.
func (a *App) GeneratePassphrase(words int) (string, error) {
	return generatePassphrase(words)
}

// CopyToClipboard writes text to the system clipboard and schedules an auto-clear.
func (a *App) CopyToClipboard(text string) error {
	if err := clipboard.WriteAll(text); err != nil {
		return err
	}
	a.scheduleClipboardClear(text)
	return nil
}

func (a *App) scheduleClipboardClear(text string) {
	a.clipMu.Lock()
	defer a.clipMu.Unlock()
	if a.clipStop != nil {
		close(a.clipStop)
	}
	a.clipStop = make(chan struct{})
	stop := a.clipStop
	go func() {
		t := time.NewTimer(clipboardClearDelay)
		defer t.Stop()
		select {
		case <-t.C:
			// only clear if the user hasn't copied something else meanwhile
			if cur, err := clipboard.ReadAll(); err == nil && cur == text {
				_ = clipboard.WriteAll("")
			}
		case <-stop:
		}
	}()
}

// ---------- Version history ----------

// GetItemVersions returns the change history of an item, newest first.
func (a *App) GetItemVersions(itemID string) ([]VersionEntry, error) {
	v := a.currentVault()
	if v == nil {
		return nil, ErrVaultLocked
	}
	return v.getVersions(itemID)
}

// RestoreVersion restores an item to a previous version and returns it.
func (a *App) RestoreVersion(versionID string) (Item, error) {
	v := a.currentVault()
	if v == nil {
		return Item{}, ErrVaultLocked
	}
	return v.restoreVersion(versionID)
}

// ---------- Settings ----------

// GetSettings returns vault settings relevant to the UI.
func (a *App) GetSettings() (map[string]string, error) {
	v := a.currentVault()
	if v == nil {
		return nil, ErrVaultLocked
	}
	out := map[string]string{}
	for _, key := range []string{"autolock_minutes", "default_type", "trash_days", "close_to_tray", "quick_access"} {
		val, err := v.getSetting(key)
		if err != nil {
			return nil, err
		}
		out[key] = val
	}
	if out["autolock_minutes"] == "" {
		out["autolock_minutes"] = "5"
	}
	if out["default_type"] == "" {
		out["default_type"] = TypeLogin
	}
	if out["trash_days"] == "" {
		out["trash_days"] = strconv.Itoa(defaultTrashDays)
	}
	if out["close_to_tray"] == "" {
		out["close_to_tray"] = "1"
	}
	if out["quick_access"] == "" {
		out["quick_access"] = "1"
	}
	return out, nil
}

// SetSetting persists a vault setting.
func (a *App) SetSetting(key, value string) error {
	v := a.currentVault()
	if v == nil {
		return ErrVaultLocked
	}
	return v.setSetting(key, value)
}

// closeToTray reports whether closing the window should hide to tray.
// Defaults to true when the vault is locked or the setting is absent.
func (a *App) closeToTray() bool {
	v := a.currentVault()
	if v == nil {
		return true
	}
	val, err := v.getSetting("close_to_tray")
	if err != nil || val == "" {
		return true
	}
	return val != "0"
}

// SetCloseToTray configures whether X hides to tray (true) or quits (false).
func (a *App) SetCloseToTray(enabled bool) error {
	v := a.currentVault()
	if v == nil {
		return ErrVaultLocked
	}
	val := "1"
	if !enabled {
		val = "0"
	}
	return v.setSetting("close_to_tray", val)
}

// SetAutolockMinutes configures the auto-lock timeout (0 = never).
func (a *App) SetAutolockMinutes(minutes int) error {
	v := a.currentVault()
	if v == nil {
		return ErrVaultLocked
	}
	return v.setSetting("autolock_minutes", strconv.Itoa(minutes))
}

// ---------- Favicons ----------

// PrefetchFavicons asynchronously fetches favicons for all login items
// and emits a "favicon" event with {domain: dataURI} for each result.
func (a *App) PrefetchFavicons() {
	go a.prefetchFavicons()
}

func (a *App) prefetchFavicons() {
	v := a.currentVault()
	if v == nil {
		return
	}
	for _, it := range v.list() {
		if it.Type != TypeLogin || it.URL == "" {
			continue
		}
		domain := extractDomain(it.URL)
		if domain == "" {
			continue
		}
		if data, ok := v.getFaviconCached(domain); ok {
			runtime.EventsEmit(a.ctx, "favicon", map[string]string{domain: data})
			continue
		}
		faviconPool.mu.Lock()
		if faviconPool.inFly[domain] {
			faviconPool.mu.Unlock()
			continue
		}
		faviconPool.inFly[domain] = true
		faviconPool.mu.Unlock()

		data, err := fetchFavicon(domain)
		faviconPool.mu.Lock()
		delete(faviconPool.inFly, domain)
		faviconPool.mu.Unlock()
		if err != nil || data == "" {
			continue
		}
		v.setFaviconCache(domain, data)
		runtime.EventsEmit(a.ctx, "favicon", map[string]string{domain: data})
	}
}

// ---------- Import ----------

// ImportCSV imports a generic CSV using a column-to-field mapping.
func (a *App) ImportCSV(data string, mapping []FieldMapping) (ImportResult, error) {
	if !a.IsUnlocked() {
		return ImportResult{}, ErrVaultLocked
	}
	items, err := parseCSV(data, mapping)
	if err != nil {
		return ImportResult{}, err
	}
	return ImportResult{Preview: items}, nil
}

// ImportAutoCSV imports a CSV whose headers are auto-detected
// (Chrome, Firefox, LastPass, 1Password and Bitwarden CSV exports).
func (a *App) ImportAutoCSV(data string) (ImportResult, error) {
	if !a.IsUnlocked() {
		return ImportResult{}, ErrVaultLocked
	}
	items, err := parseAutoCSV(data)
	if err != nil {
		return ImportResult{}, err
	}
	return ImportResult{Preview: items}, nil
}

// ImportBitwardenJSON imports an unencrypted Bitwarden JSON export.
func (a *App) ImportBitwardenJSON(data string) (ImportResult, error) {
	if !a.IsUnlocked() {
		return ImportResult{}, ErrVaultLocked
	}
	items, err := parseBitwardenJSON(data)
	if err != nil {
		return ImportResult{}, err
	}
	return ImportResult{Preview: items}, nil
}

// ImportKeePassDB imports a KeePass .kdbx database (v3/v4) given its password.
func (a *App) ImportKeePassDB(dataB64, password string) (ImportResult, error) {
	if !a.IsUnlocked() {
		return ImportResult{}, ErrVaultLocked
	}
	raw, err := base64.StdEncoding.DecodeString(dataB64)
	if err != nil {
		return ImportResult{}, errors.New("invalid file data")
	}
	items, err := parseKeePassDB(raw, password)
	if err != nil {
		return ImportResult{}, err
	}
	return ImportResult{Preview: items}, nil
}

// ImportEncryptedTransfer imports items from a CipherSync transfer file.
func (a *App) ImportEncryptedTransfer(data, password string) (ImportResult, error) {
	if !a.IsUnlocked() {
		return ImportResult{}, ErrVaultLocked
	}
	items, err := openTransfer(data, password)
	if err != nil {
		return ImportResult{}, err
	}
	return ImportResult{Preview: items}, nil
}

// ImportCommit persists a set of items from an import preview.
func (a *App) ImportCommit(items []Item) (ImportResult, error) {
	v := a.currentVault()
	if v == nil {
		return ImportResult{}, ErrVaultLocked
	}
	return v.importItems(items), nil
}

// ---------- Export ----------

// ExportCSV returns all items as CSV text.
func (a *App) ExportCSV() (string, error) {
	v := a.currentVault()
	if v == nil {
		return "", ErrVaultLocked
	}
	return exportCSV(v.list()), nil
}

// ExportJSON returns all items as JSON text.
func (a *App) ExportJSON() (string, error) {
	v := a.currentVault()
	if v == nil {
		return "", ErrVaultLocked
	}
	return exportJSON(v.list())
}

// ExportEncryptedJSON seals all items into a CipherSync transfer string.
func (a *App) ExportEncryptedJSON(password string) (string, error) {
	v := a.currentVault()
	if v == nil {
		return "", ErrVaultLocked
	}
	return sealTransfer(v.list(), password)
}

// ---------- TOTP / 2FA ----------

// GenerateTOTPSetup creates a fresh TOTP secret + QR code for an item.
func (a *App) GenerateTOTPSetup(itemID string) (TOTPSetupInfo, error) {
	v := a.currentVault()
	if v == nil {
		return TOTPSetupInfo{}, ErrVaultLocked
	}
	item, err := v.getItem(itemID)
	if err != nil {
		return TOTPSetupInfo{}, err
	}
	issuer, account := otpIssuerForItem(item.Title, item.Username)
	secret, otpauthURL, err := generateTOTPKey(issuer, account)
	if err != nil {
		return TOTPSetupInfo{}, err
	}
	qr, err := generateTOTPQR(otpauthURL)
	if err != nil {
		return TOTPSetupInfo{}, err
	}
	return TOTPSetupInfo{Secret: secret, QR: qr, OtpauthURL: otpauthURL}, nil
}

// GetTOTPCode returns the current code for an item's stored secret.
func (a *App) GetTOTPCode(itemID string) (TOTPCode, error) {
	v := a.currentVault()
	if v == nil {
		return TOTPCode{}, ErrVaultLocked
	}
	item, err := v.getItem(itemID)
	if err != nil {
		return TOTPCode{}, err
	}
	if item.TotpSecret == "" {
		return TOTPCode{}, errors.New("item sem 2FA configurado")
	}
	code, seconds, err := totpCode(item.TotpSecret)
	if err != nil {
		return TOTPCode{}, err
	}
	return TOTPCode{Code: code, SecondsRemaining: seconds}, nil
}

// GetTOTPCodeForSecret verifies a secret during setup (without saving).
func (a *App) GetTOTPCodeForSecret(secret string) (TOTPCode, error) {
	if err := validateTOTPSecret(secret); err != nil {
		return TOTPCode{}, err
	}
	code, seconds, err := totpCode(secret)
	if err != nil {
		return TOTPCode{}, err
	}
	return TOTPCode{Code: code, SecondsRemaining: seconds}, nil
}

// ValidateTOTPSecret checks a manually-entered TOTP secret.
func (a *App) ValidateTOTPSecret(secret string) error {
	return validateTOTPSecret(secret)
}

// IngestTOTPURI extracts and validates the secret from a scanned otpauth URI.
func (a *App) IngestTOTPURI(uri string) (string, error) {
	return parseTOTPSecretFromURI(uri)
}

// ---------- Watchtower ----------

// AnalyzeVault returns the password health report for the current vault.
func (a *App) AnalyzeVault() (HealthReport, error) {
	v := a.currentVault()
	if v == nil {
		return HealthReport{}, ErrVaultLocked
	}
	return analyzeVault(v.list()), nil
}
