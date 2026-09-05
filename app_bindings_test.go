package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newBoundApp returns an App with a fresh unlocked vault, with VaultDir
// redirected to a temp dir for the duration of the test.
func newBoundApp(t *testing.T) (*App, *Vault) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AppData", dir)
	a := &App{}
	v, err := createVault(filepath.Join(a.VaultDir(), "bind.passapp"), "correct-horse-2026!")
	if err != nil {
		t.Fatal(err)
	}
	a.vaultMu.Lock()
	a.vault, a.vaultFile, a.vaultName = v, "bind.passapp", "bind"
	a.vaultMu.Unlock()
	t.Cleanup(func() {
		a.vaultMu.Lock()
		a.vault = nil
		a.vaultMu.Unlock()
		v.close()
	})
	return a, v
}

func TestAppQuitFlag(t *testing.T) {
	a := &App{}
	if a.isQuitting() {
		t.Fatal("fresh app must not be quitting")
	}
	a.setQuitting()
	if !a.isQuitting() {
		t.Fatal("setQuitting must stick")
	}
}

func TestAppCloseToTraySetting(t *testing.T) {
	a, _ := newBoundApp(t)
	if !a.closeToTray() {
		t.Fatal("default must be tray-close")
	}
	if err := a.SetCloseToTray(false); err != nil {
		t.Fatal(err)
	}
	if a.closeToTray() {
		t.Fatal("false must stick")
	}
	// locked vault defaults to true again
	a.Lock()
	if !a.closeToTray() {
		t.Fatal("locked vault must default to tray-close")
	}
}

func TestAppLockClearsState(t *testing.T) {
	a, v := newBoundApp(t)
	if !a.IsUnlocked() {
		t.Fatal("expected unlocked")
	}
	name := a.GetCurrentVaultName()
	if name != "bind" {
		t.Fatalf("vault name: %q", name)
	}
	a.Lock()
	if a.IsUnlocked() {
		t.Fatal("Lock must clear the vault")
	}
	if a.vaultFile != "" || a.vaultName != "" {
		t.Fatal("Lock must clear vault metadata")
	}
	_ = v
}

func TestAppItemCRUDBindings(t *testing.T) {
	a, _ := newBoundApp(t)

	it, err := a.CreateItem(Item{Type: TypeLogin, Title: "GH", Username: "u", Password: "p", URL: "https://github.com"})
	if err != nil {
		t.Fatal(err)
	}
	items, err := a.GetItems()
	if err != nil || len(items) != 1 {
		t.Fatalf("GetItems: %v %d", err, len(items))
	}

	it.Username = "u2"
	if err := a.UpdateItem(it); err != nil {
		t.Fatal(err)
	}
	items, _ = a.GetItems()
	if items[0].Username != "u2" {
		t.Fatal("UpdateItem failed")
	}

	// versions via App bindings
	vs, err := a.GetItemVersions(it.ID)
	if err != nil || len(vs) == 0 {
		t.Fatalf("GetItemVersions: %v %d", err, len(vs))
	}
	if _, err := a.RestoreVersion(vs[0].ID); err != nil {
		t.Fatalf("RestoreVersion: %v", err)
	}

	// batch ops
	it2, err := a.CreateItem(Item{Type: TypeLogin, Title: "GL"})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SetCategoryBatch([]string{it.ID, it2.ID}, "work"); err != nil {
		t.Fatal(err)
	}
	if err := a.AddTagBatch([]string{it.ID}, "dev"); err != nil {
		t.Fatal(err)
	}
	if err := a.SetFavoriteBatch([]string{it.ID}, true); err != nil {
		t.Fatal(err)
	}
	items, _ = a.GetItems()
	for _, i := range items {
		if i.ID == it.ID && (i.Category != "work" || !i.Favorite || len(i.Tags) != 1) {
			t.Fatalf("batch ops failed: %+v", i)
		}
	}

	// trash flows
	if err := a.DeleteItems([]string{it.ID, it2.ID}); err != nil {
		t.Fatal(err)
	}
	trash, err := a.ListTrashed()
	if err != nil || len(trash) != 2 {
		t.Fatalf("ListTrashed: %v %d", err, len(trash))
	}
	if err := a.RestoreTrashed(it.ID); err != nil {
		t.Fatal(err)
	}
	if err := a.RestoreTrashedBatch([]string{it2.ID}); err != nil {
		t.Fatal(err)
	}
	items, _ = a.GetItems()
	if len(items) != 2 {
		t.Fatalf("restore batch failed: %d", len(items))
	}
	// restore non-trashed item is an error
	if err := a.RestoreTrashedBatch([]string{it.ID}); err == nil {
		t.Fatal("restoring active items must fail")
	}
	// soft delete + purge
	if err := a.DeleteItem(it.ID); err != nil {
		t.Fatal(err)
	}
	if err := a.PurgeTrashed([]string{it.ID}); err != nil {
		t.Fatal(err)
	}
	items, _ = a.GetItems()
	if len(items) != 1 {
		t.Fatalf("purge failed: %d", len(items))
	}
}

