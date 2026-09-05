package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestOpenVaultWithKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v.passapp")
	v, err := createVault(path, "correct-horse-2026!")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.create(Item{Title: "GH", Username: "u", Password: "p"}); err != nil {
		t.Fatal(err)
	}
	key := append([]byte{}, v.vaultKey...)
	v.close()

	// reopen with the raw key (no password derivation)
	v2, err := openVaultWithKey(path, key)
	if err != nil {
		t.Fatalf("openVaultWithKey: %v", err)
	}
	defer v2.close()
	items := v2.list()
	if len(items) != 1 || items[0].Title != "GH" {
		t.Fatalf("unexpected items: %+v", items)
	}
	// same key instance must be owned by the new vault now; make a fresh copy
	// to prove reopening twice works
	key2 := append([]byte{}, v2.vaultKey...)
	v2.close()
	v3, err := openVaultWithKey(path, key2)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer v3.close()
}

func TestReloadVaultAfterSync(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AppData", dir) // VaultDir() -> %AppData%\LockSync
	name := "v.passapp"
	path := filepath.Join(aVaultDirForTest(dir), name)
	v, err := createVault(path, "correct-horse-2026!")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.create(Item{Title: "A"}); err != nil {
		t.Fatal(err)
	}
	key := append([]byte{}, v.vaultKey...)

	a := &App{vault: v, vaultFile: name, vaultName: "v"}

	// simulate a remote swap: modify the DB directly while closed
	a.vaultMu.Lock()
	a.vault = nil
	a.vaultMu.Unlock()
	v.close()
	v2, err := openVault(path, "correct-horse-2026!")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v2.create(Item{Title: "B"}); err != nil {
		t.Fatal(err)
	}
	v2.close()

	// reload must reopen with the in-memory key and expose the new item
	if err := a.reloadVaultAfterSync(name, append([]byte{}, key...)); err != nil {
		t.Fatalf("reload: %v", err)
	}
	items := a.vault.list()
	if len(items) != 2 {
		t.Fatalf("expected 2 items after reload, got %d", len(items))
	}

	// reload with a bogus file must clear the app state (no vault left open)
	bogus := aVaultDirForTest(dir) + string(filepath.Separator) + "bogus.passapp"
	if err := os.WriteFile(bogus, []byte("not a vault"), 0o600); err != nil {
		t.Fatal(err)
	}
	a.vaultFile = "bogus.passapp"
	if err := a.reloadVaultAfterSync("bogus.passapp", append([]byte{}, key...)); err == nil {
		t.Fatal("expected error reloading bogus file")
	}
	if a.vault != nil {
		t.Fatal("app must not keep a vault after failed reload")
	}
}

// aVaultDirForTest mirrors App.VaultDir() layout under a temp root.
func aVaultDirForTest(root string) string {
	return filepath.Join(root, "LockSync")
}

