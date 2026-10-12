package cmd

import (
	"flag"
	"fmt"
)

// repoPathArg returns the optional positional repository path. Go's flag
// package stops parsing at the first positional, so anything after the path,
// including a flag such as --dry-run, would otherwise be silently ignored and
// the command would run for real.
func repoPathArg(fs *flag.FlagSet) (string, error) {
	if fs.NArg() > 1 {
		return "", fmt.Errorf("unexpected argument %q after repository path %q: flags must come before the path", fs.Arg(1), fs.Arg(0))
	}
	return fs.Arg(0), nil
}
