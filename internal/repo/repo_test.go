package repo

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

func TestDetectContext(t *testing.T) {
	tests := []struct {
		name      string
		responses map[string]mock.Response
		setupDir  func(t *testing.T, dir string)
		want      RepoContext
	}{
		{
			name: "bare repo detected via git",
			responses: map[string]mock.Response{
				"{dir}:[rev-parse --is-bare-repository]": {Output: "true"},
			},
			want: ContextBareRepo,
		},
		{
			name: "worktree inside bare repo with .bare directory",
			responses: map[string]mock.Response{
				"{dir}:[rev-parse --is-bare-repository]": {Output: "false"},
				"{dir}:[rev-parse --git-dir]":            {Output: "/repo/.bare"},
				"{dir}:[rev-parse --git-common-dir]":     {Output: "/repo/.bare"},
			},
			want: ContextBareRepo,
		},
		{
			name: "non-bare regular repo",
			responses: map[string]mock.Response{
				"{dir}:[rev-parse --is-bare-repository]": {Output: "false"},
				"{dir}:[rev-parse --git-dir]":            {Output: ".git"},
				"{dir}:[rev-parse --git-common-dir]":     {Output: ".git"},
			},
			want: ContextNonBareRepo,
		},
		{
			name: "no repo at all",
			responses: map[string]mock.Response{
				"{dir}:[rev-parse --is-bare-repository]": {Output: "", Err: fmt.Errorf("not a git repository")},
				"{dir}:[rev-parse --git-dir]":            {Output: "", Err: fmt.Errorf("not a git repository")},
			},
			want: ContextNoRepo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			// Replace {dir} placeholders in response keys
			resolved := make(map[string]mock.Response)
			for k, v := range tt.responses {
				resolvedKey := strings.ReplaceAll(k, "{dir}", dir)
				resolvedVal := mock.Response{
					Output: strings.ReplaceAll(v.Output, "{dir}", dir),
					Err:    v.Err,
				}
				resolved[resolvedKey] = resolvedVal
			}

			if tt.setupDir != nil {
				tt.setupDir(t, dir)
			}

			runner := &mock.Runner{Responses: resolved}
			got := DetectContext(runner, dir)
			if got != tt.want {
				t.Errorf("DetectContext() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveBareRoot(t *testing.T) {
	tests := []struct {
		name      string
		responses map[string]mock.Response
		wantExact string // if set, expect this exact path
		wantSelf  bool   // if true, expect the input dir back
	}{
		{
			name: "plain bare repo returns itself (git-common-dir is dot)",
			responses: map[string]mock.Response{
				"{dir}:[rev-parse --git-common-dir]": {Output: "."},
			},
			wantSelf: true,
		},
		{
			name: "from worktree resolves to bare root via absolute .bare path",
			responses: map[string]mock.Response{
				"{dir}:[rev-parse --git-common-dir]": {Output: "/code/myproject/.bare"},
			},
			wantExact: "/code/myproject",
		},
		{
			name: "git command fails returns original path",
			responses: map[string]mock.Response{
				"{dir}:[rev-parse --git-common-dir]": {Output: "", Err: fmt.Errorf("not a git repo")},
			},
			wantSelf: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			resolved := make(map[string]mock.Response)
			for k, v := range tt.responses {
				resolved[strings.ReplaceAll(k, "{dir}", dir)] = mock.Response{
					Output: strings.ReplaceAll(v.Output, "{dir}", dir),
					Err:    v.Err,
				}
			}

			runner := &mock.Runner{Responses: resolved}
			got := ResolveBareRoot(runner, dir)

			if tt.wantExact != "" && got != tt.wantExact {
				t.Errorf("ResolveBareRoot() = %q, want %q", got, tt.wantExact)
			}
			if tt.wantSelf && got != dir {
				t.Errorf("ResolveBareRoot() = %q, want %q", got, dir)
			}
		})
	}
}
