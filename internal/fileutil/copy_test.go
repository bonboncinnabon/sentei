package fileutil

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src"), filepath.Join(dir, "dst")
	// Larger than one io.Copy buffer, so the copy takes several reads.
	content := bytes.Repeat([]byte("sentei\n"), 20_000)
	if err := os.WriteFile(src, content, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := CopyFile(src, dst); err != nil {
		t.Fatalf("CopyFile() error: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("copied %d bytes, want the %d source bytes unchanged", len(got), len(content))
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o750 {
		t.Errorf("mode = %v, want the source mode 0750", info.Mode().Perm())
	}
}

func TestCopyFile_ReplacesLongerDestination(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src"), filepath.Join(dir, "dst")
	if err := os.WriteFile(src, []byte("short"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("a much longer previous content"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := CopyFile(src, dst); err != nil {
		t.Fatalf("CopyFile() error: %v", err)
	}
	if got, _ := os.ReadFile(dst); string(got) != "short" {
		t.Errorf("destination = %q, want %q", got, "short")
	}
}

func TestCopyFile_DirectorySourceLeavesDestinationAlone(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "dst")
	if err := os.WriteFile(dst, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := CopyFile(t.TempDir(), dst); err == nil {
		t.Fatal("expected an error when the source is a directory")
	}
	if got, _ := os.ReadFile(dst); string(got) != "keep" {
		t.Errorf("destination = %q after a failed copy, want it untouched", got)
	}
}

func TestCopyFile_OntoItselfKeepsContent(t *testing.T) {
	dir := t.TempDir()
	src, link := filepath.Join(dir, ".env"), filepath.Join(dir, "link")
	if err := os.WriteFile(src, []byte("SECRET=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(src, link); err != nil {
		t.Fatal(err)
	}

	for _, dst := range []string{src, link} {
		if err := CopyFile(src, dst); err != nil {
			t.Fatalf("CopyFile(%s, %s) error: %v", src, dst, err)
		}
		if got, _ := os.ReadFile(src); string(got) != "SECRET=1\n" {
			t.Fatalf("source = %q after copying onto %s, want it intact", got, dst)
		}
	}
}