func TestSyncNowEndToEnd(t *testing.T) {
	dir := t.TempDir()
	// point VaultDir() at the temp dir for this test
	switch runtime.GOOS {
	case "windows":
		t.Setenv("AppData", dir)
	case "darwin":
		home := dir
		t.Setenv("HOME", home)
		t.Setenv("XDG_CONFIG_HOME", "")
	default:
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
		t.Setenv("HOME", dir)
	}

	a := &App{}
	a.vaultMu.Lock()
	v, err := createVault(filepath.Join(a.VaultDir(), "e2e.passapp"), "correct-horse-2026!")
	if err != nil {
		t.Fatal(err)
	}
	a.vault, a.vaultFile, a.vaultName = v, "e2e.passapp", "e2e"
	a.vaultMu.Unlock()
	defer func() {
		a.vaultMu.Lock()
		cur := a.vault
		a.vault = nil
		a.vaultMu.Unlock()
		if cur != nil {
			cur.close()
		}
	}()

	if _, err := v.create(Item{Title: "A"}); err != nil {
		t.Fatal(err)
	}

	syncDir := filepath.Join(dir, "sync-target")
	if err := os.MkdirAll(syncDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := a.SetSyncConfig("local", syncDir); err != nil {
		t.Fatal(err)
	}

	// first sync: upload (uses VACUUM INTO snapshot)
	res, err := a.SyncNow()
	if err != nil || res != "uploaded" {
		t.Fatalf("first sync: %v %q", err, res)
	}
	if _, err := os.Stat(filepath.Join(syncDir, "e2e.passapp")); err != nil {
		t.Fatal("remote file missing")
	}

	// second sync: nothing to do
	res, err = a.SyncNow()
	if err != nil || res != "up to date" {
		t.Fatalf("second sync: %v %q", err, res)
	}

	// local change -> upload
	if _, err := v.create(Item{Title: "B"}); err != nil {
		t.Fatal(err)
	}
	res, err = a.SyncNow()
	if err != nil || res != "uploaded" {
		t.Fatalf("third sync: %v %q", err, res)
	}

	// status carries the remote
	st := a.GetSyncStatus()
	if !st.Configured || st.Remote != syncDir || st.State != "ok" {
		t.Fatalf("bad status: %+v", st)
	}

	// remote change -> download + reload keeps working vault
	a.vaultMu.Lock()
	a.vault = nil
	a.vaultMu.Unlock()
	v.close()
	vr, err := openVault(filepath.Join(syncDir, "e2e.passapp"), "correct-horse-2026!")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := vr.create(Item{Title: "Remote"}); err != nil {
		t.Fatal(err)
	}
	vr.close()
	// touch remote so mtime changes
	later := filepath.Join(syncDir, "e2e.passapp")
	newT := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(later, newT, newT); err != nil {
		t.Fatal(err)
	}

	a.vaultMu.Lock()
	v, err = openVault(filepath.Join(a.VaultDir(), "e2e.passapp"), "correct-horse-2026!")
	if err != nil {
		t.Fatal(err)
	}
	a.vault, a.vaultFile, a.vaultName = v, "e2e.passapp", "e2e"
	a.vaultMu.Unlock()

	res, err = a.SyncNow()
	if err != nil || res != "downloaded" {
		t.Fatalf("download sync: %v %q", err, res)
	}
	items := a.vault.list()
	found := false
	for _, it := range items {
		if it.Title == "Remote" {
			found = true
		}
	}
	if !found {
		t.Fatalf("remote item missing after download: %+v", items)
	}
}

func TestSyncConflictKeptLocalPreservesVault(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AppData", dir)

	a := &App{}
	a.vaultMu.Lock()
	v, err := createVault(filepath.Join(a.VaultDir(), "c1.passapp"), "correct-horse-2026!")
	if err != nil {
		t.Fatal(err)
	}
	a.vault, a.vaultFile, a.vaultName = v, "c1.passapp", "c1"
	a.vaultMu.Unlock()
	defer func() {
		a.vaultMu.Lock()
		cur := a.vault
		a.vault = nil
		a.vaultMu.Unlock()
		if cur != nil {
			cur.close()
		}
	}()

	if _, err := v.create(Item{Title: "A"}); err != nil {
		t.Fatal(err)
	}
	syncDir := filepath.Join(dir, "sync-target")
	if err := os.MkdirAll(syncDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := a.SetSyncConfig("local", syncDir); err != nil {
		t.Fatal(err)
	}
	if res, err := a.SyncNow(); err != nil || res != "uploaded" {
		t.Fatalf("initial upload: %v %q", err, res)
	}

	// both sides change; local stays NEWER (local wins)
	a.vaultMu.RLock()
	cur := a.vault
	a.vaultMu.RUnlock()
	if _, err := cur.create(Item{Title: "Local"}); err != nil {
		t.Fatal(err)
	}
	a.vaultMu.Lock()
	a.vault = nil
	a.vaultMu.Unlock()
	cur.close()
	vr, err := openVault(filepath.Join(syncDir, "c1.passapp"), "correct-horse-2026!")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := vr.create(Item{Title: "Remote"}); err != nil {
		t.Fatal(err)
	}
	vr.close()
	// reopen as the app session
	v2, err := openVault(filepath.Join(a.VaultDir(), "c1.passapp"), "correct-horse-2026!")
	if err != nil {
		t.Fatal(err)
	}
	a.vaultMu.Lock()
	a.vault, a.vaultFile, a.vaultName = v2, "c1.passapp", "c1"
	a.vaultMu.Unlock()
	future := time.Now().Add(5 * time.Second)
	if err := os.Chtimes(filepath.Join(a.VaultDir(), "c1.passapp"), future, future); err != nil {
		t.Fatal(err)
	}

	// kept-local conflict: vault must REMAIN unlocked afterwards (bug #1
	// used to close it via preSwap on the stash pull and fail the push)
	res, err := a.SyncNow()
	if err != nil || res != "conflict: kept local" {
		t.Fatalf("conflict sync: %v %q", err, res)
	}
	if !a.IsUnlocked() {
		t.Fatal("vault must stay unlocked after kept-local conflict")
	}
	conflicts, err := filepath.Glob(filepath.Join(a.VaultDir(), "c1*(conflict*.passapp"))
	if err != nil || len(conflicts) != 1 {
		t.Fatalf("remote conflict stash missing: %v %v", conflicts, err)
	}
}

func TestSyncConflictKeptRemoteReloadsVault(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AppData", dir)

	a := &App{}
	a.vaultMu.Lock()
	v, err := createVault(filepath.Join(a.VaultDir(), "c2.passapp"), "correct-horse-2026!")
	if err != nil {
		t.Fatal(err)
	}
	a.vault, a.vaultFile, a.vaultName = v, "c2.passapp", "c2"
	a.vaultMu.Unlock()

	if _, err := v.create(Item{Title: "A"}); err != nil {
		t.Fatal(err)
	}
	syncDir := filepath.Join(dir, "sync-target")
	if err := os.MkdirAll(syncDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := a.SetSyncConfig("local", syncDir); err != nil {
		t.Fatal(err)
	}
	if res, err := a.SyncNow(); err != nil || res != "uploaded" {
		t.Fatalf("initial upload: %v %q", err, res)
	}

	// both sides change; remote is NEWER (remote wins)
	a.vaultMu.Lock()
	a.vault = nil
	a.vaultMu.Unlock()
	v.close()
	vr, err := openVault(filepath.Join(syncDir, "c2.passapp"), "correct-horse-2026!")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := vr.create(Item{Title: "Remote"}); err != nil {
		t.Fatal(err)
	}
	vr.close()
	future := time.Now().Add(5 * time.Second)
	if err := os.Chtimes(filepath.Join(syncDir, "c2.passapp"), future, future); err != nil {
		t.Fatal(err)
	}
	// local change (older)
	v2, err := openVault(filepath.Join(a.VaultDir(), "c2.passapp"), "correct-horse-2026!")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v2.create(Item{Title: "Local"}); err != nil {
		t.Fatal(err)
	}
	a.vaultMu.Lock()
	a.vault, a.vaultFile, a.vaultName = v2, "c2.passapp", "c2"
	a.vaultMu.Unlock()

	res, err := a.SyncNow()
	if err != nil || res != "conflict: kept remote" {
		t.Fatalf("conflict sync: %v %q", err, res)
	}
	if !a.IsUnlocked() {
		t.Fatal("vault must be reloaded after kept-remote conflict")
	}
	items := a.vault.list()
	found := false
	for _, it := range items {
		if it.Title == "Remote" {
			found = true
		}
	}
	if !found {
		t.Fatal("remote item missing after conflict download")
	}
	localConflict, _ := filepath.Glob(filepath.Join(a.VaultDir(), "c2*(conflict*.passapp"))
	if len(localConflict) != 1 {
		t.Fatalf("local conflict copy missing: %v", localConflict)
	}
	a.vaultMu.Lock()
	cur := a.vault
	a.vault = nil
	a.vaultMu.Unlock()
	cur.close()
}
