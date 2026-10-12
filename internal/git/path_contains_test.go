package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathContains(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "sub", "dir")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	// Shares root's path as a string prefix without being inside it.
	sibling := root + "-sibling"
	if err := os.Mkdir(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sibling) })
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		root string
		dir  string
		want bool
	}{
		{"root itself", root, root, true},
		{"nested directory", root, nested, true},
		{"parent directory", root, filepath.Dir(root), false},
		{"sibling sharing a prefix", root, sibling, false},
		{"reached through a symlink", root, filepath.Join(link, "sub"), true},
		{"empty dir", root, "", false},
		{"empty root", "", root, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PathContains(tt.root, tt.dir); got != tt.want {
				t.Errorf("PathContains(%q, %q) = %v, want %v", tt.root, tt.dir, got, tt.want)
			}
		})
	}
}
