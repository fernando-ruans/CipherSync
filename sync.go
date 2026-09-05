package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// SyncMeta describes a vault file on either side.
type SyncMeta struct {
	Exists  bool   `json:"exists"`
	ModTime int64  `json:"modTime"`
	Size    int64  `json:"size"`
	Rev     string `json:"rev"`
}

// SyncProvider abstracts a sync target (local folder, Drive, ...).
type SyncProvider interface {
	Name() string
	// Stat returns metadata for the remote vault file (empty path = not found).
	Stat(remote string) (SyncMeta, error)
	// Download copies the remote file to a local temp path.
	Download(remote, dest string) error
	// Upload copies a local file to the remote path, returns the new rev.
	Upload(src, remote string) (string, error)
}

// SyncStatus is reported to the UI.
type SyncStatus struct {
	Configured bool   `json:"configured"`
	Provider   string `json:"provider"`
	Remote     string `json:"remote"`
	State      string `json:"state"`
	LastSync   int64  `json:"lastSync"`
	Detail     string `json:"detail"`
	Conflict   string `json:"conflict"`
}

// syncState tracks the last synced revisions. It lives in a sidecar JSON
// next to the vault file (NOT inside the vault DB) so download-swaps
// cannot clobber it.
type syncState struct {
	RemoteRev   string `json:"remoteRev"`
	RemoteMtime int64  `json:"remoteMtime"`
	RemoteSize  int64  `json:"remoteSize"`
	LocalMtime  int64  `json:"localMtime"`
	LastSync    int64  `json:"lastSync"`
}

func syncStatePath(vaultPath string) string {
	return strings.TrimSuffix(vaultPath, filepath.Ext(vaultPath)) + ".sync.json"
}

func loadSyncState(vaultPath string) (syncState, error) {
	var st syncState
	raw, err := os.ReadFile(syncStatePath(vaultPath))
	if err != nil {
		if os.IsNotExist(err) {
			return syncState{}, nil
		}
		return syncState{}, err
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		return syncState{}, nil // corrupt state: start fresh, engine re-syncs by content
	}
	return st, nil
}

func saveSyncState(vaultPath string, st syncState) error {
	raw, err := json.Marshal(st)
	if err != nil {
		return err
	}
	tmp := syncStatePath(vaultPath) + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, syncStatePath(vaultPath))
}

func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// SyncEngine performs whole-file last-write-wins sync with conflict copies.
type SyncEngine struct {
	mu       sync.Mutex
	provider SyncProvider
	remote   string
	// snapshot returns a consistent copy of the live local DB (VACUUM INTO).
	// Falls back to a plain copy when unset.
	snapshot func() (string, error)
	// preSwap runs just before the local file is replaced on pull (e.g.
	// close the vault so Windows allows the swap).
	preSwap func() error
}

