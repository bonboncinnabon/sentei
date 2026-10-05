package repo

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/bonboncinnabon/sentei/internal/fileutil"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
)

type MigrateOptions struct {
	RepoPath string
}

type MigrateResult struct {
	BareRoot     string
	WorktreePath string
	BackupPath   string
	BackupSize   string
	Branch       string
	IsDirty      bool
	Phases       []progress.Phase
	Err          error
}

func Migrate(runner git.CommandRunner, shell git.ShellRunner, opts MigrateOptions, emit func(progress.Event)) MigrateResult {
	return prepareMigrate(runner, shell, opts).run(emit)
}

// copyTree recursively copies src to dst. It recreates symlinks rather than
// following them, and replaces an existing dst rather than writing through it,
// so restoring over a checked-out symlink cannot corrupt the link's target.
func copyTree(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}

	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		_ = os.Remove(dst)
		return os.Symlink(target, dst)

	case info.IsDir():
		if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := copyTree(filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name())); err != nil {
				return err
			}
		}
		return nil

	default:
		_ = os.Remove(dst)
		return fileutil.CopyFile(src, dst)
	}
}

func calculateDirSize(path string) string {
	var totalSize int64
	_ = filepath.WalkDir(path, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !entry.IsDir() {
			info, err := entry.Info()
			if err == nil {
				totalSize += info.Size()
			}
		}
		return nil
	})
	return formatSize(totalSize)
}

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.0f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.0f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func DeleteBackup(backupPath string) error {
	return os.RemoveAll(backupPath)
}

func (r MigrateResult) RestoreCommand() string {
	return fmt.Sprintf("rm -rf %q && mv %q %q", r.BareRoot, r.BackupPath, r.BareRoot)
}
