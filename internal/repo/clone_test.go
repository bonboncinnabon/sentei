package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

func TestDeriveRepoName(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{url: "git@github.com:user/repo.git", want: "repo"},
		{url: "https://github.com/user/repo.git", want: "repo"},
		{url: "https://github.com/user/repo", want: "repo"},
		{url: "git@gitlab.com:org/sub/project.git", want: "project"},
		{url: "https://github.com/user/my-repo.git", want: "my-repo"},
		{url: "https://github.com/user/repo/", want: "repo"},         // trailing slash
		{url: "https://github.com/user/repo.git/", want: "repo"},     // trailing slash + .git
		{url: "https://github.com/user/repo?ref=main", want: "repo"}, // query string
		{url: "git@github.com:user/repo.git#frag", want: "repo"},     // fragment
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := DeriveRepoName(tt.url)
			if got != tt.want {
				t.Errorf("DeriveRepoName(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestClone_Successful(t *testing.T) {
	dir := t.TempDir()
	repoName := "repo"
	repoPath := filepath.Join(dir, repoName)
	barePath := filepath.Join(repoPath, ".bare")

	runner := &mock.Runner{Responses: map[string]mock.Response{
		// Clone phase
		fmt.Sprintf("%s:[clone --bare --single-branch -- git@github.com:user/repo.git %s]", dir, barePath): {Output: ""},
		// Structure phase
		fmt.Sprintf("%s:[config remote.origin.fetch +refs/heads/*:refs/remotes/origin/*]", barePath): {Output: ""},
		fmt.Sprintf("%s:[fetch origin]", barePath):                                                   {Output: ""},
		// Worktree phase
		fmt.Sprintf("%s:[symbolic-ref --short HEAD]", barePath):                 {Output: "main"},
		fmt.Sprintf("%s:[show-ref --verify refs/heads/main]", barePath):         {Output: "abc123"},
		fmt.Sprintf("%s:[worktree add %s/main main]", repoPath, repoPath):       {Output: ""},
		fmt.Sprintf("%s/main:[branch --set-upstream-to=origin/main]", repoPath): {Output: ""},
	}}

	ec := &mock.EventCollector[progress.Event]{}
	opts := CloneOptions{
		URL:      "git@github.com:user/repo.git",
		Location: dir,
		Name:     repoName,
	}
	result := Clone(runner, opts, ec.Emit)

	if result.RepoPath != repoPath {
		t.Errorf("RepoPath = %q, want %q", result.RepoPath, repoPath)
	}
	if result.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want %q", result.DefaultBranch, "main")
	}
	for _, phase := range result.Phases {
		for _, step := range phase.Steps {
			if step.Status == progress.StepFailed {
				t.Errorf("step %q failed: %v", step.Name, step.Error)
			}
		}
	}
}

func TestClone_DefaultBranchFallback(t *testing.T) {
	dir := t.TempDir()
	repoName := "repo"
	repoPath := filepath.Join(dir, repoName)
	barePath := filepath.Join(repoPath, ".bare")

	runner := &mock.Runner{Responses: map[string]mock.Response{
		fmt.Sprintf("%s:[clone --bare --single-branch -- git@github.com:user/repo.git %s]", dir, barePath): {Output: ""},
		fmt.Sprintf("%s:[config remote.origin.fetch +refs/heads/*:refs/remotes/origin/*]", barePath):       {Output: ""},
		fmt.Sprintf("%s:[fetch origin]", barePath):                                                         {Output: ""},
		// symbolic-ref fails — fallback to main
		fmt.Sprintf("%s:[symbolic-ref --short HEAD]", barePath):                 {Output: "", Err: fmt.Errorf("not found")},
		fmt.Sprintf("%s:[show-ref --verify refs/heads/main]", barePath):         {Output: "abc123"},
		fmt.Sprintf("%s:[worktree add %s/main main]", repoPath, repoPath):       {Output: ""},
		fmt.Sprintf("%s/main:[branch --set-upstream-to=origin/main]", repoPath): {Output: ""},
	}}

	ec := &mock.EventCollector[progress.Event]{}
	opts := CloneOptions{URL: "git@github.com:user/repo.git", Location: dir, Name: repoName}
	result := Clone(runner, opts, ec.Emit)

	if result.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want %q (fallback)", result.DefaultBranch, "main")
	}
}

