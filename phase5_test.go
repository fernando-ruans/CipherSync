package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTrashRestorePurge(t *testing.T) {
	v := setupVault(t)
	defer v.close()
	a, _ := v.create(Item{Title: "A"})
	_, _ = v.create(Item{Title: "B"})

	if err := v.trash(a.ID); err != nil {
		t.Fatal(err)
	}
	if len(v.list()) != 1 {
		t.Fatalf("expected 1 active, got %d", len(v.list()))
	}
	if len(v.listTrashed()) != 1 {
		t.Fatal("expected 1 trashed")
	}

	if err := v.restoreTrashed(a.ID); err != nil {
		t.Fatal(err)
	}
	if len(v.list()) != 2 || len(v.listTrashed()) != 0 {
		t.Fatal("restore failed")
	}

	if err := v.trash(a.ID); err != nil {
		t.Fatal(err)
	}
	if err := v.purgeItems([]string{a.ID}); err != nil {
		t.Fatal(err)
	}
	if len(v.list()) != 1 || len(v.listTrashed()) != 0 {
		t.Fatal("purge failed")
	}
	if _, err := v.getItem(a.ID); err != ErrItemNotFound {
		t.Fatal("expected item to be gone")
	}
}

func TestTrashAutoPurge(t *testing.T) {
	v := setupVault(t)
	defer v.close()
	old, _ := v.create(Item{Title: "Old"})
	recent, _ := v.create(Item{Title: "Recent"})

	if err := v.trashItems([]string{old.ID, recent.ID}); err != nil {
		t.Fatal(err)
	}
	// backdate one
	for i := range v.items {
		if v.items[i].ID == old.ID {
			v.items[i].DeletedAt = time.Now().AddDate(0, 0, -40).UnixMilli()
			if err := v.persistItem(v.items[i]); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := v.purgeExpiredTrash(30); err != nil {
		t.Fatal(err)
	}
	if len(v.listTrashed()) != 1 || v.listTrashed()[0].ID != recent.ID {
		t.Fatalf("auto purge failed: %+v", v.listTrashed())
	}
}

func TestAttachments(t *testing.T) {
	v := setupVault(t)
	defer v.close()
	it, _ := v.create(Item{Title: "Doc"})

	data := []byte("hello world contents")
	a, err := v.addAttachment(it.ID, "nota.txt", data)
	if err != nil {
		t.Fatal(err)
	}
	if a.Size != int64(len(data)) || a.Name != "nota.txt" {
		t.Fatalf("bad attachment meta: %+v", a)
	}
	list, err := v.listAttachments(it.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("expected 1 attachment: %v %+v", err, list)
	}
	_, name, got, err := v.getAttachment(a.ID)
	if err != nil || name != "nota.txt" || string(got) != string(data) {
		t.Fatal("attachment round trip failed")
	}
	if err := v.deleteAttachment(a.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := v.listAttachments(it.ID); len(list) != 0 {
		t.Fatal("delete failed")
	}

	big := make([]byte, maxAttachmentBytes+1)
	if _, err := v.addAttachment(it.ID, "big.bin", big); err != ErrTooLarge {
		t.Fatalf("expected ErrTooLarge, got %v", err)
	}
	if _, err := v.addAttachment("nope", "x", data); err != ErrItemNotFound {
		t.Fatalf("expected ErrItemNotFound, got %v", err)
	}
}

func TestBackupTo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v.passapp")
	v, err := createVault(path, "backup-pw")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.create(Item{Title: "Keep", Password: "x"}); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "snap.passapp")
	if err := v.backupTo(dest); err != nil {
		t.Fatal(err)
	}
	v.close()

	v2, err := openVault(dest, "backup-pw")
	if err != nil {
		t.Fatal(err)
	}
	defer v2.close()
	if len(v2.list()) != 1 || v2.list()[0].Title != "Keep" {
		t.Fatalf("backup contents wrong: %+v", v2.list())
	}
}

func TestBOMAndSemicolonCSV(t *testing.T) {
	bom := string(rune(0xFEFF))
	data := bom + "title;username;password\nGitHub;ghuser;ghpass\n"
	items, err := parseAutoCSV(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "GitHub" || items[0].Password != "ghpass" {
		t.Fatalf("BOM/semicolon parse failed: %+v", items)
	}
}

func TestImportDedupePassword(t *testing.T) {
	v := setupVault(t)
	defer v.close()
	res := v.importItems([]Item{
		{Title: "Site", Username: "u", Password: "old"},
		{Title: "Site", Username: "u", Password: "new"},
	})
	if res.Created != 2 {
		t.Fatalf("expected both variants imported, got %+v", res)
	}
}

func TestZxcvbnScore(t *testing.T) {
	if goPasswordScore("password") != 0 {
		t.Fatal("common password should score 0")
	}
	if goPasswordScore("Password123") > 2 {
		t.Fatalf("Password123 should be weak-ish, got %d", goPasswordScore("Password123"))
	}
	if goPasswordScore("K7#mPq2!vXz9T4$") < 3 {
		t.Fatalf("strong random should score >= 3, got %d", goPasswordScore("K7#mPq2!vXz9T4$"))
	}
}

func TestValidVaultFile(t *testing.T) {
	for _, f := range []string{"a.passapp", "meu-cofre.passapp"} {
		if !validVaultFile(f) {
			t.Fatalf("expected valid: %s", f)
		}
	}
	for _, f := range []string{"", "../x.passapp", "a.db", "C:\\x\\a.passapp", "a.PASSAPP"} {
		if validVaultFile(f) {
			t.Fatalf("expected invalid: %s", f)
		}
	}
}

func TestSemicolonManualCSV(t *testing.T) {
	data := "Site;Usuario;Senha\nA;u;p\n"
	m := []FieldMapping{{Column: 0, Field: "title"}, {Column: 1, Field: "username"}, {Column: 2, Field: "password"}}
	items, err := parseCSV(data, m)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Username != "u" {
		t.Fatalf("manual semicolon parse failed: %+v", items)
	}
	if !strings.Contains(data, ";") {
		t.Fatal("sanity")
	}
	_ = os.DevNull
}
