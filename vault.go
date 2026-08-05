package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

var (
	ErrWrongPassword = errors.New("wrong master password")
	ErrVaultLocked   = errors.New("vault is locked")
	ErrItemNotFound  = errors.New("item not found")
	ErrNotAVault     = errors.New("not a valid LockSync vault")
)

const metaSaltKey = "kdf_salt"
const metaParamsKey = "kdf_params"
const metaVaultKey = "encrypted_vault_key"

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
	}
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

type Vault struct {
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
		itemCopy := item
		v.items = append(v.items, itemCopy)
		v.itemsBy[item.ID] = &v.items[len(v.items)-1]
	}
	sort.Slice(v.items, func(i, j int) bool {
		return v.items[i].Title < v.items[j].Title
	})
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

func (v *Vault) close() {
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

func (v *Vault) list() []Item {
	out := make([]Item, len(v.items))
	copy(out, v.items)
	return out
}

func (v *Vault) create(input Item) (Item, error) {
	normalizeItem(&input)
	if input.Type == "" {
		input.Type = TypeLogin
	}
	now := time.Now().UnixMilli()
	input.ID = uuid.NewString()
	input.CreatedAt = now
	input.UpdatedAt = now
	if err := v.persistItem(input); err != nil {
		return Item{}, err
	}
	v.items = append(v.items, input)
	v.itemsBy[input.ID] = &v.items[len(v.items)-1]
	sort.Slice(v.items, func(i, j int) bool {
		return v.items[i].Title < v.items[j].Title
	})
	return input, nil
}

func (v *Vault) update(input Item) error {
	existing, ok := v.itemsBy[input.ID]
	if !ok {
		return ErrItemNotFound
	}
	normalizeItem(&input)
	if err := v.addVersion(*existing); err != nil {
		return err
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
	v.itemsBy[input.ID] = &v.items[idx]
	sort.Slice(v.items, func(i, j int) bool {
		return v.items[i].Title < v.items[j].Title
	})
	return nil
}

func (v *Vault) delete(id string) error {
	if _, ok := v.itemsBy[id]; !ok {
		return ErrItemNotFound
	}
	if _, err := v.db.Exec(`DELETE FROM items WHERE id = ?`, id); err != nil {
		return err
	}
	_, _ = v.db.Exec(`DELETE FROM item_versions WHERE item_id = ?`, id)
	delete(v.itemsBy, id)
	for i, it := range v.items {
		if it.ID == id {
			v.items = append(v.items[:i], v.items[i+1:]...)
			break
		}
	}
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

func (v *Vault) changeMasterPassword(oldPassword, newPassword string) error {
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
	_, err = v.db.Exec(`UPDATE meta SET value = ? WHERE key = ?`, newSalt, metaSaltKey)
	if err == nil {
		_, err = v.db.Exec(`UPDATE meta SET value = ? WHERE key = ?`, newEncVaultKey, metaVaultKey)
	}
	wipe(salt)
	wipe(newSalt)
	wipe(newEncVaultKey)
	return err
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
	return entries, nil
}

func (v *Vault) restoreVersion(versionID string) (Item, error) {
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
	v.itemsBy[restored.ID] = &v.items[idx]
	sort.Slice(v.items, func(i, j int) bool {
		return v.items[i].Title < v.items[j].Title
	})
	return restored, nil
}

func (v *Vault) setSetting(key, value string) error {
	if v.db == nil {
		return ErrVaultLocked
	}
	_, err := v.db.Exec(`INSERT INTO meta (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, []byte(value))
	return err
}

// ---------- Batch operations ----------

func (v *Vault) deleteItems(ids []string) error {
	if v.vaultKey == nil {
		return ErrVaultLocked
	}
	valid := []string{}
	keep := make([]Item, 0, len(v.items))
	for _, it := range v.items {
		if contains(ids, it.ID) {
			valid = append(valid, it.ID)
		} else {
			keep = append(keep, it)
		}
	}
	for _, id := range valid {
		if _, err := v.db.Exec(`DELETE FROM items WHERE id = ?`, id); err != nil {
			return err
		}
		_, _ = v.db.Exec(`DELETE FROM item_versions WHERE item_id = ?`, id)
	}
	v.items = keep
	v.itemsBy = map[string]*Item{}
	for i := range v.items {
		v.itemsBy[v.items[i].ID] = &v.items[i]
	}
	return nil
}

func (v *Vault) setCategoryBatch(ids []string, category string) error {
	for i := range v.items {
		if contains(ids, v.items[i].ID) {
			v.items[i].Category = category
			if err := v.persistItem(v.items[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (v *Vault) addTagBatch(ids []string, tag string) error {
	tag = strings.ToLower(strings.TrimSpace(tag))
	if tag == "" {
		return errors.New("tag cannot be empty")
	}
	for i := range v.items {
		if contains(ids, v.items[i].ID) {
			if !contains(v.items[i].Tags, tag) {
				v.items[i].Tags = append(v.items[i].Tags, tag)
				if err := v.persistItem(v.items[i]); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (v *Vault) setFavoriteBatch(ids []string, favorite bool) error {
	for i := range v.items {
		if contains(ids, v.items[i].ID) {
			v.items[i].Favorite = favorite
			if err := v.persistItem(v.items[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (v *Vault) itemsByIDs(ids []string) []Item {
	out := []Item{}
	for _, it := range v.items {
		if contains(ids, it.ID) {
			out = append(out, it)
		}
	}
	return out
}

func (v *Vault) getSetting(key string) (string, error) {
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

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
