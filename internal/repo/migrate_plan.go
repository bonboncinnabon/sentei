package repo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/git"
)

// MigratePlan describes an in-place conversion that PlanMigrate has cleared.
type MigratePlan struct {
	RepoPath      string // absolute repository root; it becomes the container
	Branch        string // branch checked out in the working tree
	WorktreePath  string // directory the working tree moves into
	DefaultBranch string // branch the bare HEAD will name

	// What the conversion handles besides the checkout itself.
	LinkedWorktrees int      // linked worktrees; they stay where they are
	Submodules      int      // submodules checked out in the working tree
	RelativeRemotes []string // remotes whose relative path becomes absolute

	linked     []string // directories of the linked worktrees
	kept       []string // those inside the root, relative to it
	relinks    []relink
	submodules []submodule
	remotes    []remoteURL
	// worktreeConfig is extensions.worktreeConfig: each worktree reads a
	// config.worktree of its own on top of the shared config.
	worktreeConfig bool
	// state is what git reported for the checkout before the conversion. The
	// new worktree must report the same, or the conversion is rolled back.
	state []string
}

// relink is a .git file that reaches the repository through <root>/.git and
// has to reach it through <root>/.bare: the one of a linked worktree, which
// stays where it is, or of a submodule checked out in one.
type relink struct {
	gitfile  string
	link     []byte // content before the conversion
	relinked []byte // and after it
}

// submodule is a submodule whose working directory points at a repository
// under .git/modules. Everything the rollback puts back is read up front.
type submodule struct {
	work   string      // working directory, relative to the repository root
	gitDir string      // its repository
	link   []byte      // content of the .git file in the working directory
	mode   fs.FileMode // and its permissions
	config []byte      // content of the repository's config
	// absorb marks a submodule the superproject tracks: git moves its
	// repository into the new worktree's own modules directory.
	absorb bool
}

// remoteURL is a remote URL or push URL that is a path relative to the
// checkout, and the absolute path it resolves to.
type remoteURL struct{ key, relative, absolute string }

// migrateBlockers are entries under .git that hold state the conversion
// cannot carry into a linked worktree. A directory only counts when it has
// content.
var migrateBlockers = []struct{ path, reason string }{
	{"index.lock", "another git command is running in this repository (.git/index.lock exists); wait for it to finish, then retry"},
	{"MERGE_HEAD", "a merge is in progress; finish or abort it (git merge --abort), then retry"},
	{"CHERRY_PICK_HEAD", "a cherry-pick is in progress; finish or abort it (git cherry-pick --abort), then retry"},
	{"REVERT_HEAD", "a revert is in progress; finish or abort it (git revert --abort), then retry"},
	{"sequencer", "a cherry-pick or revert sequence is in progress; finish or abort it (--continue or --abort), then retry"},
	{"rebase-merge", "a rebase is in progress; finish or abort it (git rebase --abort), then retry"},
	{"rebase-apply", "a rebase or git am is in progress; finish or abort it (git rebase --abort or git am --abort), then retry"},
	{"BISECT_LOG", "a bisect is in progress; end it (git bisect reset), then retry"},
}