func TestClone_NonStandardDefaultBranch(t *testing.T) {
	dir := t.TempDir()
	repoName := "repo"
	repoPath := filepath.Join(dir, repoName)
	barePath := filepath.Join(repoPath, ".bare")

	runner := &mock.Runner{Responses: map[string]mock.Response{
		fmt.Sprintf("%s:[clone --bare --single-branch -- git@github.com:user/repo.git %s]", dir, barePath): {Output: ""},
		fmt.Sprintf("%s:[config remote.origin.fetch +refs/heads/*:refs/remotes/origin/*]", barePath):       {Output: ""},
		fmt.Sprintf("%s:[fetch origin]", barePath):                                                         {Output: ""},
		// Default branch is neither main nor master; a bare clone records it in HEAD.
		fmt.Sprintf("%s:[symbolic-ref --short HEAD]", barePath):                             {Output: "production"},
		fmt.Sprintf("%s:[show-ref --verify refs/heads/production]", barePath):               {Output: "abc123"},
		fmt.Sprintf("%s:[worktree add %s/production production]", repoPath, repoPath):       {Output: ""},
		fmt.Sprintf("%s/production:[branch --set-upstream-to=origin/production]", repoPath): {Output: ""},
	}}

	ec := &mock.EventCollector[progress.Event]{}
	opts := CloneOptions{URL: "git@github.com:user/repo.git", Location: dir, Name: repoName}
	result := Clone(runner, opts, ec.Emit)

	if result.DefaultBranch != "production" {
		t.Errorf("DefaultBranch = %q, want %q", result.DefaultBranch, "production")
	}
	for _, phase := range result.Phases {
		for _, step := range phase.Steps {
			if step.Status == progress.StepFailed {
				t.Errorf("step %q failed: %v", step.Name, step.Error)
			}
		}
	}
}

func TestClone_NetworkError(t *testing.T) {
	dir := t.TempDir()
	barePath := filepath.Join(dir, "repo", ".bare")

	runner := &mock.Runner{Responses: map[string]mock.Response{
		fmt.Sprintf("%s:[clone --bare --single-branch -- git@github.com:user/repo.git %s]", dir, barePath): {
			Output: "", Err: fmt.Errorf("fatal: Could not read from remote repository"),
		},
	}}

	ec := &mock.EventCollector[progress.Event]{}
	opts := CloneOptions{URL: "git@github.com:user/repo.git", Location: dir, Name: "repo"}
	result := Clone(runner, opts, ec.Emit)

	clonePhase := findPhase(result.Phases, "Clone")
	if clonePhase == nil || !clonePhase.HasFailures() {
		t.Error("expected clone phase to fail on network error")
	}
}

func TestClone_FetchFailure_StillSucceedsWithoutTracking(t *testing.T) {
	dir := t.TempDir()
	repoPath := filepath.Join(dir, "repo")
	barePath := filepath.Join(repoPath, ".bare")

	// Everything succeeds except the tracking fetch (simulating a network blip
	// after the bare clone already succeeded). The worktree must still be created.
	runner := &mock.Runner{Responses: map[string]mock.Response{
		fmt.Sprintf("%s:[clone --bare --single-branch -- git@h:u/repo.git %s]", dir, barePath):       {Output: ""},
		fmt.Sprintf("%s:[config remote.origin.fetch +refs/heads/*:refs/remotes/origin/*]", barePath): {Output: ""},
		fmt.Sprintf("%s:[symbolic-ref --short HEAD]", barePath):                                      {Output: "main"},
		fmt.Sprintf("%s:[show-ref --verify refs/heads/main]", barePath):                              {Output: "abc"},
		fmt.Sprintf("%s:[worktree add %s/main main]", repoPath, repoPath):                            {Output: ""},
		fmt.Sprintf("%s:[fetch origin]", barePath):                                                   {Output: "", Err: fmt.Errorf("could not read from remote")},
	}}

	ec := &mock.EventCollector[progress.Event]{}
	result := Clone(runner, CloneOptions{URL: "git@h:u/repo.git", Location: dir, Name: "repo"}, ec.Emit)

	if result.HasFailures() {
		t.Error("a tracking fetch failure must not fail the clone; the worktree is usable")
	}
	if result.WorktreePath != filepath.Join(repoPath, "main") {
		t.Errorf("worktree should still be created, got WorktreePath=%q", result.WorktreePath)
	}
}

