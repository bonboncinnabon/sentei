package git

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

func TestWorktreeDirName(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		want   string
	}{
		{name: "slash replaced with dash", branch: "feature/auth", want: "feature-auth"},
		{name: "multiple slashes", branch: "bugfix/login/redirect", want: "bugfix-login-redirect"},
		{name: "no slash unchanged", branch: "hotfix", want: "hotfix"},
		{name: "trailing slash stripped", branch: "feature/", want: "feature-"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WorktreeDirName(tt.branch)
			if got != tt.want {
				t.Errorf("WorktreeDirName(%q) = %q, want %q", tt.branch, got, tt.want)
			}
		})
	}
}

func TestWorktreePath(t *testing.T) {
	got := WorktreePath("/repo", "feature/auth")
	want := filepath.Join("/repo", "feature-auth")
	if got != want {
		t.Errorf("WorktreePath = %q, want %q", got, want)
	}
}

func TestCommonDir(t *testing.T) {
	tests := []struct {
		name      string
		repoPath  string
		commonDir string
		runErr    error
		want      string
		wantErr   bool
	}{
		{
			name:      "absolute sentei layout",
			repoPath:  "/repo/main",
			commonDir: "/repo/.bare",
			want:      "/repo/.bare",
		},
		{
			name:      "absolute non-sentei bare name",
			repoPath:  "/play/wt-feature",
			commonDir: "/play/repo.git",
			want:      "/play/repo.git",
		},
		{
			name:      "relative common dir joined with repo path",
			repoPath:  "/repo/main",
			commonDir: "../.bare",
			want:      "/repo/.bare",
		},
		{
			name:     "runner error propagates",
			repoPath: "/repo/main",
			runErr:   errors.New("not a git repository"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mock.Runner{
				Responses: map[string]mock.Response{
					tt.repoPath + ":[rev-parse --git-common-dir]": {Output: tt.commonDir, Err: tt.runErr},
				},
			}
			got, err := CommonDir(runner, tt.repoPath)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("CommonDir(%q) error = nil, want error", tt.repoPath)
				}
				return
			}
			if err != nil {
				t.Fatalf("CommonDir(%q) unexpected error: %v", tt.repoPath, err)
			}
			if got != tt.want {
				t.Errorf("CommonDir(%q) = %q, want %q", tt.repoPath, got, tt.want)
			}
		})
	}
}
