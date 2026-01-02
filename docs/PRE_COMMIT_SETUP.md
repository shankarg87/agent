# Pre-commit Hooks Setup

This project uses [pre-commit](https://pre-commit.com/) to ensure code quality and consistency before commits are made to the repository.

## What is pre-commit?

Pre-commit is a framework for managing and maintaining multi-language pre-commit hooks. It automatically runs a series of checks and fixes on your code before you commit changes.

## Installation

### Prerequisites

Make sure you have the following installed:
- Python (for pre-commit)
- Go (for the Go-specific hooks)
- Make (for running Makefile targets)

### Setup

1. Install pre-commit hooks:
   ```bash
   make install-hooks
   ```

   This will:
   - Install pre-commit if not already installed
   - Install the git hooks in your local repository

## What hooks are enabled?

### Standard hooks
- **trailing-whitespace**: Removes trailing whitespace
- **end-of-file-fixer**: Ensures files end with a newline
- **check-yaml**: Validates YAML files
- **check-added-large-files**: Prevents large files from being committed
- **check-case-conflict**: Prevents case conflicts on case-insensitive filesystems
- **check-merge-conflict**: Prevents committing merge conflict markers
- **debug-statements**: Detects debug statements
- **check-json**: Validates JSON files

### Go-specific hooks
- **go-fmt**: Formats Go code using `gofmt`
- **go-vet-mod**: Runs `go vet` with module support
- **go-imports**: Organizes Go imports
- **go-cyclo**: Checks cyclomatic complexity (max 15)
- **go-mod-tidy**: Ensures `go.mod` and `go.sum` are tidy
- **go-unit-tests**: Runs unit tests
- **golangci-lint**: Comprehensive Go linting

### Local hooks (using Makefile)
- **go-build**: Ensures the project builds successfully (`make build`)
- **go-test-unit**: Runs unit tests (`make test-unit`)
- **go-lint-local**: Runs local linter (`make lint`)
- **go-mod-check**: Verifies `go mod tidy` doesn't change anything

### Security hooks
- **detect-secrets**: Scans for secrets and sensitive information

## Usage

### Automatic execution
Once installed, the hooks will run automatically every time you make a commit:
```bash
git commit -m "Your commit message"
```

If any hook fails, the commit will be rejected and you'll need to fix the issues before committing again.

### Manual execution

Run all hooks on staged files:
```bash
make pre-commit
```

Run all hooks on all files:
```bash
make pre-commit-all
```

Run a comprehensive CI check (what CI would run):
```bash
make ci-check
```

### Skipping hooks

If you need to skip hooks for a specific commit (not recommended for production):
```bash
git commit --no-verify -m "Your commit message"
```

## Makefile targets

The following new targets have been added:

- `make install-hooks`: Install pre-commit hooks
- `make pre-commit`: Run pre-commit checks on staged files
- `make pre-commit-all`: Run pre-commit checks on all files
- `make security-scan`: Run security scan using gosec
- `make ci-check`: Run all CI checks (deps, fmt, lint, test-unit, build, security-scan)

## Troubleshooting

### Hook failures

If a hook fails:

1. **Read the error message** - it will tell you what went wrong
2. **Fix the issue** - most formatting issues can be auto-fixed by running the hook again
3. **Stage your changes** - `git add .`
4. **Try committing again** - `git commit -m "Your message"`

### Common issues

**Go formatting issues**: Run `make fmt` to fix automatically.

**Linting issues**: Run `make lint` to see specific issues, then fix them manually.

**Test failures**: Run `make test-unit` to see which tests are failing.

**Build failures**: Run `make build` to see compilation errors.

**Secrets detected**: Review the detected secrets and either remove them or update `.secrets.baseline` if they're false positives.

### Updating hooks

To update to the latest versions of all hooks:
```bash
pre-commit autoupdate
```

### Debugging hooks

To run a specific hook:
```bash
pre-commit run <hook-id>
```

For example:
```bash
pre-commit run go-fmt
pre-commit run go-build
```

## Configuration

The pre-commit configuration is stored in `.pre-commit-config.yaml`. You can modify this file to:
- Add new hooks
- Change hook arguments
- Exclude certain files
- Configure hook behavior

For more information, see the [pre-commit documentation](https://pre-commit.com/).
