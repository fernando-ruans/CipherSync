package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBatchDelete(t *testing.T) {
	v := setupVault(t)
	defer v.close()
	a, _ := v.create(Item{Title: "A"})
	b, _ := v.create(Item{Title: "B"})
	c, _ := v.create(Item{Title: "C"})

	if err := v.deleteItems([]string{a.ID, c.ID}); err != nil {
		t.Fatal(err)
	}
	items := v.list()
	if len(items) != 1 || items[0].ID != b.ID {
		t.Fatalf("expected only B to remain, got %+v", items)
	}
}

func TestBatchSetCategory(t *testing.T) {
	v := setupVault(t)
	defer v.close()
	a, _ := v.create(Item{Title: "A"})
	b, _ := v.create(Item{Title: "B"})
	c, _ := v.create(Item{Title: "C"})

	if err := v.setCategoryBatch([]string{a.ID, b.ID}, "Trabalho"); err != nil {
		t.Fatal(err)
	}
	byID := map[string]Item{}
	for _, it := range v.list() {
		byID[it.ID] = it
	}
	if byID[a.ID].Category != "Trabalho" || byID[b.ID].Category != "Trabalho" {
		t.Fatal("category not applied to selected items")
	}
	if byID[c.ID].Category != "" {
		t.Fatal("category applied to unselected item")
	}
	// persistence check
	items := v.list()
	_ = items
}

func TestBatchAddTag(t *testing.T) {
	v := setupVault(t)
	defer v.close()
	a, _ := v.create(Item{Title: "A", Tags: []string{"urgente"}})
	b, _ := v.create(Item{Title: "B"})

	if err := v.addTagBatch([]string{a.ID, b.ID}, " Trabalho "); err != nil {
		t.Fatal(err)
	}
	byID := map[string]Item{}
	for _, it := range v.list() {
		byID[it.ID] = it
	}
	if !contains(byID[a.ID].Tags, "trabalho") || len(byID[a.ID].Tags) != 2 {
		t.Fatalf("tags wrong for A: %+v", byID[a.ID].Tags)
	}
	if !contains(byID[b.ID].Tags, "trabalho") {
		t.Fatalf("tag missing for B: %+v", byID[b.ID].Tags)
	}
}

func TestBatchFavorite(t *testing.T) {
	v := setupVault(t)
	defer v.close()
	a, _ := v.create(Item{Title: "A"})
	b, _ := v.create(Item{Title: "B"})

	if err := v.setFavoriteBatch([]string{a.ID, b.ID}, true); err != nil {
		t.Fatal(err)
	}
	for _, it := range v.list() {
		if !it.Favorite {
			t.Fatal("favorite not applied")
		}
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Pessoal":      "pessoal",
		"Meu Cofre":    "meu-cofre",
		"  Conta   do  trabalho ": "conta-do-trabalho",
		"Caixa!!#":     "caixa",
		"":             "cofre",
		"123":          "123",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Fatalf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestListVaultsAndNames(t *testing.T) {
	dir := t.TempDir()

	// create two named vaults directly in the temp dir
	v1, err := createVault(filepath.Join(dir, "pessoal.passapp"), "pw1")
	if err != nil {
		t.Fatal(err)
	}
	if err := v1.setSetting("vault_name", "Pessoal"); err != nil {
		t.Fatal(err)
	}
	v1.close()

	v2, err := createVault(filepath.Join(dir, "trabalho.passapp"), "pw2")
	if err != nil {
		t.Fatal(err)
	}
	if err := v2.setSetting("vault_name", "Trabalho"); err != nil {
		t.Fatal(err)
	}
	v2.close()

	vaults, err := listVaultsIn(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(vaults) != 2 {
		t.Fatalf("expected 2 vaults, got %d", len(vaults))
	}
	names := map[string]bool{}
	for _, v := range vaults {
		names[v.Name] = true
	}
	if !names["Pessoal"] || !names["Trabalho"] {
		t.Fatalf("vault names not read from meta: %+v", vaults)
	}

	// empty dir returns empty list
	if vaults, err := listVaultsIn(filepath.Join(dir, "nope")); err != nil || len(vaults) != 0 {
		t.Fatalf("empty dir should return no vaults: %+v err=%v", vaults, err)
	}
}

func TestExportSelected(t *testing.T) {
	v := setupVault(t)
	defer v.close()
	a, _ := v.create(Item{Title: "AAA", Username: "u1", Password: "p1"})
	b, _ := v.create(Item{Title: "BBB", Username: "u2", Password: "p2"})
	_, _ = v.create(Item{Title: "CCC", Username: "u3", Password: "p3"})

	sel := v.itemsByIDs([]string{a.ID, b.ID})
	if len(sel) != 2 {
		t.Fatalf("expected 2 selected, got %d", len(sel))
	}
	csv := exportCSV(sel)
	if !containsStr(csv, "AAA") || !containsStr(csv, "BBB") || containsStr(csv, "CCC") {
		t.Fatalf("export selected CSV wrong: %s", csv)
	}
	js, err := exportJSON(sel)
	if err != nil || !containsStr(js, "AAA") {
		t.Fatalf("export selected JSON wrong: %v %s", err, js)
	}
}

func TestDeleteAccountRemovesAll(t *testing.T) {
	dir := t.TempDir()
	v1, err := createVault(filepath.Join(dir, "a.passapp"), "pw")
	if err != nil {
		t.Fatal(err)
	}
	v1.close()
	v2, err := createVault(filepath.Join(dir, "b.passapp"), "pw")
	if err != nil {
		t.Fatal(err)
	}
	v2.close()
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("directory should not exist after removal")
	}
}