func TestAppExportsAndImports(t *testing.T) {
	a, _ := newBoundApp(t)

	if _, err := a.CreateItem(Item{Type: TypeLogin, Title: "GH", Username: "u", Password: "p", URL: "https://github.com"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateItem(Item{Type: TypeLogin, Title: "GL", Username: "u2", Password: "p2", URL: "https://gitlab.com"}); err != nil {
		t.Fatal(err)
	}

	items, _ := a.GetItems()
	csvOut, err := a.ExportSelectedCSV([]string{items[0].ID})
	if err != nil || !strings.Contains(csvOut, "GH") || strings.Contains(csvOut, "GL") {
		t.Fatalf("ExportSelectedCSV: %v %q", err, csvOut)
	}
	jsonOut, err := a.ExportSelectedJSON([]string{items[0].ID})
	if err != nil || !strings.Contains(jsonOut, "GH") || strings.Contains(jsonOut, "GL") {
		t.Fatalf("ExportSelectedJSON: %v %q", err, jsonOut)
	}
	fullCSV, err := a.ExportCSV()
	if err != nil || !strings.Contains(fullCSV, "GH") || !strings.Contains(fullCSV, "GL") {
		t.Fatalf("ExportCSV: %v", err)
	}

	// CSV import with mapping (preview) + commit
	csvData := "title,user,pass\nT1,u1,p1\nT2,u2,p2\n"
	res, err := a.ImportCSV(csvData, []FieldMapping{{Column: 0, Field: "title"}, {Column: 1, Field: "username"}, {Column: 2, Field: "password"}})
	if err != nil || len(res.Preview) != 2 {
		t.Fatalf("ImportCSV: %v %+v", err, res)
	}
	res, err = a.ImportCommit(res.Preview)
	if err != nil || res.Created != 2 {
		t.Fatalf("ImportCommit: %v %+v", err, res)
	}
	// re-import dedupes
	res, err = a.ImportCSV(csvData, []FieldMapping{{Column: 0, Field: "title"}, {Column: 1, Field: "username"}, {Column: 2, Field: "password"}})
	if err != nil {
		t.Fatal(err)
	}
	res, err = a.ImportCommit(res.Preview)
	if err != nil || res.Skipped != 2 {
		t.Fatalf("ImportCSV dedupe: %v %+v", err, res)
	}

	// auto CSV (Chrome style) — preview + commit
	auto := "name,url,username,password\nChrome GH,https://github.com,cu,cp\n"
	res, err = a.ImportAutoCSV(auto)
	if err != nil || len(res.Preview) != 1 {
		t.Fatalf("ImportAutoCSV: %v %+v", err, res)
	}
	res, err = a.ImportCommit(res.Preview)
	if err != nil || res.Created != 1 {
		t.Fatalf("ImportAutoCSV commit: %v %+v", err, res)
	}

	// Bitwarden JSON — preview + commit
	bw := `{"items":[{"type":1,"name":"BW","login":{"username":"bu","password":"bp","uris":[{"uri":"https://bitwarden.com"}]}}]}`
	res, err = a.ImportBitwardenJSON(bw)
	if err != nil || len(res.Preview) != 1 {
		t.Fatalf("ImportBitwardenJSON: %v %+v", err, res)
	}
	res, err = a.ImportCommit(res.Preview)
	if err != nil || res.Created != 1 {
		t.Fatalf("ImportBitwardenJSON commit: %v %+v", err, res)
	}

	// encrypted transfer round trip through App bindings
	sealed, err := a.ExportEncryptedJSON("xfer-pass-9")
	if err != nil {
		t.Fatal(err)
	}
	res, err = a.ImportEncryptedTransfer(sealed, "xfer-pass-9")
	if err != nil {
		t.Fatal(err)
	}
	// preview must contain every existing item (they are all duplicates)
	if len(res.Preview) != 6 {
		t.Fatalf("transfer preview: %v %+v", err, res)
	}
	res, err = a.ImportCommit(res.Preview)
	if err != nil || res.Skipped != 6 {
		t.Fatalf("expected all items deduped, got %+v", res)
	}
	if _, err := a.ImportEncryptedTransfer(sealed, "wrong"); err == nil {
		t.Fatal("wrong transfer password must fail")
	}
}

func TestAppSettingsAndBackups(t *testing.T) {
	a, _ := newBoundApp(t)

	if err := a.SetSetting("custom_key", "42"); err != nil {
		t.Fatal(err)
	}
	if err := a.SetTrashDays(7); err != nil {
		t.Fatal(err)
	}
	s, err := a.GetSettings()
	if err != nil || s["trash_days"] != "7" {
		t.Fatalf("GetSettings: %v %+v", err, s)
	}
	if err := a.SetTrashDays(-1); err == nil {
		t.Fatal("negative retention must fail")
	}
	if err := a.SetAutolockMinutes(15); err != nil {
		t.Fatal(err)
	}
	if err := a.SetAutolockMinutes(-5); err == nil {
		t.Fatal("negative autolock must fail")
	}

	name, err := a.BackupNow()
	if err != nil {
		t.Fatalf("BackupNow: %v", err)
	}
	if name == "" {
		t.Fatal("backup path empty")
	}
	// BackupNow returns the full backup file path
	if _, err := os.Stat(name); err != nil {
		t.Fatalf("backup file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(a.VaultDir(), "backups", filepath.Base(name))); err != nil {
		t.Fatalf("backup not in backups dir: %v", err)
	}
}

func TestAppSyncConfigBindings(t *testing.T) {
	a, _ := newBoundApp(t)

	// unconfigured
	if _, err := a.SyncNow(); err == nil {
		t.Fatal("sync without config must fail")
	}
	syncDir := filepath.Join(t.TempDir(), "remote")
	if err := os.MkdirAll(syncDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := a.SetSyncConfig("bogus", syncDir); err == nil {
		t.Fatal("unknown provider must fail")
	}
	if err := a.SetSyncConfig("local", filepath.Join(syncDir, "nope")); err == nil {
		t.Fatal("invalid folder must fail")
	}
	if err := a.SetSyncConfig("local", syncDir); err != nil {
		t.Fatal(err)
	}
	cfg, err := a.GetSyncConfig()
	if err != nil || cfg["provider"] != "local" {
		t.Fatalf("GetSyncConfig: %v %+v", err, cfg)
	}
	if err := a.DisconnectSync(); err != nil {
		t.Fatal(err)
	}
	cfg, _ = a.GetSyncConfig()
	if cfg["provider"] != "" || cfg["remote"] != "" {
		t.Fatalf("DisconnectSync must clear config: %+v", cfg)
	}
}

func TestAppAnalyzeAndGenerate(t *testing.T) {
	a, _ := newBoundApp(t)

	if _, err := a.CreateItem(Item{Type: TypeLogin, Title: "W", Username: "u", Password: "123456", URL: "https://x.com"}); err != nil {
		t.Fatal(err)
	}
	rep, err := a.AnalyzeVault()
	if err != nil {
		t.Fatal(err)
	}
	if rep.TotalItems < 1 {
		t.Fatalf("bad report: %+v", rep)
	}

	pw, err := a.GeneratePassword(PasswordOptions{Length: 16, UseUpper: true, UseLower: true, UseDigits: true})
	if err != nil || len(pw) != 16 {
		t.Fatalf("GeneratePassword: %v %q", err, pw)
	}
	phrase, err := a.GeneratePassphrase(3)
	if err != nil || phrase == "" {
		t.Fatalf("GeneratePassphrase: %v", err)
	}
}

func TestAppResetQuickAccessStateNoCtx(t *testing.T) {
	a := &App{}
	a.qaMu.Lock()
	a.qaOpen = true
	a.qaPrevW, a.qaPrevH = 1200, 800
	a.qaMu.Unlock()
	// no ctx: must not panic and must reset flags
	a.resetQuickAccessState()
	a.qaMu.Lock()
	defer a.qaMu.Unlock()
	if a.qaOpen || a.qaPrevW != 0 || a.qaPrevH != 0 {
		t.Fatal("resetQuickAccessState must clear qa state")
	}
}

func TestAppTransferAttachmentBindings(t *testing.T) {
	a, _ := newBoundApp(t)
	it, err := a.CreateItem(Item{Type: TypeLogin, Title: "GH"})
	if err != nil {
		t.Fatal(err)
	}
	data := base64.StdEncoding.EncodeToString([]byte("attachment-content"))
	att, err := a.AddAttachment(it.ID, "note.txt", data)
	if err != nil {
		t.Fatal(err)
	}
	atts, err := a.ListAttachments(it.ID)
	if err != nil || len(atts) != 1 {
		t.Fatalf("ListAttachments: %v %d", err, len(atts))
	}
	payload, err := a.GetAttachment(att.ID)
	if err != nil || payload.Data != data {
		t.Fatalf("GetAttachment: %v", err)
	}
	if err := a.DeleteAttachment(att.ID); err != nil {
		t.Fatal(err)
	}
	atts, _ = a.ListAttachments(it.ID)
	if len(atts) != 0 {
		t.Fatal("DeleteAttachment failed")
	}
}
