package main

import (
	"os"
	"path/filepath"
)

// localProvider mirrors vault files into a plain folder (local disk, NAS,
// Syncthing/Nextcloud folder, mounted share). The remote identifier is the
// vault filename itself.
type localProvider struct {
	dir string
}

func (p *localProvider) Name() string { return "local" }

func (p *localProvider) resolve(remote string) (string, error) {
	if !validVaultFile(remote) {
		// allow conflict copies too (same dir, "(conflict ...)" suffix)
		base := filepath.Base(remote)
		if base != remote || !isPassappLike(base) {
			return "", errInvalidRemote
		}
	}
	return filepath.Join(p.dir, filepath.Base(remote)), nil
}

func isPassappLike(name string) bool {
	if len(name) < len(".passapp") {
		return false
	}
	return name[len(name)-len(".passapp"):] == ".passapp"
}

func (p *localProvider) Stat(remote string) (SyncMeta, error) {
	path, err := p.resolve(remote)
	if err != nil {
		return SyncMeta{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return SyncMeta{Exists: false}, nil
		}
		return SyncMeta{}, err
	}
	return SyncMeta{
		Exists:  true,
		ModTime: info.ModTime().UnixMilli(),
		Size:    info.Size(),
	}, nil
}

func (p *localProvider) Download(remote, dest string) error {
	path, err := p.resolve(remote)
	if err != nil {
		return err
	}
	return copyFile(path, dest)
}

func (p *localProvider) Upload(src, remote string) (string, error) {
	path, err := p.resolve(remote)
	if err != nil {
		return "", err
	}
	srcInfo, err := os.Stat(src)
	if err != nil {
		return "", err
	}
	if err := copyFile(src, path); err != nil {
		return "", err
	}
	// preserve mtime so the next cycle sees both sides unchanged;
	// no rev for local (mtime+size rule the comparison)
	_ = os.Chtimes(path, srcInfo.ModTime(), srcInfo.ModTime())
	return "", nil
}

var errInvalidRemote = &syncError{"invalid remote path"}

type syncError struct{ msg string }

func (e *syncError) Error() string { return e.msg }