func TestClone_EmptyName_RejectedBeforeAnyGitCall(t *testing.T) {
	dir := t.TempDir()
	// Empty Responses: any git call would return an error, proving none happens.
	runner := &mock.Runner{Responses: map[string]mock.Response{}}

	ec := &mock.EventCollector[progress.Event]{}
	opts := CloneOptions{URL: "https://host/user/repo/", Location: dir, Name: ""}
	result := Clone(runner, opts, ec.Emit)

	validate := findPhase(result.Phases, "Validate")
	if validate == nil || !validate.HasFailures() {
		t.Error("empty name must fail validation")
	}
	if result.DefaultBranch != "" {
		t.Error("no clone should have happened")
	}
}

func TestClone_PathLikeName_Rejected(t *testing.T) {
	dir := t.TempDir()
	runner := &mock.Runner{Responses: map[string]mock.Response{}}
	ec := &mock.EventCollector[progress.Event]{}

	for _, name := range []string{"/abs/target", "../escaped", "nested/name", ".."} {
		result := Clone(runner, CloneOptions{URL: "u", Location: dir, Name: name}, ec.Emit)
		validate := findPhase(result.Phases, "Validate")
		if validate == nil || !validate.HasFailures() {
			t.Errorf("name %q should be rejected by validation, got %+v", name, result.Phases)
		}
	}
}

func TestClone_ExistingTarget_RejectedAndPreserved(t *testing.T) {
	dir := t.TempDir()
	repoPath := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(repoPath, "important.txt")
	if err := os.WriteFile(sentinel, []byte("user data"), 0o644); err != nil {
		t.Fatal(err)
	}

	runner := &mock.Runner{Responses: map[string]mock.Response{}}
	ec := &mock.EventCollector[progress.Event]{}
	result := Clone(runner, CloneOptions{URL: "u", Location: dir, Name: "repo"}, ec.Emit)

	validate := findPhase(result.Phases, "Validate")
	if validate == nil || !validate.HasFailures() {
		t.Fatalf("existing target must be rejected, got %+v", result.Phases)
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Error("pre-existing user data must be left untouched")
	}
}

func TestClone_WorktreeFailure_RollsBackPartialDir(t *testing.T) {
	dir := t.TempDir()
	repoPath := filepath.Join(dir, "repo")
	barePath := filepath.Join(repoPath, ".bare")

	// All phases succeed until worktree add fails (empty remote: show-ref fails).
	runner := &mock.Runner{
		Responses: map[string]mock.Response{
			fmt.Sprintf("%s:[clone --bare --single-branch -- u %s]", dir, barePath):                      {Output: ""},
			fmt.Sprintf("%s:[config remote.origin.fetch +refs/heads/*:refs/remotes/origin/*]", barePath): {Output: ""},
			fmt.Sprintf("%s:[fetch origin]", barePath):                                                   {Output: ""},
			fmt.Sprintf("%s:[symbolic-ref --short HEAD]", barePath):                                      {Output: "main"},
			fmt.Sprintf("%s:[show-ref --verify refs/heads/main]", barePath):                              {Output: "", Err: fmt.Errorf("not found")},
		},
		// Simulate clone --bare creating the bare dir, so rollback has something
		// to remove.
		OnRun: func(d string, args []string) {
			if len(args) > 0 && args[0] == "clone" {
				_ = os.MkdirAll(barePath, 0o755)
			}
		},
	}

	ec := &mock.EventCollector[progress.Event]{}
	result := Clone(runner, CloneOptions{URL: "u", Location: dir, Name: "repo"}, ec.Emit)

	if !result.HasFailures() {
		t.Fatal("expected the clone to fail on an empty remote")
	}
	if result.WorktreePath != "" {
		t.Error("no worktree was created; WorktreePath must be empty")
	}
	if _, err := os.Stat(repoPath); !os.IsNotExist(err) {
		t.Error("the half-built repo directory must be rolled back")
	}
}

