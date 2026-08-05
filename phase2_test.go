package main

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func setupVault(t *testing.T) *Vault {
	t.Helper()
	dir := t.TempDir()
	v, err := createVault(filepath.Join(dir, "v.passapp"), "test-master-pw")
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestVersionHistory(t *testing.T) {
	v := setupVault(t)
	defer v.close()

	item, err := v.create(Item{Type: TypeLogin, Title: "GitHub", Username: "me", Password: "v1"})
	if err != nil {
		t.Fatal(err)
	}

	item.Password = "v2"
	if err := v.update(item); err != nil {
		t.Fatal(err)
	}
	item.Password = "v3"
	if err := v.update(item); err != nil {
		t.Fatal(err)
	}

	versions, err := v.getVersions(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
	if versions[0].Item.Password != "v2" {
		t.Fatalf("newest version should hold v2, got %q", versions[0].Item.Password)
	}
	if versions[1].Item.Password != "v1" {
		t.Fatalf("oldest version should hold v1, got %q", versions[1].Item.Password)
	}

	restored, err := v.restoreVersion(versions[1].ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Password != "v1" {
		t.Fatalf("restored password should be v1, got %q", restored.Password)
	}
	if restored.Title != "GitHub" || restored.Username != "me" {
		t.Fatalf("restored item corrupted: %+v", restored)
	}
	// now there should be 3 versions (v1 snapshot before restore)
	versions2, _ := v.getVersions(item.ID)
	if len(versions2) != 3 {
		t.Fatalf("expected 3 versions after restore, got %d", len(versions2))
	}
}

func TestImportAutoCSV(t *testing.T) {
	data := "url,username,password,totp,extra,name,grouping,fav\n" +
		"https://a.com,alice,pw123,,note1,Alice Account,Social,0\n" +
		"https://b.com,bob,pw456,,,Bob Site,Work,1\n" +
		",,,,," // should be skipped
	items, err := parseAutoCSV(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Title != "Alice Account" || items[0].Username != "alice" || items[0].URL != "https://a.com" {
		t.Fatalf("bad item: %+v", items[0])
	}
	if items[0].Category != "Social" || items[1].Category != "Work" {
		t.Fatalf("category mapping failed: %+v", items)
	}
}

func TestImportCSVManualMapping(t *testing.T) {
	data := "Site,User,Pass\nGitHub,ghuser,ghpass\n"
	mapping := []FieldMapping{{Column: 0, Field: "title"}, {Column: 1, Field: "username"}, {Column: 2, Field: "password"}}
	items, err := parseCSV(data, mapping)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "GitHub" || items[0].Password != "ghpass" {
		t.Fatalf("manual mapping failed: %+v", items)
	}
}

func TestImportBitwardenJSON(t *testing.T) {
	data := `{
	  "encrypted": false,
	  "folders": [{"id": "f1", "name": "Work"}],
	  "items": [
	    {"id": "i1", "name": "GitHub", "folderId": "f1", "favorite": true,
	     "login": {"username": "bwuser", "password": "bwpass", "uris": [{"uri": "https://github.com"}]}},
	    {"id": "i2", "name": "My Note", "type": 1, "secureNote": {}},
	    {"id": "i3", "name": "Visa", "type": 2, "card": {
	       "cardholderName": "Joao", "brand": "Visa", "number": "4111111111111111",
	       "expMonth": "12", "expYear": "2028", "code": "123"}}
	  ]
	}`
	items, err := parseBitwardenJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0].Category != "Work" || items[0].Username != "bwuser" || !items[0].Favorite {
		t.Fatalf("bad login import: %+v", items[0])
	}
	if items[1].Type != TypeNote {
		t.Fatalf("expected note type, got %q", items[1].Type)
	}
	if items[2].Type != TypeCreditCard || items[2].Fields["number"] != "4111111111111111" {
		t.Fatalf("bad card import: %+v", items[2])
	}
}

func TestImportCommitDedupe(t *testing.T) {
	v := setupVault(t)
	defer v.close()
	_, _ = v.create(Item{Title: "GitHub", Username: "me"})

	res := v.importItems([]Item{
		{Title: "GitHub", Username: "me"},   // duplicate
		{Title: "GitHub", Username: "other"}, // distinct
		{Title: "New", Username: "x"},
	})
	if res.Created != 2 || res.Skipped != 1 {
		t.Fatalf("expected 2 created 1 skipped, got %+v", res)
	}
}

func TestTransferRoundTrip(t *testing.T) {
	v := setupVault(t)
	defer v.close()
	item, _ := v.create(Item{Title: "Bank", Username: "u", Password: "s3cret"})

	sealed, err := sealTransfer(v.list(), "transfer-pw")
	if err != nil {
		t.Fatal(err)
	}
	if sealed == "" {
		t.Fatal("empty transfer")
	}

	items, err := openTransfer(sealed, "transfer-pw")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != item.ID || items[0].Password != "s3cret" {
		t.Fatalf("bad round trip: %+v", items)
	}

	if _, err := openTransfer(sealed, "wrong-pw"); err != ErrWrongPassword {
		t.Fatalf("expected ErrWrongPassword, got %v", err)
	}
}

func TestSettingsPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v.passapp")
	v, err := createVault(path, "pw")
	if err != nil {
		t.Fatal(err)
	}
	if err := v.setSetting("autolock_minutes", "15"); err != nil {
		t.Fatal(err)
	}
	v.close()

	v2, err := openVault(path, "pw")
	if err != nil {
		t.Fatal(err)
	}
	defer v2.close()
	got, err := v2.getSetting("autolock_minutes")
	if err != nil || got != "15" {
		t.Fatalf("expected setting 15, got %q err %v", got, err)
	}
}

func TestExportCSV(t *testing.T) {
	v := setupVault(t)
	defer v.close()
	_, _ = v.create(Item{Title: "A", Username: "u", Password: "p"})
	csv := exportCSV(v.list())
	if !containsStr(csv, "A") || !containsStr(csv, "username") {
		t.Fatalf("bad csv: %s", csv)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestOnePasswordCSVFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "1password_export.csv"))
	if err != nil {
		t.Fatal(err)
	}
	items, err := parseAutoCSV(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) < 40 {
		t.Fatalf("expected dozens of entries, got %d", len(items))
	}
	for _, it := range items {
		if it.Title == "" || it.Username == "" || it.Password == "" {
			t.Fatalf("item missing required field: %+v", it)
		}
	}
	if items[0].Title != "GitHub" || items[0].URL != "https://github.com" {
		t.Fatalf("first item wrong: %+v", items[0])
	}
	// URLs from the "Website URL" column must map correctly
	seenURL := false
	for _, it := range items {
		if it.URL != "" {
			seenURL = true
			break
		}
	}
	if !seenURL {
		t.Fatal("no URLs were mapped from the Website URL column")
	}
}

func TestChromeCSVFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "chrome_passwords.csv"))
	if err != nil {
		t.Fatal(err)
	}
	items, err := parseAutoCSV(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) < 20 {
		t.Fatalf("expected dozens of entries, got %d", len(items))
	}
	for _, it := range items {
		if it.Title == "" || it.Username == "" || it.Password == "" || it.URL == "" {
			t.Fatalf("chrome item missing required field: %+v", it)
		}
	}
	if items[0].Title != "GitHub" || items[0].Username != "ana.dev@gmail.com" || items[0].URL != "https://github.com" {
		t.Fatalf("first chrome item wrong: %+v", items[0])
	}
}

