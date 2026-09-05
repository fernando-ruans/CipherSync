package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupSyncVault(t *testing.T, dir, name, pw string) *Vault {
	t.Helper()
	v, err := createVault(filepath.Join(dir, name), pw)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestLocalSyncRoundTrip(t *testing.T) {
	dir := t.TempDir()
	localDir := filepath.Join(dir, "sync-target")
	if err := os.MkdirAll(localDir, 0o700); err != nil {
		t.Fatal(err)
	}

	v := setupSyncVault(t, dir, "a.passapp", "pw")
	defer v.close()
	if _, err := v.create(Item{Title: "A"}); err != nil {
		t.Fatal(err)
	}
	localPath := filepath.Join(dir, "a.passapp")

	engine := &SyncEngine{provider: &localProvider{dir: localDir}, remote: "a.passapp"}

	syncOnce := func() (string, error) {
		return engine.syncFile(localPath,
			func() (syncState, error) { return loadSyncState(localPath) },
			func(s syncState) error { return saveSyncState(localPath, s) },
			func() error { return nil },
		)
	}

	// first sync: upload
	res, err := syncOnce()
	if err != nil || res != "uploaded" {
		t.Fatalf("first sync: %v %q", err, res)
	}
	if _, err := os.Stat(filepath.Join(localDir, "a.passapp")); err != nil {
		t.Fatal("remote file missing after upload")
	}

	// no changes: up to date
	res, err = syncOnce()
	if err != nil || res != "up to date" {
		t.Fatalf("second sync: %v %q", err, res)
	}

	// local change: upload again
	if _, err := v.create(Item{Title: "B"}); err != nil {
		t.Fatal(err)
	}
	res, err = syncOnce()
	if err != nil || res != "uploaded" {
		t.Fatalf("third sync: %v %q", err, res)
	}
	v.close()
}

func TestLocalSyncDownload(t *testing.T) {
	dir := t.TempDir()
	localDir := filepath.Join(dir, "sync-target")
	os.MkdirAll(localDir, 0o700)

	v := setupSyncVault(t, dir, "a.passapp", "pw")
	if _, err := v.create(Item{Title: "A"}); err != nil {
		t.Fatal(err)
	}
	localPath := filepath.Join(dir, "a.passapp")
	engine := &SyncEngine{provider: &localProvider{dir: localDir}, remote: "a.passapp"}
	syncOnce := func() (string, error) {
		return engine.syncFile(localPath,
			func() (syncState, error) { return loadSyncState(localPath) },
			func(s syncState) error { return saveSyncState(localPath, s) },
			func() error { return nil },
		)
	}
	if _, err := syncOnce(); err != nil {
		t.Fatal(err)
	}

	// simulate a newer remote copy (another device added an item)
	rv, err := createVault(filepath.Join(dir, "remote.passapp"), "pw")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rv.create(Item{Title: "A"}); err != nil {
		t.Fatal(err)
	}
	if _, err := rv.create(Item{Title: "Remote-Only"}); err != nil {
		t.Fatal(err)
	}
	rv.close()
	// force remote mtime newer + copy over the sync target
	remoteCopy := filepath.Join(localDir, "a.passapp")
	if err := copyFile(filepath.Join(dir, "remote.passapp"), remoteCopy); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Hour)
	os.Chtimes(remoteCopy, future, future)
	v.close()

	res, err := syncOnce()
	if err != nil || res != "downloaded" {
		t.Fatalf("download sync: %v %q", err, res)
	}
	// local file should now contain the remote item
	check, err := openVault(localPath, "pw")
	if err != nil {
		t.Fatal(err)
	}
	defer check.close()
	found := false
	for _, it := range check.list() {
		if it.Title == "Remote-Only" {
			found = true
		}
	}
	if !found {
		t.Fatal("downloaded vault missing remote item")
	}
}

func TestLocalSyncConflict(t *testing.T) {
	dir := t.TempDir()
	localDir := filepath.Join(dir, "sync-target")
	os.MkdirAll(localDir, 0o700)

	v := setupSyncVault(t, dir, "a.passapp", "pw")
	defer v.close()
	if _, err := v.create(Item{Title: "A"}); err != nil {
		t.Fatal(err)
	}
	localPath := filepath.Join(dir, "a.passapp")
	engine := &SyncEngine{provider: &localProvider{dir: localDir}, remote: "a.passapp"}
	syncOnce := func() (string, error) {
		return engine.syncFile(localPath,
			func() (syncState, error) { return loadSyncState(localPath) },
			func(s syncState) error { return saveSyncState(localPath, s) },
			func() error { return nil },
		)
	}
	if _, err := syncOnce(); err != nil {
		t.Fatal(err)
	}

	// both sides change: local adds one, remote (simulated newer) differs
	if _, err := v.create(Item{Title: "Local-Only"}); err != nil {
		t.Fatal(err)
	}
	v.close()
	remoteCopy := filepath.Join(localDir, "a.passapp")
	future := time.Now().Add(2 * time.Hour)
	os.Chtimes(remoteCopy, future, future)
	// touch local too so mtime differs from last sync
	now := time.Now().Add(1 * time.Hour)
	os.Chtimes(localPath, now, now)

	res, err := syncOnce()
	if err != nil {
		t.Fatal(err)
	}
	if res != "conflict: kept remote" && res != "conflict: kept local" {
		t.Fatalf("expected conflict, got %q", res)
	}
	// a conflict copy must exist next to the vault
	entries, _ := os.ReadDir(dir)
	found := false
	for _, e := range entries {
		if len(e.Name()) > len("a.passapp") && containsStr(e.Name(), "conflict") {
			found = true
		}
	}
	if !found {
		t.Fatal("conflict copy not created")
	}
}

func TestValidVaultFileSync(t *testing.T) {
	for _, f := range []string{"a.passapp"} {
		if !validVaultFile(f) {
			t.Fatalf("expected valid: %s", f)
		}
	}
	for _, f := range []string{"../x.passapp", "a.db", "/abs/a.passapp", "sub/a.passapp"} {
		if validVaultFile(f) {
			t.Fatalf("expected invalid: %s", f)
		}
	}
}