func TestClone_TrackingSkip_PreservesRepoDir(t *testing.T) {
	dir := t.TempDir()
	repoPath := filepath.Join(dir, "repo")
	barePath := filepath.Join(repoPath, ".bare")

	// Worktree add succeeds; the best-effort fetch fails. The clone must still
	// succeed (tracking is progress.StepSkipped) and must NOT roll back the repo dir.
	runner := &mock.Runner{
		Responses: map[string]mock.Response{
			fmt.Sprintf("%s:[clone --bare --single-branch -- u %s]", dir, barePath):                      {Output: ""},
			fmt.Sprintf("%s:[config remote.origin.fetch +refs/heads/*:refs/remotes/origin/*]", barePath): {Output: ""},
			fmt.Sprintf("%s:[symbolic-ref --short HEAD]", barePath):                                      {Output: "main"},
			fmt.Sprintf("%s:[show-ref --verify refs/heads/main]", barePath):                              {Output: "abc"},
			fmt.Sprintf("%s:[worktree add %s/main main]", repoPath, repoPath):                            {Output: ""},
			fmt.Sprintf("%s:[fetch origin]", barePath):                                                   {Output: "", Err: fmt.Errorf("network down")},
		},
		OnRun: func(d string, args []string) {
			if len(args) >= 2 && args[0] == "worktree" && args[1] == "add" {
				_ = os.MkdirAll(filepath.Join(repoPath, "main"), 0o755)
			}
		},
	}

	ec := &mock.EventCollector[progress.Event]{}
	result := Clone(runner, CloneOptions{URL: "u", Location: dir, Name: "repo"}, ec.Emit)

	if result.HasFailures() {
		t.Error("a tracking-only skip must not fail the clone")
	}
	if result.WorktreePath != filepath.Join(repoPath, "main") {
		t.Errorf("worktree should be created, got WorktreePath=%q", result.WorktreePath)
	}
	if _, err := os.Stat(repoPath); err != nil {
		t.Errorf("repo dir must be preserved on a tracking skip (not rolled back): %v", err)
	}
}

func findPhase(phases []progress.Phase, name string) *progress.Phase {
	for i := range phases {
		if phases[i].Name == name {
			return &phases[i]
		}
	}
	return nil
}

// cloneSource builds a repository whose default branch is trunk, with two
// more branches and a tag, to clone from.
func cloneSource(t *testing.T) string {
	t.Helper()
	source := filepath.Join(t.TempDir(), "source")
	mustGit(t, filepath.Dir(source), "init", "-q", "-b", "trunk", source)
	mustWrite(t, filepath.Join(source, "a.txt"), "a\n")
	mustGit(t, source, "add", ".")
	mustGit(t, source, "commit", "-qm", "init")
	mustGit(t, source, "tag", "v1")
	mustGit(t, source, "branch", "feat-a")
	mustGit(t, source, "branch", "feat-b")
	return source
}

