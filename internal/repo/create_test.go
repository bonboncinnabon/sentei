package repo

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

type mockGhRunner struct {
	responses map[string]mock.Response
}

func (m *mockGhRunner) RunGh(dir string, args ...string) (string, error) {
	key := fmt.Sprintf("%s:gh[%s]", dir, strings.Join(args, " "))
	if resp, ok := m.responses[key]; ok {
		return resp.Output, resp.Err
	}
	return "", fmt.Errorf("unexpected gh call: %s", key)
}

func TestCreate_LocalOnly(t *testing.T) {
	dir := t.TempDir()
	repoName := "my-project"
	repoPath := filepath.Join(dir, repoName)

	runner := &mock.Runner{Responses: map[string]mock.Response{
		fmt.Sprintf("%s/.bare:[init --bare --initial-branch=main]", repoPath):                              {Output: ""},
		fmt.Sprintf("%s/.bare:[config remote.origin.fetch +refs/heads/*:refs/remotes/origin/*]", repoPath): {Output: ""},
		fmt.Sprintf("%s:[worktree add %s/main -b main]", repoPath, repoPath):                               {Output: ""},
		fmt.Sprintf("%s/main:[add -A]", repoPath):                                                          {Output: ""},
		fmt.Sprintf("%s/main:[commit -m Initial commit]", repoPath):                                        {Output: ""},
	}}

	ec := &mock.EventCollector[progress.Event]{}
	opts := CreateOptions{
		Name:          repoName,
		Location:      dir,
		PublishGitHub: false,
	}
	result := Create(runner, opts, ec.Emit)

	if result.RepoPath != repoPath {
		t.Errorf("RepoPath = %q, want %q", result.RepoPath, repoPath)
	}
	if len(result.Phases) != 1 {
		t.Errorf("want 1 phase (setup only), got %d", len(result.Phases))
	}
	for _, phase := range result.Phases {
		for _, step := range phase.Steps {
			if step.Status == progress.StepFailed {
				t.Errorf("step %q failed: %v", step.Name, step.Error)
			}
		}
	}
	if len(ec.Events) == 0 {
		t.Error("expected events to be emitted")
	}
}

func TestCreate_WithGitHub(t *testing.T) {
	dir := t.TempDir()
	repoName := "my-project"
	repoPath := filepath.Join(dir, repoName)

	runner := &mock.Runner{Responses: map[string]mock.Response{
		// Setup phase
		fmt.Sprintf("%s/.bare:[init --bare --initial-branch=main]", repoPath):                              {Output: ""},
		fmt.Sprintf("%s/.bare:[config remote.origin.fetch +refs/heads/*:refs/remotes/origin/*]", repoPath): {Output: ""},
		fmt.Sprintf("%s:[worktree add %s/main -b main]", repoPath, repoPath):                               {Output: ""},
		fmt.Sprintf("%s/main:[add -A]", repoPath):                                                          {Output: ""},
		fmt.Sprintf("%s/main:[commit -m Initial commit]", repoPath):                                        {Output: ""},
		// GitHub phase (git commands) — gh git_protocol is https (gh default).
		fmt.Sprintf("%s/.bare:[remote set-url origin https://github.com/bonboncinnabon/my-project.git]", repoPath): {Output: ""},
		fmt.Sprintf("%s/main:[push -u origin main]", repoPath):                                                     {Output: ""},
		fmt.Sprintf("%s/.bare:[remote set-head origin main]", repoPath):                                            {Output: ""},
	}}
	ghRunner := &mockGhRunner{responses: map[string]mock.Response{
		fmt.Sprintf("%s:gh[api user --jq .login]", repoPath):             {Output: "bonboncinnabon"},
		fmt.Sprintf("%s:gh[repo create my-project --private]", repoPath): {Output: ""},
		fmt.Sprintf("%s:gh[config get git_protocol]", repoPath):          {Output: "https"},
	}}

	ec := &mock.EventCollector[progress.Event]{}
	opts := CreateOptions{
		Name:          repoName,
		Location:      dir,
		PublishGitHub: true,
		Visibility:    "private",
		Description:   "",
	}
	result := CreateWithGh(runner, ghRunner, opts, ec.Emit)

	if len(result.Phases) != 2 {
		t.Errorf("want 2 phases (setup + github), got %d", len(result.Phases))
	}
	for _, phase := range result.Phases {
		for _, step := range phase.Steps {
			if step.Status == progress.StepFailed {
				t.Errorf("step %q failed: %v", step.Name, step.Error)
			}
		}
	}
}

func TestGhRemoteURL_RespectsConfiguredProtocol(t *testing.T) {
	sshGh := &mockGhRunner{responses: map[string]mock.Response{
		"/r:gh[config get git_protocol]": {Output: "ssh"},
	}}
	if got := ghRemoteURL(sshGh, "/r", "u", "n"); got != "git@github.com:u/n.git" {
		t.Errorf("ssh protocol: got %q", got)
	}

	httpsGh := &mockGhRunner{responses: map[string]mock.Response{
		"/r:gh[config get git_protocol]": {Output: "https"},
	}}
	if got := ghRemoteURL(httpsGh, "/r", "u", "n"); got != "https://github.com/u/n.git" {
		t.Errorf("https protocol: got %q", got)
	}

	// On error, default to HTTPS — gh's own default and what token auth uses.
	brokenGh := &mockGhRunner{responses: map[string]mock.Response{}}
	if got := ghRemoteURL(brokenGh, "/r", "u", "n"); got != "https://github.com/u/n.git" {
		t.Errorf("default: got %q", got)
	}
}

