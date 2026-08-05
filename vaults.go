package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func slugify(name string) string {
	fields := strings.Fields(strings.ToLower(name))
	var parts []string
	for _, f := range fields {
		var sb strings.Builder
		for _, r := range f {
			switch {
			case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
				sb.WriteRune(r)
			case r == '-' || r == '_':
				sb.WriteByte('-')
			}
		}
		if s := strings.Trim(sb.String(), "-"); s != "" {
			parts = append(parts, s)
		}
	}
	slug := strings.Join(parts, "-")
	if slug == "" {
		slug = "cofre"
	}
	return slug
}

func readVaultName(path string) (string, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return "", err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var b []byte
	err = db.QueryRow(`SELECT value FROM meta WHERE key = 'vault_name'`).Scan(&b)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func listVaultsIn(dir string) ([]VaultInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []VaultInfo{}, nil
		}
		return nil, err
	}
	vaults := []VaultInfo{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".passapp") {
			continue
		}
		info, _ := e.Info()
		v := VaultInfo{
			File:        e.Name(),
			Name:        strings.TrimSuffix(e.Name(), ".passapp"),
			LastOpened:  info.ModTime().UnixMilli(),
			HelloEnabled: fileExists(filepath.Join(dir, helloBlobName(e.Name()))),
		}
		if name, err := readVaultName(filepath.Join(dir, e.Name())); err == nil && name != "" {
			v.Name = name
		}
		vaults = append(vaults, v)
	}
	sort.Slice(vaults, func(i, j int) bool {
		return vaults[i].LastOpened > vaults[j].LastOpened
	})
	return vaults, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