// PlanMigrate checks that the repository at repoPath can be converted in
// place and reports what the conversion will do. It changes nothing.
func PlanMigrate(runner git.CommandRunner, repoPath string) (MigratePlan, error) {
	root, err := filepath.Abs(repoPath)
	if err != nil {
		return MigratePlan{}, fmt.Errorf("resolving repository path %q: %w", repoPath, err)
	}
	gitDir := filepath.Join(root, ".git")
	switch info, err := os.Lstat(gitDir); {
	case errors.Is(err, fs.ErrNotExist):
		if top, err := runner.Run(root, "rev-parse", "--show-toplevel"); err == nil && top != "" {
			return MigratePlan{}, fmt.Errorf("%s is inside a git repository, not at its root; migrate %s instead", root, top)
		}
		return MigratePlan{}, fmt.Errorf("%s is not a git repository (it has no .git directory); there is nothing to migrate", root)
	case err != nil:
		return MigratePlan{}, fmt.Errorf("reading %s: %w", gitDir, err)
	case !info.IsDir():
		if exists(filepath.Join(root, ".bare")) {
			return MigratePlan{}, fmt.Errorf("%s already uses the bare layout (its .git points at .bare); there is nothing to migrate", root)
		}
		return MigratePlan{}, fmt.Errorf("%s is a linked worktree or submodule (its .git is a file, not a directory); migrate the main checkout instead", root)
	}
	if exists(filepath.Join(root, ".bare")) {
		return MigratePlan{}, fmt.Errorf("%s already has a .bare entry, which the conversion needs for the repository data; move it away and retry", root)
	}
	for _, blocker := range migrateBlockers {
		if hasContent(filepath.Join(gitDir, blocker.path)) {
			return MigratePlan{}, errors.New(blocker.reason)
		}
	}
	plan := MigratePlan{RepoPath: root}
	// git takes its extensions from the repository's own config only.
	worktreeConfig, _ := runner.Run(root, "config", "--local", "--type=bool", "--get", "extensions.worktreeConfig")
	plan.worktreeConfig = worktreeConfig == "true"
	// With per-worktree config git keeps core.worktree in config.worktree.
	scopes := []string{"--local"}
	if plan.worktreeConfig {
		scopes = append(scopes, "--worktree")
	}
	for _, scope := range scopes {
		if worktree, _ := runner.Run(root, "config", scope, "--get", "core.worktree"); worktree != "" {
			return MigratePlan{}, fmt.Errorf("core.worktree is set (%s), which a bare repository cannot have; if the working tree is this directory, unset it (git config %s --unset core.worktree), then retry", worktree, scope)
		}
	}
	if included := includedFromWorkingTree(runner, root); included != "" {
		return MigratePlan{}, fmt.Errorf("the repository config includes %s, which moves with the working tree, and git skips an include it cannot find; remove the include from .git/config, migrate, then add it back with the new path", included)
	}

	plan.Branch, err = runner.Run(root, "branch", "--show-current")
	if err != nil {
		return MigratePlan{}, fmt.Errorf("reading the current branch: %w", err)
	}
	if plan.Branch == "" {
		return MigratePlan{}, errors.New("HEAD is detached; check out a branch first (git switch <branch>), then retry")
	}
	if _, err := runner.Run(root, "rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
		return MigratePlan{}, fmt.Errorf("branch %q has no commits yet; make a first commit, then retry", plan.Branch)
	}
	plan.WorktreePath = git.WorktreePath(root, plan.Branch)
	plan.DefaultBranch = migrateDefaultBranch(runner, root, plan.Branch)

	if plan.linked, plan.relinks, err = linkedWorktrees(runner, gitDir); err != nil {
		return MigratePlan{}, err
	}
	for _, linked := range plan.linked {
		if sameFile(linked, plan.WorktreePath) {
			return MigratePlan{}, fmt.Errorf("%s is a linked worktree and is where the %q worktree goes; move it (git worktree move %s <new-path>) or switch to another branch, then retry", plan.WorktreePath, plan.Branch, plan.WorktreePath)
		}
		if inside, ok := pathInside(root, linked); ok {
			plan.kept = append(plan.kept, inside)
		}
	}
	if exists(plan.WorktreePath) {
		return MigratePlan{}, fmt.Errorf("%s already exists and is where the %q worktree goes; rename it or switch to another branch, then retry", plan.WorktreePath, plan.Branch)
	}
	if plan.submodules, err = submodulesUnder(runner, root, filepath.Join(gitDir, "modules"), false); err != nil {
		return MigratePlan{}, err
	}
	plan.remotes = relativeRemoteURLs(runner, root)
	if plan.state, err = checkoutState(runner, root, plan.kept, nil); err != nil {
		return MigratePlan{}, fmt.Errorf("reading the working tree status: %w", err)
	}

	plan.LinkedWorktrees, plan.Submodules = len(plan.linked), len(plan.submodules)
	for _, remote := range plan.remotes {
		// remote.<name>.url; the name itself may hold dots.
		name := remote.key[len("remote."):strings.LastIndex(remote.key, ".")]
		if !slices.Contains(plan.RelativeRemotes, name) {
			plan.RelativeRemotes = append(plan.RelativeRemotes, name)
		}
	}
	return plan, nil
}