func TestCreate_DirAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	repoName := "existing"
	os.MkdirAll(filepath.Join(dir, repoName), 0755)

	runner := &mock.Runner{Responses: map[string]mock.Response{}}
	ec := &mock.EventCollector[progress.Event]{}
	opts := CreateOptions{
		Name:     repoName,
		Location: dir,
	}
	result := Create(runner, opts, ec.Emit)

	if len(result.Phases) == 0 {
		t.Fatal("expected at least one phase")
	}
	if !result.Phases[0].HasFailures() {
		t.Error("expected setup phase to have failures when dir already exists")
	}
}

func TestCreate_GitHubPhaseFailure_LocalStillUsable(t *testing.T) {
	dir := t.TempDir()
	repoName := "my-project"
	repoPath := filepath.Join(dir, repoName)

	runner := &mock.Runner{Responses: map[string]mock.Response{
		// Setup phase succeeds
		fmt.Sprintf("%s/.bare:[init --bare --initial-branch=main]", repoPath):                              {Output: ""},
		fmt.Sprintf("%s/.bare:[config remote.origin.fetch +refs/heads/*:refs/remotes/origin/*]", repoPath): {Output: ""},
		fmt.Sprintf("%s:[worktree add %s/main -b main]", repoPath, repoPath):                               {Output: ""},
		fmt.Sprintf("%s/main:[add -A]", repoPath):                                                          {Output: ""},
		fmt.Sprintf("%s/main:[commit -m Initial commit]", repoPath):                                        {Output: ""},
	}}
	ghRunner := &mockGhRunner{responses: map[string]mock.Response{
		// GitHub phase fails at user lookup
		fmt.Sprintf("%s:gh[api user --jq .login]", repoPath): {Output: "", Err: fmt.Errorf("gh: not authenticated")},
	}}

	ec := &mock.EventCollector[progress.Event]{}
	opts := CreateOptions{
		Name:          repoName,
		Location:      dir,
		PublishGitHub: true,
		Visibility:    "private",
	}
	result := CreateWithGh(runner, ghRunner, opts, ec.Emit)

	// Local repo still usable despite GitHub failure
	if result.RepoPath != repoPath {
		t.Errorf("RepoPath should still be set: got %q", result.RepoPath)
	}
	if len(result.Phases) != 2 {
		t.Fatalf("want 2 phases, got %d", len(result.Phases))
	}
	if result.Phases[0].HasFailures() {
		t.Error("setup phase should not have failures")
	}
	if !result.Phases[1].HasFailures() {
		t.Error("github phase should have failures")
	}
}

func TestCreate_PushFailure_ReportsOrphanedRepo(t *testing.T) {
	dir := t.TempDir()
	repoName := "my-project"
	repoPath := filepath.Join(dir, repoName)
	pushErr := fmt.Errorf("Permission denied (publickey)")

	runner := &mock.Runner{Responses: map[string]mock.Response{
		fmt.Sprintf("%s/.bare:[init --bare --initial-branch=main]", repoPath):                                      {Output: ""},
		fmt.Sprintf("%s/.bare:[config remote.origin.fetch +refs/heads/*:refs/remotes/origin/*]", repoPath):         {Output: ""},
		fmt.Sprintf("%s:[worktree add %s/main -b main]", repoPath, repoPath):                                       {Output: ""},
		fmt.Sprintf("%s/main:[add -A]", repoPath):                                                                  {Output: ""},
		fmt.Sprintf("%s/main:[commit -m Initial commit]", repoPath):                                                {Output: ""},
		fmt.Sprintf("%s/.bare:[remote set-url origin https://github.com/bonboncinnabon/my-project.git]", repoPath): {Output: ""},
		fmt.Sprintf("%s/main:[push -u origin main]", repoPath):                                                     {Output: "", Err: pushErr},
	}}
	ghRunner := &mockGhRunner{responses: map[string]mock.Response{
		fmt.Sprintf("%s:gh[api user --jq .login]", repoPath):             {Output: "bonboncinnabon"},
		fmt.Sprintf("%s:gh[repo create my-project --private]", repoPath): {Output: ""},
		fmt.Sprintf("%s:gh[config get git_protocol]", repoPath):          {Output: "https"},
	}}

	ec := &mock.EventCollector[progress.Event]{}
	opts := CreateOptions{Name: repoName, Location: dir, PublishGitHub: true, Visibility: "private"}
	result := CreateWithGh(runner, ghRunner, opts, ec.Emit)

	var pushStep *progress.StepResult
	for i := range result.Phases {
		if result.Phases[i].Name != "GitHub" {
			continue
		}
		for j := range result.Phases[i].Steps {
			if result.Phases[i].Steps[j].Name == "Push to GitHub" {
				pushStep = &result.Phases[i].Steps[j]
			}
		}
	}
	if pushStep == nil || pushStep.Status != progress.StepFailed {
		t.Fatal("expected a failed 'Push to GitHub' step")
	}
	msg := pushStep.Error.Error()
	if !strings.Contains(msg, opts.Name) {
		t.Errorf("push error should name the orphaned repo %q: %v", opts.Name, msg)
	}
	if !strings.Contains(msg, "delete it or push") {
		t.Errorf("push error should guide the user on the orphaned remote repo: %v", msg)
	}
	if !errors.Is(pushStep.Error, pushErr) {
		t.Errorf("push error must wrap the original error: %v", msg)
	}
}

