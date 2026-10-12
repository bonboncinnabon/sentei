# Progress arc deterministic recordings

These VHS sources exercise the real Sentei binary against a fixed, local-only
bare repository. Generated binaries, repositories, GIFs, and inspection frames
stay under `/tmp/sentei-vhs-progress-arc` and are never committed.

```bash
.demos/progress-arc/setup-fixture_test.sh
.demos/progress-arc/setup-fixture.sh
mkdir -p /tmp/sentei-vhs-progress-arc/bin
git worktree add --detach /tmp/sentei-vhs-progress-arc/source-before 778421a
(cd /tmp/sentei-vhs-progress-arc/source-before && go build -o /tmp/sentei-vhs-progress-arc/bin/sentei-before .)
printf '%s\n' 778421a >/tmp/sentei-vhs-progress-arc/bin/sentei-before.sha
go build -o /tmp/sentei-vhs-progress-arc/bin/sentei-after .
git rev-parse --short=12 HEAD >/tmp/sentei-vhs-progress-arc/bin/sentei-after.sha
vhs validate .demos/progress-arc/*.tape
vhs .demos/progress-arc/removal-success.tape
```

Each tape first changes to `/`, runs `setup-fixture.sh`, chains setup with
`&&`, then changes to the fixture repository with another fail-closed `&&`
before launching Sentei. A missing fixture can therefore never fall back to
the caller's working directory. Setup recreates
only named children of the exact fixture root. It rejects symlinked, non-owned,
or canonically mismatched fixture paths, including symlinks anywhere beneath
retained output trees, before resetting anything. It isolates HOME/XDG and Git
identity and installs runtime shims. The Git shim denies
network-capable verbs and
rejects absolute worktree paths outside the fixture. Network clients, package
managers, and `gh` fail closed.

The tapes set the PTY to 80 columns by 24 rows and use `SENTEI_MOTION=off`.
Their 960×672 pixel canvas leaves enough physical room for all 24 rows at the
recording font and line height.

After rendering, verify GIF metadata with `ffprobe`. Decode from frame zero
before selecting representative active, failure, and final frames; seeking
before the GIF input can inspect an incomplete delta frame. For example:

```bash
ffmpeg -i /tmp/sentei-vhs-progress-arc/outputs/removal-success.gif \
  -ss 9 -frames:v 1 /tmp/sentei-vhs-progress-arc/frames/removal-final.png
```

Inspect the resulting PNGs before accepting the recordings.