// includedFromWorkingTree returns a config file that .git/config includes
// from inside the working tree, or "" when there is none. An include that
// resolves into .git or outside the repository keeps resolving from .bare.
func includedFromWorkingTree(runner git.CommandRunner, root string) string {
	// With -z the output is origin, NUL, key, NUL, repeated, and paths are
	// not quoted.
	entries, _ := runner.Run(root, "config", "--local", "--includes", "--show-origin", "--name-only", "--list", "-z")
	fields := strings.Split(entries, "\x00")
	for index := 0; index+1 < len(fields); index += 2 {
		origin := strings.TrimPrefix(fields[index], "file:")
		if !filepath.IsAbs(origin) {
			origin = filepath.Join(root, origin)
		}
		if git.PathContains(root, origin) && !git.PathContains(filepath.Join(root, ".git"), origin) {
			return filepath.Clean(origin)
		}
	}
	return ""
}

// migrateDefaultBranch picks the branch the bare HEAD should name: the
// remote's default, then main, then master, whichever exists locally first.
// With none of them it stays on the branch that is checked out.
func migrateDefaultBranch(runner git.CommandRunner, root, current string) string {
	candidates := []string{"main", "master"}
	if remoteHead, err := runner.Run(root, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		candidates = append([]string{strings.TrimPrefix(remoteHead, "origin/")}, candidates...)
	}
	for _, candidate := range candidates {
		if git.BranchExists(runner, root, candidate) {
			return candidate
		}
	}
	return current
}

// linkedWorktrees reads the worktrees registered under gitDir/worktrees and
// works out the .git files that change once gitDir is named .bare: each
// worktree's own, and those of the submodules checked out in it. Every
// worktree has to be there and point back: the rollback prunes the new
// worktree's registration, and pruning would take a missing one with it.
func linkedWorktrees(runner git.CommandRunner, gitDir string) (dirs []string, relinks []relink, err error) {
	entries, err := os.ReadDir(filepath.Join(gitDir, "worktrees"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, nil, fmt.Errorf("reading the linked worktrees: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		admin := filepath.Join(gitDir, "worktrees", entry.Name())
		// The gitdir file names the worktree's .git file. A relative path
		// counts from the directory the gitdir file is in.
		recorded, err := os.ReadFile(filepath.Join(admin, "gitdir"))
		if err != nil {
			return nil, nil, fmt.Errorf("the linked worktree %q has no readable record of where it is; drop it with git worktree prune, then retry", entry.Name())
		}
		gitfile := strings.TrimSpace(string(recorded))
		if !filepath.IsAbs(gitfile) {
			gitfile = filepath.Join(admin, gitfile)
		}
		dir := filepath.Dir(gitfile)
		link, target, err := readGitfile(gitfile)
		if err != nil {
			return nil, nil, fmt.Errorf("the linked worktree at %s cannot be read (its directory or its .git file is missing); bring it back, or drop it with git worktree prune (after git worktree unlock if it is locked), then retry", dir)
		}
		// git writes these paths with forward slashes on every platform.
		through, throughBare := "/.git/worktrees/"+entry.Name(), "/.bare/worktrees/"+entry.Name()
		pointer, ok := strings.CutSuffix(strings.TrimRight(string(link), "\r\n"), through)
		if !ok || !sameFile(target, admin) {
			return nil, nil, fmt.Errorf("the linked worktree at %s does not point back at %s; run git worktree repair in %s, then retry", dir, admin, filepath.Dir(gitDir))
		}
		dirs = append(dirs, dir)
		relinks = append(relinks, relink{gitfile: gitfile, link: link, relinked: []byte(pointer + throughBare + "\n")})

		// A submodule's repository names its working directory by a path
		// that climbs out of the git directory without naming it, so only
		// the .git file in the working directory changes.
		submodules, err := submodulesUnder(runner, dir, filepath.Join(admin, "modules"), false)
		if err != nil {
			return nil, nil, err
		}
		for _, sub := range submodules {
			pointer := strings.TrimRight(string(sub.link), "\r\n")
			if strings.Count(pointer, through+"/modules/") != 1 {
				return nil, nil, fmt.Errorf("the submodule %s of the linked worktree at %s does not point into %s; check it out again there (git submodule deinit, then git submodule update --init), then retry", sub.work, dir, admin)
			}
			relinks = append(relinks, relink{
				gitfile: filepath.Join(dir, sub.work, ".git"), link: sub.link,
				relinked: []byte(strings.Replace(pointer, through+"/modules/", throughBare+"/modules/", 1) + "\n"),
			})
		}
	}
	return dirs, relinks, nil
}

// submodulesUnder finds the repositories under a modules directory whose
// working directory points back at them, parents before the submodules
// nested in them. A repository without such a working directory (a submodule
// that was deinitialised, or belongs to another branch) is left out: it moves
// with the modules directory and git links it again when it is checked out.
func submodulesUnder(runner git.CommandRunner, root, modules string, nested bool) ([]submodule, error) {
	entries, err := os.ReadDir(modules)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("reading the submodule repositories: %w", err)
	}
	var found []submodule
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(modules, entry.Name())
		below, belowNested := dir, nested
		if head, err := os.Lstat(filepath.Join(dir, "HEAD")); err == nil && head.Mode().IsRegular() {
			sub, active, err := readSubmodule(runner, root, dir)
			if err != nil {
				return nil, err
			}
			if !active {
				continue
			}
			// git absorbs a nested repository together with its parent.
			sub.absorb = sub.absorb && !nested
			found = append(found, sub)
			below, belowNested = filepath.Join(dir, "modules"), true
		}
		// Without a HEAD the directory is part of a submodule name that has
		// a slash in it, and the repositories are further down.
		inner, err := submodulesUnder(runner, root, below, belowNested)
		if err != nil {
			return nil, err
		}
		found = append(found, inner...)
	}
	return found, nil
}