func TestCreateResult_SetupFailed(t *testing.T) {
	setupBroken := CreateResult{Phases: []progress.Phase{
		{Name: "Setup", Steps: []progress.StepResult{{Name: "Initial commit", Status: progress.StepFailed, Error: fmt.Errorf("exit 128")}}},
	}}
	if failed, err := setupBroken.SetupFailed(); !failed || err == nil {
		t.Errorf("a Setup-phase failure must be hard, got failed=%v err=%v", failed, err)
	}

	githubOnly := CreateResult{Phases: []progress.Phase{
		{Name: "Setup", Steps: []progress.StepResult{{Status: progress.StepDone}}},
		{Name: PhaseGitHub, Steps: []progress.StepResult{{Status: progress.StepFailed, Error: fmt.Errorf("push")}}},
	}}
	if failed, _ := githubOnly.SetupFailed(); failed {
		t.Error("a GitHub-only failure must be soft (local repo is fine)")
	}

	clean := CreateResult{Phases: []progress.Phase{{Name: "Setup", Steps: []progress.StepResult{{Status: progress.StepDone}}}}}
	if failed, _ := clean.SetupFailed(); failed {
		t.Error("a clean result must not report a setup failure")
	}
}

// Without init.defaultBranch, `git init --bare` points HEAD at master while
// the only branch sentei creates is main. HEAD is what names the default
// branch for protection and for "merged".
func TestCreate_BareHEADNamesMainWithoutInitDefaultBranch(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	for _, name := range []string{"GIT_AUTHOR_NAME", "GIT_COMMITTER_NAME"} {
		t.Setenv(name, "sentei-test")
	}
	for _, name := range []string{"GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(name, "test@sentei.invalid")
	}
	dir := t.TempDir()

	result := CreateWithGh(&git.GitRunner{}, &mockGhRunner{}, CreateOptions{Name: "proj", Location: dir}, nil)

	if failed, err := result.SetupFailed(); failed {
		t.Fatalf("create failed: %v", err)
	}
	if got := git.DetectDefaultBranch(&git.GitRunner{}, result.RepoPath); got != "main" {
		t.Errorf("default branch = %q, want main", got)
	}
}

// When gh cannot be started there is no stderr to report. The reason it
// failed must not be dropped with it.
func TestDefaultGhRunner_ReportsWhyGhDidNotRun(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := (&DefaultGhRunner{}).RunGh(t.TempDir(), "api", "user")

	if err == nil {
		t.Fatal("expected an error with no gh on PATH")
	}
	if !errors.Is(err, exec.ErrNotFound) {
		t.Errorf("err = %q, want it to carry the reason gh did not run", err)
	}
	if !strings.HasPrefix(err.Error(), "gh api user: ") {
		t.Errorf("err = %q, want it to name the command", err)
	}
}

func TestDefaultGhRunner_PrefersStderr(t *testing.T) {
	bin := t.TempDir()
	script := "#!/bin/sh\necho 'HTTP 401: Bad credentials' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err := (&DefaultGhRunner{}).RunGh(t.TempDir(), "api", "user")

	if err == nil || err.Error() != "gh api user: HTTP 401: Bad credentials" {
		t.Errorf("err = %v, want gh's own message", err)
	}
}

// A hook or a git alias that starts sentei exports GIT_DIR to it. gh runs git
// to find the repository it works on, and has to find the one in its
// directory.
func TestDefaultGhRunner_DropsInheritedGitDir(t *testing.T) {
	bin := t.TempDir()
	script := "#!/bin/sh\necho \"GIT_DIR=${GIT_DIR-unset} GIT_WORK_TREE=${GIT_WORK_TREE-unset}\"\n"
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	foreign := t.TempDir()
	t.Setenv("GIT_DIR", filepath.Join(foreign, ".git"))
	t.Setenv("GIT_WORK_TREE", foreign)

	got, err := (&DefaultGhRunner{}).RunGh(t.TempDir(), "repo", "create", "project", "--private")
	if err != nil {
		t.Fatal(err)
	}
	if want := "GIT_DIR=unset GIT_WORK_TREE=unset"; got != want {
		t.Errorf("gh saw %q, want %q: an inherited GIT_DIR names another repository", got, want)
	}
}
