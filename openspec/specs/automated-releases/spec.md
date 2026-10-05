## Purpose
Covers automated release pipeline including release-please for versioning, GoReleaser for cross-compilation and distribution, and Homebrew tap publishing.

## Requirements

### Requirement: release-please creates Release PRs from conventional commits
A GitHub Actions workflow SHALL run on pushes to `main` and invoke release-please. release-please SHALL analyze conventional commits since the last release and create or update a Release PR that includes a version bump and CHANGELOG.md updates. The release type SHALL be `go`. The manifest file (`.release-please-manifest.json`) with initial version `0.1.0` serves as the bootstrap — no manual tag is needed.

#### Scenario: feat commit triggers minor bump PR
- **WHEN** a `feat:` commit is pushed to `main`
- **THEN** release-please creates or updates a Release PR with a minor version bump

#### Scenario: fix commit triggers patch bump PR
- **WHEN** a `fix:` commit is pushed to `main`
- **THEN** release-please creates or updates a Release PR with a patch version bump

#### Scenario: Breaking change triggers major bump PR
- **WHEN** a commit with `feat!:` or `BREAKING CHANGE:` footer is pushed to `main`
- **THEN** release-please creates or updates a Release PR with a major version bump

#### Scenario: Non-release commits do not create a Release PR
- **WHEN** only `chore:`, `docs:`, or `test:` commits are pushed to `main`
- **THEN** no Release PR is created or updated

### Requirement: Merging Release PR creates a git tag and GitHub Release
When a release-please Release PR is merged, release-please SHALL create an annotated git tag (format `vMAJOR.MINOR.PATCH`) and a GitHub Release with generated release notes.

#### Scenario: Release PR merged
- **WHEN** a release-please Release PR is merged to `main`
- **THEN** a git tag `v<version>` is created and a GitHub Release is published

### Requirement: GoReleaser runs on tag push
A GitHub Actions workflow SHALL trigger on tags matching `v*`. It SHALL run GoReleaser to cross-compile the binary for linux, darwin, and windows on amd64 and arm64 (excluding windows/arm64). The build SHALL use CGO_ENABLED=0 and inject version, commit SHA, and commit date via ldflags.

#### Scenario: Tag push triggers GoReleaser
- **WHEN** a tag matching `v*` is pushed
- **THEN** GoReleaser cross-compiles binaries and uploads archives to the GitHub Release

#### Scenario: Archives use platform-appropriate formats
- **WHEN** GoReleaser creates archives
- **THEN** linux and darwin archives use `.tar.gz` format and windows archives use `.zip` format

### Requirement: GoReleaser generates and signs checksums
GoReleaser SHALL generate a `checksums.txt` file containing SHA256 hashes of all archives. It SHALL sign the checksums file using cosign keyless signing (GitHub Actions OIDC), producing a `.sigstore.json` bundle file.

#### Scenario: Checksums file is generated
- **WHEN** GoReleaser completes a release
- **THEN** a `checksums.txt` file is included in the GitHub Release assets

#### Scenario: Checksums are signed with cosign
- **WHEN** GoReleaser completes a release
- **THEN** a `checksums.txt.sigstore.json` bundle is included in the GitHub Release assets

### Requirement: GoReleaser pushes Homebrew cask to tap
GoReleaser SHALL generate a Homebrew cask and push it to the `bonboncinnabon/homebrew-tap` repository. The cask SHALL include the project homepage and description.

#### Scenario: Homebrew cask is updated on release
- **WHEN** GoReleaser completes a release
- **THEN** the Homebrew cask in `bonboncinnabon/homebrew-tap` is created or updated

#### Scenario: User installs via Homebrew
- **WHEN** a user runs `brew tap bonboncinnabon/tap && brew install sentei`
- **THEN** the latest release binary is installed

### Requirement: release-please configuration files exist
The project SHALL include `release-please-config.json` with release-type `go` and `.release-please-manifest.json` tracking the current version. The initial manifest version SHALL be `0.1.0`. The manifest serves as the version baseline — no initial git tag is required.

#### Scenario: Config files are present
- **WHEN** the release-please action runs
- **THEN** it reads `release-please-config.json` and `.release-please-manifest.json` from the repo root

### Requirement: GoReleaser configuration file exists
The project SHALL include a `.goreleaser.yaml` at the repo root defining builds, archives, checksum, signing, homebrew, and changelog configuration.

#### Scenario: GoReleaser config is valid
- **WHEN** `goreleaser check` is run
- **THEN** the configuration passes validation