func (e *SyncEngine) syncFile(localPath string, loadState func() (syncState, error), saveState func(syncState) error, reload func() error) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	st, err := loadState()
	if err != nil {
		return "", err
	}

	localInfo, localErr := os.Stat(localPath)
	remote, err := e.provider.Stat(e.remote)
	if err != nil {
		return "", fmt.Errorf("remote: %w", err)
	}

	localMtime := int64(0)
	if localErr == nil {
		localMtime = localInfo.ModTime().UnixMilli()
	}

	localChanged := localErr == nil && (st.LocalMtime == 0 || localMtime != st.LocalMtime)
	// rev is only compared when both sides report one (Drive); otherwise mtime rules
	revChanged := st.RemoteRev != "" && remote.Rev != "" && remote.Rev != st.RemoteRev
	remoteChanged := remote.Exists && (st.RemoteMtime == 0 || remote.ModTime != st.RemoteMtime || revChanged)
	now := time.Now().UnixMilli()

	// Fresh state (first run or reset sidecar): don't blindly declare a
	// conflict when both sides "changed". Compare content instead; only
	// fall through to the both-changed branch when they truly differ.
	if localChanged && remoteChanged && st.LocalMtime == 0 && st.RemoteMtime == 0 {
		same, herr := e.remoteMatches(localPath)
		if herr != nil {
			return "", herr
		}
		if same {
			_ = saveState(syncState{RemoteRev: remote.Rev, RemoteMtime: remote.ModTime, RemoteSize: remote.Size, LocalMtime: localMtime, LastSync: now})
			return "up to date", nil
		}
	}

	switch {
	case !remote.Exists:
		// first sync or remote deleted: push local
		if localErr != nil {
			return "nothing to sync", nil
		}
		rev, err := e.push(localPath)
		if err != nil {
			return "", err
		}
		_ = saveState(syncState{RemoteRev: rev, RemoteMtime: localMtime, RemoteSize: localInfo.Size(), LocalMtime: localMtime, LastSync: now})
		return "uploaded", nil

	case !localChanged && !remoteChanged:
		_ = saveState(syncState{RemoteRev: st.RemoteRev, RemoteMtime: st.RemoteMtime, RemoteSize: st.RemoteSize, LocalMtime: localMtime, LastSync: now})
		return "up to date", nil

	case localChanged && !remoteChanged:
		rev, err := e.push(localPath)
		if err != nil {
			return "", err
		}
		_ = saveState(syncState{RemoteRev: rev, RemoteMtime: localMtime, RemoteSize: localInfo.Size(), LocalMtime: localMtime, LastSync: now})
		return "uploaded", nil

	case !localChanged && remoteChanged:
		if err := e.pull(localPath, true); err != nil {
			return "", err
		}
		if err := reload(); err != nil {
			return "", err
		}
		after, statErr := os.Stat(localPath)
		if statErr != nil {
			return "", statErr
		}
		_ = saveState(syncState{RemoteRev: remote.Rev, RemoteMtime: remote.ModTime, RemoteSize: remote.Size, LocalMtime: after.ModTime().UnixMilli(), LastSync: now})
		return "downloaded", nil

	default:
		// both changed: keep newer by mtime, stash loser as conflict copy.
		// Stashes are hard failures — proceeding would silently lose data.
		// The local loser is stashed via the consistent snapshot (never a
		// plain copy of the live DB).
		if remote.ModTime > localMtime {
			conflictPath := conflictName(localPath)
			if err := e.stashLocal(localPath, conflictPath); err != nil {
				return "", fmt.Errorf("conflict stash: %w", err)
			}
			if err := e.pull(localPath, true); err != nil {
				return "", err
			}
			if err := reload(); err != nil {
				return "", err
			}
			after, statErr := os.Stat(localPath)
			if statErr != nil {
				return "", statErr
			}
			_ = saveState(syncState{RemoteRev: remote.Rev, RemoteMtime: remote.ModTime, RemoteSize: remote.Size, LocalMtime: after.ModTime().UnixMilli(), LastSync: now})
			return "conflict: kept remote", nil
		}
		conflictPath := conflictName(localPath)
		// stash remote loser next to the local file for visibility.
		// swap=false: preSwap (vault close) must NOT run for a stash.
		if err := e.pull(conflictPath, false); err != nil {
			return "", fmt.Errorf("conflict stash: %w", err)
		}
		rev, err := e.push(localPath)
		if err != nil {
			return "", err
		}
		_ = saveState(syncState{RemoteRev: rev, RemoteMtime: localMtime, RemoteSize: localInfo.Size(), LocalMtime: localMtime, LastSync: now})
		return "conflict: kept local", nil
	}
}

func (e *SyncEngine) push(localPath string) (string, error) {
	tmp := ""
	if e.snapshot != nil {
		t, err := e.snapshot()
		if err != nil {
			return "", err
		}
		tmp = t
	} else {
		t, err := snapshotVaultFile(localPath)
		if err != nil {
			return "", err
		}
		tmp = t
	}
	defer os.Remove(tmp)
	return e.provider.Upload(tmp, e.remote)
}