// readSubmodule describes the submodule whose repository is gitDir. It is
// active when the working directory the repository names has a .git file
// that points back.
func readSubmodule(runner git.CommandRunner, root, gitDir string) (sub submodule, active bool, err error) {
	configPath := filepath.Join(gitDir, "config")
	work, err := runner.Run(root, "config", "--file", configPath, "--get", "core.worktree")
	if err != nil {
		return submodule{}, false, nil
	}
	if !filepath.IsAbs(work) {
		work = filepath.Join(gitDir, work)
	}
	gitfile := filepath.Join(work, ".git")
	link, target, err := readGitfile(gitfile)
	if err != nil || !sameFile(target, gitDir) {
		return submodule{}, false, nil
	}
	inside, ok := pathInside(root, work)
	if !ok {
		return submodule{}, false, fmt.Errorf("the submodule repository %s has its working directory at %s, outside the repository; remove that submodule (git submodule deinit), then retry", gitDir, work)
	}
	info, err := os.Lstat(gitfile)
	if err != nil {
		return submodule{}, false, fmt.Errorf("reading %s: %w", gitfile, err)
	}
	config, err := os.ReadFile(configPath)
	if err != nil {
		return submodule{}, false, fmt.Errorf("reading %s: %w", configPath, err)
	}
	// A submodule the index does not list (left behind by a branch switch)
	// is not git's to absorb: it keeps its repository inside it.
	entry, _ := runner.Run(root, "ls-files", "--stage", "--", ":(literal)"+filepath.ToSlash(inside))
	return submodule{
		work: inside, gitDir: gitDir, link: link, mode: info.Mode().Perm(), config: config,
		absorb: strings.HasPrefix(entry, "160000 "),
	}, true, nil
}

// relativeRemoteURLs returns the remote URLs and push URLs that are a path
// relative to the checkout and lead somewhere. git resolves such a path from
// the top of the working tree, which the conversion moves one level down.
func relativeRemoteURLs(runner git.CommandRunner, root string) []remoteURL {
	// The path counts from where the root really is, not from a symlink
	// that leads to it.
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil
	}
	// With -z each entry is key, newline, value, NUL.
	entries, _ := runner.Run(root, "config", "--local", "-z", "--get-regexp", `^remote\..*\.(url|pushurl)$`)
	var remotes []remoteURL
	for _, entry := range strings.Split(entries, "\x00") {
		key, value, _ := strings.Cut(entry, "\n")
		if !isRelativePath(value) {
			continue
		}
		// git also finds a repository when the name leaves out its .git
		// suffix. The absolute path keeps leaving it out.
		absolute := filepath.Join(resolved, value)
		if exists(absolute) || exists(absolute+".git") {
			remotes = append(remotes, remoteURL{key: key, relative: value, absolute: absolute})
		}
	}
	return remotes
}

