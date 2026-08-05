package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratePassword(t *testing.T) {
	p, err := generatePassword(PasswordOptions{Length: 24, UseUpper: true, UseLower: true, UseDigits: true, UseSymbols: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 24 {
		t.Fatalf("expected length 24, got %d", len(p))
	}
	_, err = generatePassword(PasswordOptions{Length: 0, UseLower: true})
	if err == nil {
		t.Fatal("expected error for zero length")
	}
	_, err = generatePassword(PasswordOptions{Length: 10})
	if err == nil {
		t.Fatal("expected error for no character types")
	}
}

func TestGeneratePassphrase(t *testing.T) {
	p, err := generatePassphrase(4)
	if err != nil {
		t.Fatal(err)
	}
	if p == "" {
		t.Fatal("expected non-empty passphrase")
	}
}

func TestVaultLifecycle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.passapp")

	v, err := createVault(path, "correct-horse-2026!")
	if err != nil {
		t.Fatal(err)
	}

	item, err := v.create(Item{Title: "GitHub", Username: "me", Password: "secret", URL: "https://github.com"})
	if err != nil {
		t.Fatal(err)
	}
	if item.ID == "" {
		t.Fatal("expected generated id")
	}

	item.Username = "me2"
	if err := v.update(item); err != nil {
		t.Fatal(err)
	}
	if err := v.delete(item.ID); err != nil {
		t.Fatal(err)
	}

	_, err = v.create(Item{Title: "Bank", Username: "user", Password: "pw"})
	if err != nil {
		t.Fatal(err)
	}
	v.close()

	if _, err := openVault(path, "wrong"); err != ErrWrongPassword {
		t.Fatalf("expected ErrWrongPassword, got %v", err)
	}

	v2, err := openVault(path, "correct-horse-2026!")
	if err != nil {
		t.Fatal(err)
	}
	defer v2.close()
	items := v2.list()
	if len(items) != 1 || items[0].Title != "Bank" {
		t.Fatalf("unexpected items after reopen: %+v", items)
	}
	if items[0].Password != "pw" {
		t.Fatal("password not preserved")
	}

	if err := v2.changeMasterPassword("correct-horse-2026!", "new-password-999"); err != nil {
		t.Fatal(err)
	}
	v2.close()

	v3, err := openVault(path, "new-password-999")
	if err != nil {
		t.Fatal(err)
	}
	defer v3.close()
	if len(v3.list()) != 1 {
		t.Fatal("items lost after password change")
	}
	if _, err := openVault(path, "correct-horse-2026!"); err != ErrWrongPassword {
		t.Fatal("old password should no longer work")
	}
}

func TestVaultPersistenceAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v.passapp")
	pass := "s3cr3t-master!"

	v, err := createVault(path, pass)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, err := v.create(Item{Title: "Item" + string(rune('A'+i)), Username: "u", Password: "p"}); err != nil {
			t.Fatal(err)
		}
	}
	v.close()

	v2, err := openVault(path, pass)
	if err != nil {
		t.Fatal(err)
	}
	defer v2.close()
	if len(v2.list()) != 5 {
		t.Fatalf("expected 5 items, got %d", len(v2.list()))
	}
	os.Remove(path)
}