// pull downloads the remote to a temp file and swaps it over dest.
// swap=true runs preSwap first (closes the vault so Windows allows the
// replace); swap=false is used for conflict stashes, where the live vault
// must stay untouched.
func (e *SyncEngine) pull(dest string, swap bool) error {
	tmp := dest + ".synctmp"
	defer os.Remove(tmp)
	if err := e.provider.Download(e.remote, tmp); err != nil {
		return err
	}
	// verify it looks like a SQLite vault before swapping
	if err := verifyVaultFile(tmp); err != nil {
		return err
	}
	if swap && e.preSwap != nil {
		if err := e.preSwap(); err != nil {
			return err
		}
	}
	return atomicReplace(tmp, dest)
}

// stashLocal writes a consistent copy of the live local DB to dest (used for
// conflict losers — a plain copy could catch a mid-write SQLite state).
func (e *SyncEngine) stashLocal(localPath, dest string) error {
	if e.snapshot != nil {
		tmp, err := e.snapshot()
		if err != nil {
			return err
		}
		defer os.Remove(tmp)
		return copyFile(tmp, dest)
	}
	return copyFile(localPath, dest)
}

// remoteMatches downloads the remote file and compares content hashes.
// Used when the sync state is fresh/absent so an unchanged pair does not
// degrade into a false conflict.
func (e *SyncEngine) remoteMatches(localPath string) (bool, error) {
	tmp := localPath + ".synccmp"
	defer os.Remove(tmp)
	if err := e.provider.Download(e.remote, tmp); err != nil {
		return false, err
	}
	if err := verifyVaultFile(tmp); err != nil {
		return false, err
	}
	localHash, err := fileHash(localPath)
	if err != nil {
		return false, err
	}
	remoteHash, err := fileHash(tmp)
	if err != nil {
		return false, err
	}
	return localHash == remoteHash, nil
}

// snapshotVaultFile copies the live DB consistently. For SQLite files opened
// by our own process, a plain copy can catch a mid-write state; callers that
// hold the vault should prefer VACUUM INTO via backupTo.
func snapshotVaultFile(localPath string) (string, error) {
	srcInfo, err := os.Stat(localPath)
	if err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp("", "ciphersync-sync-*.passapp")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	if err := copyFile(localPath, tmpPath); err != nil {
		os.Remove(tmpPath)
		return "", err
	}
	// preserve mtime so change detection stays consistent across the snapshot
	_ = os.Chtimes(tmpPath, srcInfo.ModTime(), srcInfo.ModTime())
	return tmpPath, nil
}

func verifyVaultFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	header := make([]byte, 16)
	if _, err := io.ReadFull(f, header); err != nil {
		return errors.New("invalid vault file")
	}
	if string(header) != "SQLite format 3\x00" {
		return errors.New("invalid vault file")
	}
	return nil
}

func atomicReplace(tmp, dest string) error {
	// Windows cannot rename over an open file; caller must have closed it.
	if err := os.Rename(tmp, dest); err == nil {
		return nil
	}
	// fallback: copy over + remove tmp
	if err := copyFile(tmp, dest); err != nil {
		return err
	}
	return os.Remove(tmp)
}

func conflictName(localPath string) string {
	ext := filepath.Ext(localPath)
	base := strings.TrimSuffix(localPath, ext)
	stamp := time.Now().Format("20060102-150405.000") // ms precision avoids same-second collisions
	host, _ := os.Hostname()
	if host == "" {
		host = "device"
	}
	// sanitize hostname for filenames
	host = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return -1
	}, host)
	candidate := fmt.Sprintf("%s (conflict %s, %s)%s", base, stamp, host, ext)
	for i := 2; fileExists(candidate); i++ {
		candidate = fmt.Sprintf("%s (conflict %s, %s, %d)%s", base, stamp, host, i, ext)
	}
	return candidate
}