// isRelativePath reports whether git reads a remote URL as a relative local
// path: it is not absolute and has no colon before its first slash, which
// would make it a URL with a scheme or an SSH address (host:path).
func isRelativePath(url string) bool {
	if url == "" || filepath.IsAbs(url) {
		return false
	}
	colon, slash := strings.Index(url, ":"), strings.Index(url, "/")
	return colon < 0 || (slash >= 0 && slash < colon)
}

// checkoutProbes are the questions whose answers must be the same for the new
// worktree as they were for the checkout. An optional one fails when there is
// nothing to ask about (a checkout that is not sparse has no pattern list),
// and is only asked again when it had an answer.
var checkoutProbes = []struct {
	name     string
	args     []string
	optional bool
	// excludes marks the probe that takes the paths to leave out.
	excludes bool
}{
	// Without the optional index lock, so a check never rewrites the index.
	{name: "git status", args: []string{"--no-optional-locks", "status", "--porcelain", "--"}, excludes: true},
	{name: "git submodule status", args: []string{"submodule", "status", "--recursive"}, optional: true},
	{name: "git sparse-checkout list", args: []string{"sparse-checkout", "list"}, optional: true},
}

// checkoutState answers checkoutProbes for the checkout at path. The kept
// paths, relative to the top of the checkout, are left out of the status:
// they do not move with it. With before, the answers of an earlier call, an
// optional probe that had none is skipped.
func checkoutState(runner git.CommandRunner, path string, kept, before []string) ([]string, error) {
	state := make([]string, len(checkoutProbes))
	for index, probe := range checkoutProbes {
		if probe.optional && before != nil && before[index] == "" {
			continue
		}
		args := slices.Clone(probe.args)
		if probe.excludes {
			for _, keep := range kept {
				args = append(args, ":(exclude,top,literal)"+filepath.ToSlash(keep))
			}
		}
		answer, err := runner.Run(path, args...)
		if err != nil && !probe.optional {
			return nil, err
		}
		state[index] = answer
	}
	return state, nil
}

// readGitfile reads a .git file and returns its content and the directory it
// points at. A relative path counts from the directory that holds the file.
func readGitfile(path string) (content []byte, target string, err error) {
	content, err = os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	pointer, ok := strings.CutPrefix(strings.TrimRight(string(content), "\r\n"), "gitdir: ")
	if !ok {
		return nil, "", fmt.Errorf("%s is not a .git file", path)
	}
	if !filepath.IsAbs(pointer) {
		pointer = filepath.Join(filepath.Dir(path), pointer)
	}
	return content, filepath.Clean(pointer), nil
}

// pathInside returns dir relative to root when it lies inside root.
// Directories are compared as files, not by name: git records resolved paths,
// while the root may be reached through a symlink or, where the filesystem
// ignores case, be spelled differently. The result is matched against
// directory listings when the working tree moves, so it uses the names the
// directory entries have wherever they can be read.
func pathInside(root, dir string) (string, bool) {
	rootInfo, err := os.Stat(root)
	if err != nil {
		return "", false
	}
	if dir, err = filepath.EvalSymlinks(dir); err != nil {
		return "", false
	}
	// dir and its parents below the root, innermost first.
	var below []fs.FileInfo
	for {
		info, err := os.Stat(dir)
		if err != nil {
			return "", false
		}
		if os.SameFile(info, rootInfo) {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		below, dir = append(below, info), parent
	}
	inside := ""
	for _, info := range slices.Backward(below) {
		name := info.Name()
		entries, _ := os.ReadDir(filepath.Join(root, inside))
		for _, entry := range entries {
			if entryInfo, err := entry.Info(); err == nil && os.SameFile(entryInfo, info) {
				name = entry.Name()
				break
			}
		}
		inside = filepath.Join(inside, name)
	}
	return inside, inside != ""
}

// sameFile reports whether two paths lead to the same file or directory.
func sameFile(left, right string) bool {
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	return leftErr == nil && rightErr == nil && os.SameFile(leftInfo, rightInfo)
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return !errors.Is(err, fs.ErrNotExist)
}

// hasContent reports whether path exists and, for a directory, is not empty.
func hasContent(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	if !info.IsDir() {
		return true
	}
	entries, err := os.ReadDir(path)
	return err != nil || len(entries) > 0
}
