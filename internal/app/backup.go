package app

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Backups are folders named designhub-YYYYMMDD-HHMMSS in the backups volume:
//
//	database.dump   PostgreSQL custom-format dump (pg_restore)
//	repository.tar.gz  the engineering git repository
//	manifest.json   version, time and contents
//
// Attachment files are content addressed and never change, so they are
// copied once into backups/files and shared by all backups.

type backupInfo struct {
	Name    string    `json:"name"`
	Time    time.Time `json:"time"`
	Size    int64     `json:"size"`
	Version string    `json:"version"`
	Error   string    `json:"error,omitempty"`
}

var backupMu sync.Mutex

func (a *App) listBackups() ([]backupInfo, error) {
	entries, err := os.ReadDir(a.Cfg.BackupsDir)
	if err != nil {
		return []backupInfo{}, nil
	}
	out := []backupInfo{}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "designhub-") {
			continue
		}
		bi := backupInfo{Name: e.Name()}
		b, err := os.ReadFile(filepath.Join(a.Cfg.BackupsDir, e.Name(), "manifest.json"))
		if err != nil {
			bi.Error = "incomplete"
		} else {
			json.Unmarshal(b, &bi)
			bi.Name = e.Name()
		}
		filepath.WalkDir(filepath.Join(a.Cfg.BackupsDir, e.Name()), func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				if fi, err := d.Info(); err == nil {
					bi.Size += fi.Size()
				}
			}
			return nil
		})
		out = append(out, bi)
	}
	return out, nil
}

// Backup writes a complete backup and removes old ones beyond the keep count.
func (a *App) Backup(ctx context.Context) (string, error) {
	backupMu.Lock()
	defer backupMu.Unlock()
	name := "designhub-" + time.Now().UTC().Format("20060102-150405")
	dir := filepath.Join(a.Cfg.BackupsDir, name)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	fail := func(err error) (string, error) {
		os.WriteFile(filepath.Join(dir, "error.txt"), []byte(err.Error()), 0o640)
		return "", err
	}
	if err := a.dumpDatabase(ctx, filepath.Join(dir, "database.dump")); err != nil {
		return fail(fmt.Errorf("database: %w", err))
	}
	if err := a.Repo.Repack(); err != nil {
		log.Printf("repack: %v", err)
	}
	if err := tarDir(a.Repo.Dir(), filepath.Join(dir, "repository.tar.gz")); err != nil {
		return fail(fmt.Errorf("repository: %w", err))
	}
	n, err := syncFiles(a.Files.Dir(), filepath.Join(a.Cfg.BackupsDir, "files"))
	if err != nil {
		return fail(fmt.Errorf("files: %w", err))
	}
	head, _ := a.Repo.Head("main")
	m, _ := json.MarshalIndent(map[string]any{"name": name, "time": time.Now().UTC(), "version": a.Cfg.Version, "repositoryHead": head, "newFiles": n}, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), m, 0o640); err != nil {
		return fail(err)
	}
	var sched backupSchedule
	sched.Keep = 14
	a.Setting("backup_schedule", &sched)
	a.pruneBackups(sched.Keep)
	return name, nil
}

func (a *App) pruneBackups(keep int) {
	bs, _ := a.listBackups()
	sort.Slice(bs, func(i, j int) bool { return bs[i].Name > bs[j].Name })
	for i, b := range bs {
		if i >= keep {
			os.RemoveAll(filepath.Join(a.Cfg.BackupsDir, b.Name))
		}
	}
}

// pgEnv returns environment variables for pg_dump and pg_restore.
func (a *App) pgEnv() []string {
	env := os.Environ()
	if a.Cfg.DatabaseURL != "" {
		return env
	}
	return append(env, "PGHOST="+a.Cfg.DBHost, "PGPORT="+a.Cfg.DBPort, "PGDATABASE="+a.Cfg.DBName, "PGUSER="+a.Cfg.DBUser, "PGPASSWORD="+a.Cfg.DBPassword)
}

func (a *App) dumpDatabase(ctx context.Context, out string) error {
	args := []string{"--format=custom", "--no-owner", "--file", out}
	if a.Cfg.DatabaseURL != "" {
		args = append(args, "--dbname", a.Cfg.DatabaseURL)
	}
	cmd := exec.CommandContext(ctx, "pg_dump", args...)
	cmd.Env = a.pgEnv()
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(b)))
	}
	return nil
}

// RestoreDatabase replaces the database contents from a backup.
func (a *App) RestoreDatabase(ctx context.Context, dump string) error {
	args := []string{"--clean", "--if-exists", "--no-owner", "--single-transaction"}
	if a.Cfg.DatabaseURL != "" {
		args = append(args, "--dbname", a.Cfg.DatabaseURL)
	} else {
		args = append(args, "--dbname", a.Cfg.DBName)
	}
	args = append(args, dump)
	cmd := exec.CommandContext(ctx, "pg_restore", args...)
	cmd.Env = a.pgEnv()
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(b)))
	}
	return nil
}

func tarDir(src, out string) error {
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	err = filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil || rel == "." {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() && !info.IsDir() {
			return nil
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			in, err := os.Open(p)
			if err != nil {
				return err
			}
			_, err = io.Copy(tw, in)
			in.Close()
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

// UntarDir extracts a backup archive into dst, which must be empty.
func UntarDir(archive, dst string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		p := filepath.Join(dst, filepath.FromSlash(hdr.Name))
		if !strings.HasPrefix(p, filepath.Clean(dst)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid path in archive: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(p, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
}

// syncFiles copies files missing from dst. It returns how many it copied.
func syncFiles(src, dst string) (int, error) {
	n := 0
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			if rel == "tmp" {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		if _, err := os.Stat(target); err == nil {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		tmp := target + ".tmp"
		out, err := os.Create(tmp)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
		n++
		return os.Rename(tmp, target)
	})
	return n, err
}

// SyncFilesBack copies backed-up attachment files into the file store.
func SyncFilesBack(backups, files string) (int, error) {
	return syncFiles(filepath.Join(backups, "files"), files)
}

func (a *App) apiBackupNow(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if !u.Can(PermSystem) {
		return errForbidden
	}
	name, err := a.Backup(r.Context())
	a.Audit(r.Context(), u, clientIP(r), "backup", "", map[string]any{"name": name, "error": errString(err)})
	if err != nil {
		return errf(http.StatusInternalServerError, "Backup failed: %v", err)
	}
	return ok(w, map[string]string{"name": name})
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
