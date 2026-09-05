package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

var (
	ErrWrongPassword = errors.New("wrong master password")
	ErrVaultLocked   = errors.New("vault is locked")
	ErrItemNotFound  = errors.New("item not found")
	ErrNotAVault     = errors.New("not a valid CipherSync vault")
	ErrTooLarge      = errors.New("file too large")
)

const metaSaltKey = "kdf_salt"
const metaParamsKey = "kdf_params"
const metaVaultKey = "encrypted_vault_key"

const defaultTrashDays = 30
const maxAttachmentBytes = 10 * 1024 * 1024

func ensureTables(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS items (
			id TEXT PRIMARY KEY,
			encrypted BLOB NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS meta (
			key TEXT PRIMARY KEY,
			value BLOB NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS item_versions (
			id TEXT PRIMARY KEY,
			item_id TEXT NOT NULL,
			encrypted BLOB NOT NULL,
			ts INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_item_versions_item ON item_versions(item_id)`,
		`CREATE TABLE IF NOT EXISTS favicons (
			domain TEXT PRIMARY KEY,
			data TEXT NOT NULL,
			fetched_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS attachments (
			id TEXT PRIMARY KEY,
			item_id TEXT NOT NULL,
			name TEXT NOT NULL,
			encrypted BLOB NOT NULL,
			size INTEGER NOT NULL,
			added_at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_attachments_item ON attachments(item_id)`,
	}
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

type Vault struct {
	mu       sync.RWMutex
	path     string
	db       *sql.DB
	vaultKey []byte
	items    []Item
	itemsBy  map[string]*Item
}

func createVault(path, password string) (*Vault, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := ensureTables(db); err != nil {
		db.Close()
		return nil, err
	}

	salt, err := randomBytes(saltLen)
	if err != nil {
		db.Close()
		return nil, err
	}
	vaultKey, err := randomBytes(keyLen)
	if err != nil {
		wipe(salt)
		db.Close()
		return nil, err
	}
	params := newKDFParams()
	masterKey := deriveKey(password, salt, params)
	encVaultKey, err := encrypt(masterKey, vaultKey)
	wipe(masterKey)
	if err != nil {
		wipe(vaultKey)
		wipe(salt)
		db.Close()
		return nil, err
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		wipe(vaultKey)
		wipe(salt)
		db.Close()
		return nil, err
	}

	tx, err := db.Begin()
	if err != nil {
		wipe(vaultKey)
		wipe(salt)
		db.Close()
		return nil, err
	}
	for k, v := range map[string][]byte{
		metaSaltKey:   salt,
		metaParamsKey: paramsJSON,
		metaVaultKey:  encVaultKey,
	} {
		if _, err := tx.Exec(`INSERT INTO meta (key, value) VALUES (?, ?)`, k, v); err != nil {
			tx.Rollback()
			wipe(vaultKey)
			wipe(salt)
			db.Close()
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		wipe(vaultKey)
		wipe(salt)
		db.Close()
		return nil, err
	}

	return &Vault{
		path:     path,
		db:       db,
		vaultKey: vaultKey,
		items:    []Item{},
		itemsBy:  map[string]*Item{},
	}, nil
}

func openVault(path, password string) (*Vault, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, ErrNotAVault
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := ensureTables(db); err != nil {
		db.Close()
		return nil, ErrNotAVault
	}

	var salt, encVaultKey, paramsJSON []byte
	rows, err := db.Query(`SELECT key, value FROM meta`)
	if err != nil {
		db.Close()
		return nil, ErrNotAVault
	}
	for rows.Next() {
		var k string
		var v []byte
		if err := rows.Scan(&k, &v); err != nil {
			rows.Close()
			db.Close()
			return nil, err
		}
		switch k {
		case metaSaltKey:
			salt = v
		case metaParamsKey:
			paramsJSON = v
		case metaVaultKey:
			encVaultKey = v
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		db.Close()
		return nil, err
	}
	rows.Close()
	if salt == nil || encVaultKey == nil || paramsJSON == nil {
		db.Close()
		return nil, ErrNotAVault
	}

	var params kdfParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		wipe(salt)
		wipe(encVaultKey)
		db.Close()
		return nil, ErrNotAVault
	}

	masterKey := deriveKey(password, salt, params)
	wipe(salt)
	vaultKey, err := decrypt(masterKey, encVaultKey)
	wipe(masterKey)
	wipe(encVaultKey)
	if err != nil {
		db.Close()
		return nil, ErrWrongPassword
	}

	v := &Vault{
		path:     path,
		db:       db,
		vaultKey: vaultKey,
		items:    []Item{},
		itemsBy:  map[string]*Item{},
	}
	if err := v.loadItems(); err != nil {
		v.close()
		return nil, err
	}
	// purge expired trash based on the vault's retention setting
	trashDays := defaultTrashDays
	if raw, err := v.getSetting("trash_days"); err == nil && raw != "" {
		if d, err := parseIntDefault(raw, defaultTrashDays); err == nil {
			trashDays = d
		}
	}
	_ = v.purgeExpiredTrash(trashDays)
	return v, nil
}

func (v *Vault) loadItems() error {
	rows, err := v.db.Query(`SELECT encrypted FROM items`)
	if err != nil {
		return err
	}
	defer rows.Close()
	v.items = []Item{}
	v.itemsBy = map[string]*Item{}
	for rows.Next() {
		var blob []byte
		if err := rows.Scan(&blob); err != nil {
			return err
		}
		plain, err := decrypt(v.vaultKey, blob)
		if err != nil {
			return err
		}
		var item Item
		if err := json.Unmarshal(plain, &item); err != nil {
			return err
		}
		normalizeItem(&item)
		v.items = append(v.items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	v.sortItems()
	return nil
}

func normalizeItem(item *Item) {
	if item.Type == "" {
		item.Type = TypeLogin
	}
	if item.Tags == nil {
		item.Tags = []string{}
	}
	if item.Fields == nil {
		item.Fields = map[string]string{}
	}
}

// openVaultWithKey opens a vault using a previously-obtained vault key
// (e.g. held in memory across a sync download-swap) instead of the
// master password. The key is verified by decrypting the stored items.
// The key is copied internally; callers keep ownership of their slice.
func openVaultWithKey(path string, vaultKey []byte) (*Vault, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, ErrNotAVault
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := ensureTables(db); err != nil {
		db.Close()
		return nil, ErrNotAVault
	}
	keyCopy := append([]byte{}, vaultKey...)
	v := &Vault{
		path:     path,
		db:       db,
		vaultKey: keyCopy,
		items:    []Item{},
		itemsBy:  map[string]*Item{},
	}
	if err := v.loadItems(); err != nil {
		v.close()
		return nil, ErrWrongPassword
	}
	return v, nil
}

// sortItems sorts the in-memory items and rebuilds the index map, keeping
// itemsBy pointers consistent with the sorted slice.
func (v *Vault) sortItems() {
	sort.Slice(v.items, func(i, j int) bool {
		return v.items[i].Title < v.items[j].Title
	})
	v.rebuildIndex()
}

func (v *Vault) rebuildIndex() {
	v.itemsBy = make(map[string]*Item, len(v.items))
	for i := range v.items {
		v.itemsBy[v.items[i].ID] = &v.items[i]
	}
}

func (v *Vault) close() {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.vaultKey != nil {
		wipe(v.vaultKey)
		v.vaultKey = nil
	}
	v.items = nil
	v.itemsBy = nil
	if v.db != nil {
		v.db.Close()
		v.db = nil
	}
}

// list returns the active (non-trashed) items.
func (v *Vault) list() []Item {
	v.mu.RLock()
	defer v.mu.RUnlock()
	out := []Item{}
	for _, it := range v.items {
		if !it.Deleted {
			out = append(out, it)
		}
	}
	return out
}

// listTrashed returns the soft-deleted items, oldest first.
func (v *Vault) listTrashed() []Item {
	v.mu.RLock()
	defer v.mu.RUnlock()
	out := []Item{}
	for _, it := range v.items {
		if it.Deleted {
			out = append(out, it)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].DeletedAt < out[j].DeletedAt
	})
	return out
}

func (v *Vault) getItem(id string) (Item, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	it, ok := v.itemsBy[id]
	if !ok {
		return Item{}, ErrItemNotFound
	}
	return *it, nil
}

func (v *Vault) create(input Item) (Item, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	normalizeItem(&input)
	if input.Type == "" {
		input.Type = TypeLogin
	}
	if input.Type == TypePasskey || input.Passkey != nil {
		if err := validatePasskey("", input.Passkey, v.items); err != nil {
			return Item{}, err
		}
		if input.Passkey != nil {
			input.Passkey.RpID = strings.ToLower(strings.TrimSpace(input.Passkey.RpID))
			input.Passkey.CredentialID = strings.TrimSpace(input.Passkey.CredentialID)
			input.Passkey.UserHandle = strings.TrimSpace(input.Passkey.UserHandle)
		}
	}
	now := time.Now().UnixMilli()
	input.ID = uuid.NewString()
	input.CreatedAt = now
	input.UpdatedAt = now
	if err := v.persistItem(input); err != nil {
		return Item{}, err
	}
	v.items = append(v.items, input)
	v.sortItems()
	return input, nil
}

func (v *Vault) update(input Item) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	existing, ok := v.itemsBy[input.ID]
	if !ok {
		return ErrItemNotFound
	}
	normalizeItem(&input)
	if input.Type == TypePasskey || input.Passkey != nil {
		if err := validatePasskey(input.ID, input.Passkey, v.items); err != nil {
			return err
		}
		if input.Passkey != nil {
			input.Passkey.RpID = strings.ToLower(strings.TrimSpace(input.Passkey.RpID))
			input.Passkey.CredentialID = strings.TrimSpace(input.Passkey.CredentialID)
			input.Passkey.UserHandle = strings.TrimSpace(input.Passkey.UserHandle)
		}
	}
	// skip version snapshot when only the favorite flag changed
	onlyFavorite := existing.Favorite != input.Favorite &&
		existing.Title == input.Title &&
		existing.Username == input.Username &&
		existing.Password == input.Password &&
		existing.URL == input.URL &&
		existing.Notes == input.Notes &&
		existing.Category == input.Category &&
		existing.TotpSecret == input.TotpSecret
	if !onlyFavorite {
		if err := v.addVersion(*existing); err != nil {
			return err
		}
	}
	input.UpdatedAt = time.Now().UnixMilli()
	if err := v.persistItem(input); err != nil {
		return err
	}
	idx := 0
	for i, it := range v.items {
		if it.ID == input.ID {
			idx = i
			break
		}
	}
	v.items[idx] = input
	v.sortItems()
	return nil
}

// trash soft-deletes an item (recoverable until purged).
func (v *Vault) trash(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	it, ok := v.itemsBy[id]
	if !ok {
		return ErrItemNotFound
	}
	it.Deleted = true
	it.DeletedAt = time.Now().UnixMilli()
	if err := v.persistItem(*it); err != nil {
		return err
	}
	return nil
}

// trashItems soft-deletes multiple items in a single transaction.
func (v *Vault) trashItems(ids []string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	type change struct {
		idx  int
		item Item
	}
	var changes []change
	now := time.Now().UnixMilli()
	for i := range v.items {
		if !contains(ids, v.items[i].ID) {
			continue
		}
		ni := v.items[i]
		ni.Deleted = true
		ni.DeletedAt = now
		changes = append(changes, change{i, ni})
	}
	tx, err := v.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, c := range changes {
		if err := v.persistItemTx(tx, c.item); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	for _, c := range changes {
		v.items[c.idx] = c.item
	}
	return nil
}

// restoreTrashed brings a soft-deleted item back.
func (v *Vault) restoreTrashed(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	it, ok := v.itemsBy[id]
	if !ok || !it.Deleted {
		return ErrItemNotFound
	}
	it.Deleted = false
	it.DeletedAt = 0
	return v.persistItem(*it)
}

// restoreTrashedItems restores multiple soft-deleted items in one transaction.
func (v *Vault) restoreTrashedItems(ids []string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	type change struct {
		idx  int
		item Item
	}
	var changes []change
	for i := range v.items {
		if v.items[i].Deleted && contains(ids, v.items[i].ID) {
			ni := v.items[i]
			ni.Deleted = false
			ni.DeletedAt = 0
			changes = append(changes, change{i, ni})
		}
	}
	if len(changes) == 0 {
		return ErrItemNotFound
	}
	tx, err := v.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, c := range changes {
		if err := v.persistItemTx(tx, c.item); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	for _, c := range changes {
		v.items[c.idx] = c.item
	}
	return nil
}

// delete permanently removes an item, its versions and attachments.
func (v *Vault) delete(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if _, ok := v.itemsBy[id]; !ok {
		return ErrItemNotFound
	}
	tx, err := v.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM items WHERE id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM item_versions WHERE item_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM attachments WHERE item_id = ?`, id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	for i, it := range v.items {
		if it.ID == id {
			v.items = append(v.items[:i], v.items[i+1:]...)
			break
		}
	}
	v.rebuildIndex()
	return nil
}

// purgeItems permanently removes multiple items (used by the trash view).
func (v *Vault) purgeItems(ids []string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	tx, err := v.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		if _, err := tx.Exec(`DELETE FROM items WHERE id = ?`, id); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM item_versions WHERE item_id = ?`, id); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM attachments WHERE item_id = ?`, id); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	keep := v.items[:0]
	for _, it := range v.items {
		if !contains(ids, it.ID) {
			keep = append(keep, it)
		}
	}
	v.items = keep
	v.rebuildIndex()
	return nil
}

// purgeExpiredTrash removes trashed items older than the given retention.
func (v *Vault) purgeExpiredTrash(days int) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if days <= 0 {
		return nil
	}
	cutoff := time.Now().AddDate(0, 0, -days).UnixMilli()
	var expired []string
	for _, it := range v.items {
		if it.Deleted && it.DeletedAt > 0 && it.DeletedAt < cutoff {
			expired = append(expired, it.ID)
		}
	}
	if len(expired) == 0 {
		return nil
	}
	tx, err := v.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range expired {
		if _, err := tx.Exec(`DELETE FROM items WHERE id = ?`, id); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM item_versions WHERE item_id = ?`, id); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM attachments WHERE item_id = ?`, id); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	keep := v.items[:0]
	for _, it := range v.items {
		if !contains(expired, it.ID) {
			keep = append(keep, it)
		}
	}
	v.items = keep
	v.rebuildIndex()
	return nil
}

func (v *Vault) persistItem(item Item) error {
	plain, err := json.Marshal(item)
	if err != nil {
		return err
	}
	blob, err := encrypt(v.vaultKey, plain)
	if err != nil {
		return err
	}
	_, err = v.db.Exec(`INSERT INTO items (id, encrypted, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET encrypted = excluded.encrypted, updated_at = excluded.updated_at`,
		item.ID, blob, item.CreatedAt, item.UpdatedAt)
	return err
}

func (v *Vault) persistItemTx(tx *sql.Tx, item Item) error {
	plain, err := json.Marshal(item)
	if err != nil {
		return err
	}
	blob, err := encrypt(v.vaultKey, plain)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO items (id, encrypted, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET encrypted = excluded.encrypted, updated_at = excluded.updated_at`,
		item.ID, blob, item.CreatedAt, item.UpdatedAt)
	return err
}

func (v *Vault) changeMasterPassword(oldPassword, newPassword string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if oldPassword == newPassword {
		return errors.New("new password must differ")
	}
	if v.vaultKey == nil {
		return ErrVaultLocked
	}

	var salt, encVaultKey, paramsJSON []byte
	rows, err := v.db.Query(`SELECT key, value FROM meta WHERE key IN (?, ?, ?)`, metaSaltKey, metaParamsKey, metaVaultKey)
	if err != nil {
		return err
	}
	for rows.Next() {
		var k string
		var b []byte
		if err := rows.Scan(&k, &b); err != nil {
			rows.Close()
			return err
		}
		switch k {
		case metaSaltKey:
			salt = b
		case metaParamsKey:
			paramsJSON = b
		case metaVaultKey:
			encVaultKey = b
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if salt == nil || encVaultKey == nil || paramsJSON == nil {
		return ErrNotAVault
	}

	var params kdfParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return ErrNotAVault
	}
	masterKey := deriveKey(oldPassword, salt, params)
	derivedVaultKey, err := decrypt(masterKey, encVaultKey)
	wipe(masterKey)
	wipe(encVaultKey)
	if err != nil {
		wipe(salt)
		return ErrWrongPassword
	}
	wipe(derivedVaultKey)

	newSalt, err := randomBytes(saltLen)
	if err != nil {
		wipe(salt)
		return err
	}
	newMasterKey := deriveKey(newPassword, newSalt, params)
	newEncVaultKey, err := encrypt(newMasterKey, v.vaultKey)
	wipe(newMasterKey)
	if err != nil {
		wipe(salt)
		wipe(newSalt)
		return err
	}

	// atomic: both meta rows must change together or not at all
	tx, err := v.db.Begin()
	if err != nil {
		wipe(salt)
		wipe(newSalt)
		wipe(newEncVaultKey)
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE meta SET value = ? WHERE key = ?`, newSalt, metaSaltKey); err != nil {
		wipe(salt)
		wipe(newSalt)
		wipe(newEncVaultKey)
		return err
	}
	if _, err := tx.Exec(`UPDATE meta SET value = ? WHERE key = ?`, newEncVaultKey, metaVaultKey); err != nil {
		wipe(salt)
		wipe(newSalt)
		wipe(newEncVaultKey)
		return err
	}
	if err := tx.Commit(); err != nil {
		wipe(salt)
		wipe(newSalt)
		wipe(newEncVaultKey)
		return err
	}
	wipe(salt)
	wipe(newSalt)
	wipe(newEncVaultKey)
	return nil
}

const maxVersionsPerItem = 50

func (v *Vault) addVersion(item Item) error {
	if v.vaultKey == nil {
		return ErrVaultLocked
	}
	plain, err := json.Marshal(item)
	if err != nil {
		return err
	}
	blob, err := encrypt(v.vaultKey, plain)
	if err != nil {
		return err
	}
	id := uuid.NewString()
	ts := time.Now().UnixMilli()
	if _, err := v.db.Exec(
		`INSERT INTO item_versions (id, item_id, encrypted, ts) VALUES (?, ?, ?, ?)`,
		id, item.ID, blob, ts,
	); err != nil {
		return err
	}
	_, err = v.db.Exec(`
		DELETE FROM item_versions
		WHERE item_id = ? AND rowid NOT IN (
			SELECT rowid FROM item_versions
			WHERE item_id = ?
			ORDER BY ts DESC, rowid DESC
			LIMIT ?
		)`, item.ID, item.ID, maxVersionsPerItem)
	return err
}

func (v *Vault) getVersions(itemID string) ([]VersionEntry, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.vaultKey == nil {
		return nil, ErrVaultLocked
	}
	rows, err := v.db.Query(
		`SELECT id, encrypted, ts FROM item_versions WHERE item_id = ? ORDER BY ts DESC`,
		itemID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := []VersionEntry{}
	for rows.Next() {
		var id string
		var blob []byte
		var ts int64
		if err := rows.Scan(&id, &blob, &ts); err != nil {
			return nil, err
		}
		plain, err := decrypt(v.vaultKey, blob)
		if err != nil {
			continue
		}
		var item Item
		if err := json.Unmarshal(plain, &item); err != nil {
			continue
		}
		normalizeItem(&item)
		entries = append(entries, VersionEntry{ID: id, Timestamp: ts, Item: item})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func (v *Vault) restoreVersion(versionID string) (Item, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.vaultKey == nil {
		return Item{}, ErrVaultLocked
	}
	var blob []byte
	if err := v.db.QueryRow(`SELECT encrypted FROM item_versions WHERE id = ?`, versionID).Scan(&blob); err != nil {
		if err == sql.ErrNoRows {
			return Item{}, ErrItemNotFound
		}
		return Item{}, err
	}
	plain, err := decrypt(v.vaultKey, blob)
	if err != nil {
		return Item{}, err
	}
	var restored Item
	if err := json.Unmarshal(plain, &restored); err != nil {
		return Item{}, err
	}
	normalizeItem(&restored)

	current, ok := v.itemsBy[restored.ID]
	if !ok {
		return Item{}, ErrItemNotFound
	}
	// a restored version must pass the same checks as an edit (no duplicate
	// or invalid passkey resurrection)
	if restored.Passkey != nil {
		if err := validatePasskey(restored.ID, restored.Passkey, v.items); err != nil {
			return Item{}, err
		}
	}
	if err := v.addVersion(*current); err != nil {
		return Item{}, err
	}
	restored.UpdatedAt = time.Now().UnixMilli()
	if err := v.persistItem(restored); err != nil {
		return Item{}, err
	}
	idx := 0
	for i, it := range v.items {
		if it.ID == restored.ID {
			idx = i
			break
		}
	}
	v.items[idx] = restored
	v.sortItems()
	return restored, nil
}

func (v *Vault) setSetting(key, value string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return ErrVaultLocked
	}
	_, err := v.db.Exec(`INSERT INTO meta (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, []byte(value))
	return err
}

// getSetting must be called with the lock already held or before concurrency starts.
func (v *Vault) getSetting(key string) (string, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.getSettingLocked(key)
}

// getSettingLocked reads a setting; caller must hold at least RLock.
func (v *Vault) getSettingLocked(key string) (string, error) {
	if v.db == nil {
		return "", ErrVaultLocked
	}
	var b []byte
	err := v.db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&b)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return string(b), nil
}

// ---------- Batch operations (transactional) ----------

func (v *Vault) setCategoryBatch(ids []string, category string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	type change struct {
		idx  int
		item Item
	}
	var changes []change
	for i := range v.items {
		if !contains(ids, v.items[i].ID) {
			continue
		}
		ni := v.items[i]
		ni.Category = category
		changes = append(changes, change{i, ni})
	}
	tx, err := v.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, c := range changes {
		if err := v.persistItemTx(tx, c.item); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	for _, c := range changes {
		v.items[c.idx] = c.item
	}
	return nil
}

func (v *Vault) addTagBatch(ids []string, tag string) error {
	tag = strings.ToLower(strings.TrimSpace(tag))
	if tag == "" {
		return errors.New("tag cannot be empty")
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	type change struct {
		idx  int
		item Item
	}
	var changes []change
	for i := range v.items {
		if !contains(ids, v.items[i].ID) {
			continue
		}
		if !contains(v.items[i].Tags, tag) {
			ni := v.items[i]
			ni.Tags = append(append([]string{}, ni.Tags...), tag)
			changes = append(changes, change{i, ni})
		}
	}
	tx, err := v.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, c := range changes {
		if err := v.persistItemTx(tx, c.item); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	for _, c := range changes {
		v.items[c.idx] = c.item
	}
	return nil
}

func (v *Vault) setFavoriteBatch(ids []string, favorite bool) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	type change struct {
		idx  int
		item Item
	}
	var changes []change
	for i := range v.items {
		if !contains(ids, v.items[i].ID) {
			continue
		}
		ni := v.items[i]
		ni.Favorite = favorite
		changes = append(changes, change{i, ni})
	}
	tx, err := v.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, c := range changes {
		if err := v.persistItemTx(tx, c.item); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	for _, c := range changes {
		v.items[c.idx] = c.item
	}
	return nil
}

func (v *Vault) itemsByIDs(ids []string) []Item {
	v.mu.RLock()
	defer v.mu.RUnlock()
	out := []Item{}
	for _, it := range v.items {
		if contains(ids, it.ID) {
			out = append(out, it)
		}
	}
	return out
}

// ---------- Attachments (encrypted per file) ----------

func (v *Vault) addAttachment(itemID, name string, data []byte) (Attachment, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.vaultKey == nil {
		return Attachment{}, ErrVaultLocked
	}
	if _, ok := v.itemsBy[itemID]; !ok {
		return Attachment{}, ErrItemNotFound
	}
	if len(data) == 0 {
		return Attachment{}, errors.New("empty file")
	}
	if len(data) > maxAttachmentBytes {
		return Attachment{}, ErrTooLarge
	}
	enc, err := encrypt(v.vaultKey, data)
	if err != nil {
		return Attachment{}, err
	}
	a := Attachment{
		ID:      uuid.NewString(),
		Name:    name,
		Size:    int64(len(data)),
		AddedAt: time.Now().UnixMilli(),
	}
	_, err = v.db.Exec(
		`INSERT INTO attachments (id, item_id, name, encrypted, size, added_at) VALUES (?, ?, ?, ?, ?, ?)`,
		a.ID, itemID, a.Name, enc, a.Size, a.AddedAt,
	)
	return a, err
}

func (v *Vault) listAttachments(itemID string) ([]Attachment, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	rows, err := v.db.Query(
		`SELECT id, name, size, added_at FROM attachments WHERE item_id = ? ORDER BY added_at`,
		itemID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Attachment{}
	for rows.Next() {
		var a Attachment
		if err := rows.Scan(&a.ID, &a.Name, &a.Size, &a.AddedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (v *Vault) getAttachment(id string) (itemID, name string, data []byte, err error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	var enc []byte
	err = v.db.QueryRow(
		`SELECT item_id, name, encrypted FROM attachments WHERE id = ?`, id,
	).Scan(&itemID, &name, &enc)
	if err != nil {
		return "", "", nil, err
	}
	data, err = decrypt(v.vaultKey, enc)
	return itemID, name, data, err
}

func (v *Vault) deleteAttachment(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	_, err := v.db.Exec(`DELETE FROM attachments WHERE id = ?`, id)
	return err
}

// backupTo writes a consistent snapshot using SQLite's VACUUM INTO.
func (v *Vault) backupTo(destPath string) error {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.db == nil {
		return ErrVaultLocked
	}
	q := fmt.Sprintf(`VACUUM INTO '%s'`, strings.ReplaceAll(destPath, "'", "''"))
	_, err := v.db.Exec(q)
	return err
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func parseIntDefault(s string, def int) (int, error) {
	// absent/empty setting falls back to the default (0 must stay explicit,
	// e.g. trash_days=0 = keep forever)
	if s == "" {
		return def, nil
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return def, errors.New("invalid number")
		}
		n = n*10 + int(c-'0')
		// absurd values are treated as invalid, not silently overflowed
		if n > 1_000_000 {
			return def, errors.New("invalid number")
		}
	}
	return n, nil
}
