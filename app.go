package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/atotto/clipboard"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const clipboardClearDelay = 60 * time.Second

// App is the root application object exposed to the frontend.
type App struct {
	ctx      context.Context
	vault    *Vault
	clipMu   sync.Mutex
	clipStop chan struct{}
}

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// VaultPath returns the default vault file path.
func (a *App) VaultPath() string {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "vault.passapp"
	}
	return filepath.Join(cfg, "LockSync", "vault.passapp")
}

// VaultExists reports whether a vault already exists at the default path.
func (a *App) VaultExists() bool {
	_, err := os.Stat(a.VaultPath())
	return err == nil
}

// IsUnlocked reports whether a vault is currently unlocked.
func (a *App) IsUnlocked() bool {
	return a.vault != nil && a.vault.vaultKey != nil
}

// CreateVault creates a new vault with the given master password and unlocks it.
func (a *App) CreateVault(password, confirm string) error {
	if a.IsUnlocked() {
		return errors.New("a vault is already unlocked")
	}
	if len(password) < 8 {
		return errors.New("master password must be at least 8 characters")
	}
	if password != confirm {
		return errors.New("passwords do not match")
	}
	v, err := createVault(a.VaultPath(), password)
	if err != nil {
		return err
	}
	a.vault = v
	return nil
}

// OpenVault unlocks the existing vault with the given master password.
func (a *App) OpenVault(password string) error {
	if a.IsUnlocked() {
		return errors.New("a vault is already unlocked")
	}
	if len(password) == 0 {
		return errors.New("enter your master password")
	}
	v, err := openVault(a.VaultPath(), password)
	if err != nil {
		return err
	}
	a.vault = v
	return nil
}

// ChangeMasterPassword rotates the master password of the unlocked vault.
func (a *App) ChangeMasterPassword(oldPassword, newPassword, confirm string) error {
	if !a.IsUnlocked() {
		return ErrVaultLocked
	}
	if len(newPassword) < 8 {
		return errors.New("master password must be at least 8 characters")
	}
	if newPassword != confirm {
		return errors.New("new passwords do not match")
	}
	return a.vault.changeMasterPassword(oldPassword, newPassword)
}

// Lock locks the currently unlocked vault and wipes keys from memory.
func (a *App) Lock() {
	if a.vault != nil {
		a.vault.close()
	}
	a.vault = nil
}

// GetItems returns all items of the unlocked vault.
func (a *App) GetItems() ([]Item, error) {
	if !a.IsUnlocked() {
		return nil, ErrVaultLocked
	}
	return a.vault.list(), nil
}

// CreateItem adds a new item to the vault.
func (a *App) CreateItem(input Item) (Item, error) {
	if !a.IsUnlocked() {
		return Item{}, ErrVaultLocked
	}
	return a.vault.create(input)
}

// UpdateItem edits an existing item in the vault.
func (a *App) UpdateItem(input Item) error {
	if !a.IsUnlocked() {
		return ErrVaultLocked
	}
	return a.vault.update(input)
}

// DeleteItem removes an item from the vault.
func (a *App) DeleteItem(id string) error {
	if !a.IsUnlocked() {
		return ErrVaultLocked
	}
	return a.vault.delete(id)
}

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
	a.scheduleClipboardClear()
	return nil
}

func (a *App) scheduleClipboardClear() {
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
			_ = clipboard.WriteAll("")
		case <-stop:
		}
	}()
}

// ---------- Version history ----------

// GetItemVersions returns the change history of an item, newest first.
func (a *App) GetItemVersions(itemID string) ([]VersionEntry, error) {
	if !a.IsUnlocked() {
		return nil, ErrVaultLocked
	}
	return a.vault.getVersions(itemID)
}

// RestoreVersion restores an item to a previous version and returns it.
func (a *App) RestoreVersion(versionID string) (Item, error) {
	if !a.IsUnlocked() {
		return Item{}, ErrVaultLocked
	}
	return a.vault.restoreVersion(versionID)
}

// ---------- Settings ----------

// GetSettings returns vault settings relevant to the UI.
func (a *App) GetSettings() (map[string]string, error) {
	if !a.IsUnlocked() {
		return nil, ErrVaultLocked
	}
	out := map[string]string{}
	for _, key := range []string{"autolock_minutes", "default_type"} {
		v, err := a.vault.getSetting(key)
		if err != nil {
			return nil, err
		}
		out[key] = v
	}
	if out["autolock_minutes"] == "" {
		out["autolock_minutes"] = "5"
	}
	if out["default_type"] == "" {
		out["default_type"] = TypeLogin
	}
	return out, nil
}

// SetSetting persists a vault setting.
func (a *App) SetSetting(key, value string) error {
	if !a.IsUnlocked() {
		return ErrVaultLocked
	}
	return a.vault.setSetting(key, value)
}

// SetAutolockMinutes configures the auto-lock timeout (0 = never).
func (a *App) SetAutolockMinutes(minutes int) error {
	if !a.IsUnlocked() {
		return ErrVaultLocked
	}
	return a.vault.setSetting("autolock_minutes", strconv.Itoa(minutes))
}

// ---------- Favicons ----------

// PrefetchFavicons asynchronously fetches favicons for all login items
// and emits a "favicon" event with {domain: dataURI} for each result.
func (a *App) PrefetchFavicons() {
	go a.prefetchFavicons()
}

func (a *App) prefetchFavicons() {
	if !a.IsUnlocked() {
		return
	}
	for _, it := range a.vault.list() {
		if it.Type != TypeLogin || it.URL == "" {
			continue
		}
		domain := extractDomain(it.URL)
		if domain == "" {
			continue
		}
		if data, ok := a.vault.getFaviconCached(domain); ok {
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
		a.vault.setFaviconCache(domain, data)
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
// (LastPass, 1Password and Bitwarden CSV exports).
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

// ImportEncryptedTransfer imports items from a LockSync transfer file.
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
	if !a.IsUnlocked() {
		return ImportResult{}, ErrVaultLocked
	}
	return a.vault.importItems(items), nil
}

// ---------- Export ----------

// ExportCSV returns all items as CSV text.
func (a *App) ExportCSV() (string, error) {
	if !a.IsUnlocked() {
		return "", ErrVaultLocked
	}
	return exportCSV(a.vault.list()), nil
}

// ExportJSON returns all items as JSON text.
func (a *App) ExportJSON() (string, error) {
	if !a.IsUnlocked() {
		return "", ErrVaultLocked
	}
	return exportJSON(a.vault.list())
}

// ExportEncryptedJSON seals all items into a LockSync transfer string.
func (a *App) ExportEncryptedJSON(password string) (string, error) {
	if !a.IsUnlocked() {
		return "", ErrVaultLocked
	}
	return sealTransfer(a.vault.list(), password)
}
