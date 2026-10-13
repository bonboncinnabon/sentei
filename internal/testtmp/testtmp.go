// Package testtmp provides temp-dir helpers for tests. It is a dependency-free
// leaf package so any package's tests can use it without import cycles.
//
// On macOS, Spotlight (mds/mdworker) indexes files under the default TMPDIR
// (/var/folders/...). When a test writes git object files and then removes the
// dir, the indexer can still be holding a file, so RemoveAll fails with
// ENOTEMPTY and the test flakes. These helpers address that two ways: a
// .metadata_never_index marker (the documented Spotlight exclusion) and a
// bounded retry on RemoveAll.
package testtmp

import (
	"os"
	"path/filepath"
	"testing"
)

// HermeticGitEnv returns the process environment hardened so a git child
// process cannot read or write ANY configuration outside the repository it
// is pointed at: global and system config are voided, and identity comes
// from the environment. Tests must never depend on, or be able to mutate,
// the developer's real git identity or config; a path bug in a test then
// fails loudly instead of silently poisoning the developer's repositories.
// The .invalid TLD (RFC 2606) can never be claimed by a forge account, so a
// leaked identity can never be attributed to a stranger.
func HermeticGitEnv() []string {
	return append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=sentei-test",
		"GIT_AUTHOR_EMAIL=test@sentei.invalid",
		"GIT_COMMITTER_NAME=sentei-test",
		"GIT_COMMITTER_EMAIL=test@sentei.invalid",
	)
}

// markNoIndex excludes dir's subtree from macOS Spotlight indexing.
func markNoIndex(dir string) {
	_ = os.WriteFile(filepath.Join(dir, ".metadata_never_index"), nil, 0o644)
}

// removeWithRetry removes dir, retrying to absorb the transient ENOTEMPTY race.
// Bounded and condition-based (retry until success), with no sleep.
func removeWithRetry(dir string) {
	for i := 0; i < 50; i++ {
		if err := os.RemoveAll(dir); err == nil {
			return
		}
	}
}

// RobustTempDir is like t.TempDir but excludes its subtree from Spotlight and
// retries cleanup, so a macOS indexing race can't fail an otherwise-passing
// test. Cleanup is best-effort and never fails the test.
func RobustTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "sentei-test-")
	if err != nil {
		t.Fatal(err)
	}
	markNoIndex(dir)
	t.Cleanup(func() { removeWithRetry(dir) })
	return dir
}

// RunWithIsolatedTemp runs a package's tests with TMPDIR pointed at a dedicated
// Spotlight-excluded dir, so every t.TempDir in the package is safe from the
// macOS indexing race without changing individual tests. Use from TestMain:
//
//	func TestMain(m *testing.M) { os.Exit(testtmp.RunWithIsolatedTemp(m)) }
func RunWithIsolatedTemp(m *testing.M) int {
	base, err := os.MkdirTemp("", "sentei-tests-")
	if err != nil {
		panic(err)
	}
	markNoIndex(base)
	old, hadOld := os.LookupEnv("TMPDIR")
	_ = os.Setenv("TMPDIR", base)

	// Make test git hermetic with a controlled global config: it drops the
	// developer's real one (esp. a custom core.hooksPath whose hooks write into
	// .git and race t.TempDir cleanup) while keeping what tests rely on — the
	// default branch name and a commit identity. core.hooksPath points at an
	// empty dir so no hooks run.
	noHooks := filepath.Join(base, "no-hooks")
	_ = os.MkdirAll(noHooks, 0o755)
	gitconfig := filepath.Join(base, "gitconfig")
	_ = os.WriteFile(gitconfig, []byte(
		"[init]\n\tdefaultBranch = main\n"+
			"[user]\n\tname = sentei-test\n\temail = test@example.com\n"+
			"[core]\n\thooksPath = "+noHooks+"\n"+
			// No background maintenance: git starts it detached after a commit
			// and it outlives the command, writing into a repository a test is
			// comparing byte for byte or already deleting.
			"[maintenance]\n\tauto = false\n[gc]\n\tauto = 0\n"), 0o644)
	_ = os.Setenv("GIT_CONFIG_GLOBAL", gitconfig)
	_ = os.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	code := m.Run()

	if hadOld {
		_ = os.Setenv("TMPDIR", old)
	} else {
		_ = os.Unsetenv("TMPDIR")
	}
	removeWithRetry(base)
	return code
}