func TestFirefoxCSVTitleFallback(t *testing.T) {
	data := "url,username,password\n" +
		"https://github.com/login,ffuser,ffpass\n" +
		"https://mail.google.com,ff.mail,ffpw2\n"
	items, err := parseAutoCSV(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Title != "github.com" || items[0].URL != "https://github.com/login" {
		t.Fatalf("title fallback failed: %+v", items[0])
	}
}

func TestBitwardenJSONFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "bitwarden_export.json"))
	if err != nil {
		t.Fatal(err)
	}
	items, err := parseBitwardenJSON(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 20 {
		t.Fatalf("expected 20 items, got %d", len(items))
	}

	counts := map[string]int{}
	for _, it := range items {
		counts[it.Type]++
	}
	if counts[TypeLogin] != 6 {
		t.Fatalf("expected 6 logins, got %d", counts[TypeLogin])
	}
	if counts[TypeNote] != 5 {
		t.Fatalf("expected 5 notes, got %d", counts[TypeNote])
	}
	if counts[TypeCreditCard] != 5 {
		t.Fatalf("expected 5 credit cards, got %d", counts[TypeCreditCard])
	}
	if counts[TypeIdentity] != 4 {
		t.Fatalf("expected 4 identities, got %d", counts[TypeIdentity])
	}

	var card *Item
	var identity *Item
	var note *Item
	for i := range items {
		switch items[i].Type {
		case TypeCreditCard:
			if card == nil {
				card = &items[i]
			}
		case TypeIdentity:
			if identity == nil {
				identity = &items[i]
			}
		case TypeNote:
			if note == nil {
				note = &items[i]
			}
		}
	}
	if card.Fields["number"] != "4111111111111111" || card.Fields["cardholder"] != "ANA C FERREIRA" || card.Fields["expiry"] != "12/28" {
		t.Fatalf("card fields wrong: %+v", card.Fields)
	}
	if identity.Fields["fullName"] != "Ana Costa Ferreira" {
		t.Fatalf("identity fullName wrong: %q", identity.Fields["fullName"])
	}
	if note.Title == "" || note.Notes == "" {
		t.Fatalf("note item incomplete: %+v", note)
	}
}

func TestMigrateOldSchemaVault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v.passapp")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	for _, q := range []string{
		`CREATE TABLE items (id TEXT PRIMARY KEY, encrypted BLOB NOT NULL, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE meta (key TEXT PRIMARY KEY, value BLOB NOT NULL)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	salt, _ := randomBytes(saltLen)
	vaultKey, _ := randomBytes(keyLen)
	params := newKDFParams()
	masterKey := deriveKey("old-pw", salt, params)
	encKey, _ := encrypt(masterKey, vaultKey)
	paramsJSON, _ := json.Marshal(params)
	for k, v := range map[string][]byte{metaSaltKey: salt, metaParamsKey: paramsJSON, metaVaultKey: encKey} {
		if _, err := db.Exec(`INSERT INTO meta (key, value) VALUES (?, ?)`, k, v); err != nil {
			t.Fatal(err)
		}
	}
	item := Item{ID: "i1", Title: "Old", Username: "u", Password: "p"}
	plain, _ := json.Marshal(item)
	blob, _ := encrypt(vaultKey, plain)
	if _, err := db.Exec(`INSERT INTO items (id, encrypted, created_at, updated_at) VALUES (?, ?, ?, ?)`, "i1", blob, 1, 1); err != nil {
		t.Fatal(err)
	}
	db.Close()

	v, err := openVault(path, "old-pw")
	if err != nil {
		t.Fatal(err)
	}
	defer v.close()
	if len(v.list()) != 1 || v.list()[0].Title != "Old" {
		t.Fatalf("migration lost items: %+v", v.list())
	}
	if _, err := v.getVersions("i1"); err != nil {
		t.Fatalf("item_versions table missing after migration: %v", err)
	}
}
