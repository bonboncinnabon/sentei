package fileutil

import (
	"fmt"
	"io"
	"os"
)

// CopyFile copies src to dst, preserving the source file's permissions. The
// content is streamed, so memory use does not grow with the file.
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	info, err := in.Stat()
	if err != nil {
		return err
	}

	// Both checks come before dst is opened, because opening truncates it: a
	// source that cannot be read must leave dst alone, and a file copied onto
	// itself (same path, or a hard link) must keep its content.
	if info.IsDir() {
		return fmt.Errorf("copy %s: is a directory", src)
	}
	if existing, err := os.Stat(dst); err == nil && os.SameFile(info, existing) {
		return nil
	}

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