// A bare clone copies every remote branch into refs/heads. Those copies have
// no upstream and never move again, so a worktree created for one of them
// later starts from wherever the branch was on the day of the clone.
func TestClone_KeepsOnlyTheDefaultBranchAsALocalHead(t *testing.T) {
	source := cloneSource(t)

	result := Clone(realGit, CloneOptions{URL: source, Location: t.TempDir(), Name: "cloned"}, nil)

	if result.HasFailures() {
		t.Fatalf("clone failed: %+v", result)
	}
	repoPath := result.RepoPath
	if heads := mustGit(t, repoPath, "for-each-ref", "--format=%(refname)", "refs/heads"); heads != "refs/heads/trunk" {
		t.Errorf("local heads = %q, want only the default branch", heads)
	}
	for _, ref := range []string{"refs/remotes/origin/trunk", "refs/remotes/origin/feat-a", "refs/remotes/origin/feat-b", "refs/tags/v1"} {
		mustGit(t, repoPath, "show-ref", "--verify", ref)
	}
	if upstream := mustGit(t, result.WorktreePath, "rev-parse", "--abbrev-ref", "trunk@{upstream}"); upstream != "origin/trunk" {
		t.Errorf("upstream of trunk = %q, want origin/trunk", upstream)
	}

	// The remote branch moves on. A worktree made for it after a fetch has
	// to start from the new tip and track the remote.
	mustGit(t, source, "checkout", "-q", "feat-a")
	mustWrite(t, filepath.Join(source, "later.txt"), "later\n")
	mustGit(t, source, "add", ".")
	mustGit(t, source, "commit", "-qm", "later work")
	mustGit(t, repoPath, "fetch", "-q", "origin")
	worktree := filepath.Join(repoPath, "feat-a")
	mustGit(t, repoPath, "worktree", "add", "-q", worktree, "feat-a")
	if got, want := mustGit(t, worktree, "rev-parse", "HEAD"), mustGit(t, source, "rev-parse", "feat-a"); got != want {
		t.Errorf("the feat-a worktree is at %s, want the remote tip %s", got, want)
	}
	if upstream := mustGit(t, worktree, "rev-parse", "--abbrev-ref", "feat-a@{upstream}"); upstream != "origin/feat-a" {
		t.Errorf("upstream of feat-a = %q, want origin/feat-a", upstream)
	}
}

// The URL is user input. Without the option terminator, one that starts with
// a dash is read by git as an option.
func TestClone_URLStartingWithADashIsNotAnOption(t *testing.T) {
	source := cloneSource(t)
	location := t.TempDir()
	mustGit(t, location, "clone", "-q", "--bare", source, "./-source.git")

	result := Clone(realGit, CloneOptions{URL: "-source.git", Location: location, Name: "cloned"}, nil)

	if result.HasFailures() {
		_, failed, _ := progress.FirstFailure(result.Phases)
		t.Fatalf("clone failed at %q: %v (Err %v)", failed.Name, failed.Error, result.Err)
	}
	if _, err := os.Stat(filepath.Join(result.WorktreePath, "a.txt")); err != nil {
		t.Errorf("the default branch was not checked out: %v", err)
	}
}

// A remote whose HEAD is detached names no default branch, so a
// single-branch clone brings no branch at all. The clone still has to find
// main and check it out.
func TestClone_RemoteWithDetachedHEADStillChecksOutMain(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source")
	mustGit(t, filepath.Dir(source), "init", "-q", "-b", "main", source)
	mustWrite(t, filepath.Join(source, "a.txt"), "a\n")
	mustGit(t, source, "add", ".")
	mustGit(t, source, "commit", "-qm", "init")
	mustGit(t, source, "checkout", "-q", "--detach")
	mustWrite(t, filepath.Join(source, "wip.txt"), "wip\n")
	mustGit(t, source, "add", ".")
	mustGit(t, source, "commit", "-qm", "detached work")

	result := Clone(realGit, CloneOptions{URL: source, Location: t.TempDir(), Name: "cloned"}, nil)

	if result.HasFailures() {
		_, failed, _ := progress.FirstFailure(result.Phases)
		t.Fatalf("clone failed at %q: %v (Err %v)", failed.Name, failed.Error, result.Err)
	}
	if result.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want main", result.DefaultBranch)
	}
	if _, err := os.Stat(filepath.Join(result.RepoPath, "main", "a.txt")); err != nil {
		t.Errorf("main was not checked out: %v", err)
	}
	if upstream := mustGit(t, result.WorktreePath, "rev-parse", "--abbrev-ref", "main@{upstream}"); upstream != "origin/main" {
		t.Errorf("upstream of main = %q, want origin/main", upstream)
	}
}
