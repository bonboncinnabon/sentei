## Purpose
Covers the README.md content requirements including project description, installation instructions, usage documentation, and status indicator legend.

## Requirements

### Requirement: README exists at repository root
A `README.md` file SHALL exist at the repository root and serve as the primary user-facing documentation.

#### Scenario: User visits the repository
- **WHEN** a user views the repository on GitHub or locally
- **THEN** they see a README.md with a project description, installation instructions, and usage guide

### Requirement: README contains installation instructions
The README SHALL document at least two installation methods: `go install` and building from source.

#### Scenario: User installs via go install
- **WHEN** user reads the installation section
- **THEN** they find a `go install github.com/bonboncinnabon/sentei@latest` command

#### Scenario: User wants to build from source
- **WHEN** user reads the installation section
- **THEN** they find clone + `go build` instructions including the ldflags for version injection

### Requirement: README contains usage documentation
The README SHALL document CLI usage including all flags (`--version`, `--dry-run`, `--playground`) and key bindings for the TUI.

#### Scenario: User looks up key bindings
- **WHEN** user reads the usage section
- **THEN** they find a table of keyboard shortcuts (navigation, selection, sorting, filtering, quit)

#### Scenario: User looks up CLI flags
- **WHEN** user reads the usage section
- **THEN** they find a description of each flag and example invocations

### Requirement: README documents status indicators
The README SHALL explain the ASCII status indicators used in the TUI: `[ok]`, `[~]`, `[!]`, `[L]`, `[P]`.

#### Scenario: User sees unfamiliar status indicator
- **WHEN** user reads the status indicators section
- **THEN** they find a legend mapping each indicator to its meaning
