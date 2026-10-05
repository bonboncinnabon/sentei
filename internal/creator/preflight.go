package creator

import (
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/git"
	"gopkg.in/yaml.v3"
)

type dependencyTarget struct {
	ecosystem config.EcosystemConfig
	workspace string
}

// prepareDependencyTargets reads workspace declarations from the base commit.
// It never examines a future worktree, so all dependency work is known before
// progress starts and before any mutating command can run.
func prepareDependencyTargets(runner git.CommandRunner, opts Options) ([]dependencyTarget, error) {
	needsTree := false
	for _, ecosystem := range opts.Ecosystems {
		if strings.TrimSpace(ecosystem.Install.Command) == "" && strings.TrimSpace(ecosystem.Install.WorkspaceInstall) == "" {
			continue
		}
		if ecosystem.Install.WorkspaceDetect != "" && ecosystem.Install.WorkspaceInstall != "" {
			needsTree = true
		}
	}
	if !needsTree {
		return rootDependencyTargets(opts.Ecosystems), nil
	}

	treeOutput, err := runner.Run(opts.RepoPath, "ls-tree", "-r", "--name-only", opts.BaseBranch)
	if err != nil {
		return nil, fmt.Errorf("reading base branch %q tree: %w", opts.BaseBranch, err)
	}
	treeFiles, treeDirs := indexTree(treeOutput)
	var targets []dependencyTarget
	for _, ecosystem := range opts.Ecosystems {
		install := ecosystem.Install
		if strings.TrimSpace(install.Command) == "" && strings.TrimSpace(install.WorkspaceInstall) == "" {
			continue
		}
		manifest := path.Clean(install.WorkspaceDetect)
		if install.WorkspaceDetect == "" || install.WorkspaceInstall == "" || !treeFiles[manifest] {
			if strings.TrimSpace(install.Command) != "" {
				targets = append(targets, dependencyTarget{ecosystem: ecosystem})
			}
			continue
		}
		data, err := runner.Run(opts.RepoPath, "show", opts.BaseBranch+":"+manifest)
		if err != nil {
			return nil, fmt.Errorf("reading %s from base branch %q: %w", manifest, opts.BaseBranch, err)
		}
		patterns, err := workspacePatterns(manifest, []byte(data))
		if err != nil {
			if strings.TrimSpace(install.Command) != "" {
				targets = append(targets, dependencyTarget{ecosystem: ecosystem})
			}
			continue
		}
		members, err := matchWorkspaceDirs(patterns, treeDirs)
		if err != nil {
			if strings.TrimSpace(install.Command) != "" {
				targets = append(targets, dependencyTarget{ecosystem: ecosystem})
			}
			continue
		}
		if len(members) == 0 {
			if strings.TrimSpace(install.Command) != "" {
				targets = append(targets, dependencyTarget{ecosystem: ecosystem})
			}
			continue
		}
		for _, member := range members {
			targets = append(targets, dependencyTarget{ecosystem: ecosystem, workspace: member})
		}
	}
	return targets, nil
}

func rootDependencyTargets(ecosystems []config.EcosystemConfig) []dependencyTarget {
	var targets []dependencyTarget
	for _, ecosystem := range ecosystems {
		if strings.TrimSpace(ecosystem.Install.Command) != "" {
			targets = append(targets, dependencyTarget{ecosystem: ecosystem})
		}
	}
	return targets
}

func indexTree(output string) (map[string]bool, map[string]bool) {
	files := make(map[string]bool)
	dirs := make(map[string]bool)
	for _, line := range strings.Split(output, "\n") {
		file := strings.TrimSpace(line)
		if file == "" {
			continue
		}
		file = path.Clean(file)
		files[file] = true
		for dir := path.Dir(file); dir != "." && dir != "/"; dir = path.Dir(dir) {
			dirs[dir] = true
		}
	}
	return files, dirs
}

func workspacePatterns(manifest string, data []byte) ([]string, error) {
	switch path.Base(manifest) {
	case "pnpm-workspace.yaml":
		var document struct {
			Packages []string `yaml:"packages"`
		}
		if err := yaml.Unmarshal(data, &document); err != nil {
			return nil, err
		}
		return document.Packages, nil
	case "package.json":
		var document struct {
			Workspaces json.RawMessage `json:"workspaces"`
		}
		if err := json.Unmarshal(data, &document); err != nil {
			return nil, err
		}
		if len(document.Workspaces) == 0 || string(document.Workspaces) == "null" {
			return nil, nil
		}
		var patterns []string
		if err := json.Unmarshal(document.Workspaces, &patterns); err == nil {
			return patterns, nil
		}
		var object struct {
			Packages []string `json:"packages"`
		}
		if err := json.Unmarshal(document.Workspaces, &object); err != nil {
			return nil, fmt.Errorf("workspaces must be an array or packages object: %w", err)
		}
		return object.Packages, nil
	default:
		return nil, fmt.Errorf("unsupported workspace manifest %q", manifest)
	}
}

func matchWorkspaceDirs(patterns []string, dirs map[string]bool) ([]string, error) {
	seen := make(map[string]bool)
	for _, pattern := range patterns {
		pattern = strings.TrimPrefix(path.Clean(pattern), "./")
		if pattern == "." || strings.HasPrefix(pattern, "../") || path.IsAbs(pattern) {
			return nil, fmt.Errorf("workspace pattern %q escapes repository root", pattern)
		}
		for dir := range dirs {
			matched, err := path.Match(pattern, dir)
			if err != nil {
				return nil, fmt.Errorf("invalid workspace pattern %q: %w", pattern, err)
			}
			if matched {
				seen[dir] = true
			}
		}
	}
	members := make([]string, 0, len(seen))
	for member := range seen {
		members = append(members, member)
	}
	sort.Strings(members)
	return members, nil
}
