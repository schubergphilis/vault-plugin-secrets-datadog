# Contributing to vault-plugin-secrets-datadog

Thank you for your interest in contributing! This document provides guidelines for contributing to this project.

## Getting Started

1. Clone the repository: `git clone ssh://git@sbp.gitlab.schubergphilis.com:2228/SaaS/Azure/vault/vault-plugins/vault-plugin-secrets-datadog.git`
2. Create a feature branch: `git checkout -b feature/my-new-feature`
3. Make your changes
4. Test your changes: `go test ./...`
5. Commit using conventional commits (see below)
6. Push your branch and open a merge request

## Development Setup

### Prerequisites

- Go 1.25.0 or later
- Git
- Make (optional, for convenience)

### Building

```bash
# Build the plugin
go build -o vault/plugins/vault-plugin-secrets-datadog cmd/vault-plugin-secrets-datadog/main.go

# Or use make
make build
```

### Testing

```bash
# Run all tests
go test -v ./...

# Or use make
make test
```

## Commit Message Guidelines

**⚠️ IMPORTANT:** This project enforces [Conventional Commits](https://www.conventionalcommits.org/) format. Merge requests with non-compliant commit messages will fail the `commitlint` CI job.

### Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types

- **feat**: A new feature (triggers minor version bump)
- **fix**: A bug fix (triggers patch version bump)
- **docs**: Documentation changes only
- **style**: Code style changes (formatting, semicolons, etc.)
- **refactor**: Code changes that neither fix bugs nor add features
- **perf**: Performance improvements
- **test**: Adding or updating tests
- **build**: Changes to build system or dependencies
- **ci**: Changes to CI configuration
- **chore**: Other changes that don't modify src or test files
- **revert**: Reverts a previous commit

### Scopes (Optional)

- `deps`: Dependency updates
- `security`: Security-related changes
- `auth`: Authentication-related
- `api`: API key generation
- `roles`: Role management
- `config`: Configuration management
- `client`: Datadog client

### Examples

#### Good Commit Messages ✅

```bash
feat: add support for incident_write scope

fix(auth): handle expired credentials gracefully

docs: update README with installation instructions

fix!: remove deprecated configuration fields

feat(api): implement key rotation with zero downtime

BREAKING CHANGE: Configuration format has changed
```

#### Bad Commit Messages ❌

```bash
# Missing type
Added new feature

# Doesn't start with lowercase type
Feat: new feature

# Subject ends with period
fix: resolve bug.

# Not descriptive
fix: fix issue

# Too vague
update code
```

### Breaking Changes

For changes that break backwards compatibility:

```bash
# Option 1: Use ! after type
feat!: change API response format

# Option 2: Add footer
feat: redesign configuration structure

BREAKING CHANGE: Configuration file format has changed from YAML to JSON.
Migration guide: https://...
```

## Merge Request Process

1. **Ensure tests pass**: Run `go test ./...`
2. **Update documentation**: If you've changed APIs or behavior
3. **Use conventional commits**: MR title and commits must follow format
4. **Keep MRs focused**: One feature/fix per PR
5. **Write clear descriptions**: Explain what and why, not just how

### MR Title Format

Your MR title must follow conventional commit format (it becomes the commit message when squash merged):

```
feat: Add support for new Datadog scopes
fix(client): Handle API rate limiting properly
docs: Improve role configuration examples
```

### Automated Checks

Every MR will run:
- ✅ **Test Suite**: All unit tests must pass
- ✅ **Conventional Commits**: commit messages validated by `commitlint`
- ✅ **Build**: Code must compile successfully

## Code Style

- Follow standard Go conventions
- Run `go fmt` before committing
- Keep functions small and focused
- Add comments for complex logic
- Write tests for new features

## Security

If you discover a security vulnerability:

1. **DO NOT** open a public issue
2. Email the maintainers privately
3. Allow time for a fix before public disclosure

## License

By contributing, you agree that your contributions will be licensed under the same license as this project.

## Questions?

- Check existing issues and MRs
- Read the documentation in `RELEASE.md`
- Open a discussion for questions

## Recognition

Contributors will be recognized in release notes and the project README.

Thank you for contributing! 🎉
