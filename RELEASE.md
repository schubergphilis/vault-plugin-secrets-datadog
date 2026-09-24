# Release Process

Releases are published by the GitLab pipeline (`.gitlab-ci.yml`) whenever a semantic version tag (`vX.Y.Z`) is pushed. [GoReleaser](https://goreleaser.com/) builds the binaries, uploads them to the project's generic package registry and creates a GitLab Release.

## Version consistency

The version the plugin reports to Vault (`RunningVersion`, visible in `vault plugin list` and `sys/plugins/catalog`) is not stored in the source. It is injected at build time from the git tag via `-ldflags -X .../plugin.Version=...`:

- Release builds (GoReleaser) use the tag being released, so the binary and the GitLab Release always carry the same version.
- Local builds (`make build`) use `git describe --tags`, e.g. `v0.3.0-2-gabc1234-dirty`.
- Plain `go build` without ldflags reports `v0.0.0-dev`.

There is no version constant to bump before a release.

## Releasing

1. Make sure `main` is green and contains everything you want to release.
2. Move the `[Unreleased]` entries in `CHANGELOG.md` under a new version heading and merge that change to `main`.
3. Tag the release commit on `main` and push the tag:

   ```bash
   git checkout main && git pull
   git tag -a v0.3.0 -m "v0.3.0"
   git push origin v0.3.0
   ```

4. The tag pipeline runs `test` and then `release`. The release appears under **Deploy → Releases**, and its archives and checksums under **Deploy → Package Registry**.

Only tags matching `vX.Y.Z` trigger the `release` job. Pre-release tags such as `v0.3.0-rc1` only run the tests.

## Choosing the version

This project follows [Semantic Versioning](https://semver.org/). Use the [Conventional Commits](https://www.conventionalcommits.org/) since the last tag to decide on the version:

- `fix:`: patch (0.1.0 → 0.1.1)
- `feat:`: minor (0.1.0 → 0.2.0)
- `BREAKING CHANGE:` or `!`: major (0.1.0 → 1.0.0)

While in 0.x.x, minor versions may contain breaking changes.

```bash
# commits since the last release
git log --oneline "$(git describe --tags --abbrev=0)"..HEAD
```

## Commit message validation

On merge requests, the `commitlint` job validates every commit against `.config/.commitlintrc.json` (types `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`; header up to 100 characters; no trailing period). If you squash-merge, make sure the squash commit message follows the same format.

## Pipeline requirements

- The `release` job authenticates with `CI_JOB_TOKEN` (`use_job_token` in `.config/.goreleaser.yaml`), so it needs no extra CI/CD variables.
- The job fails if the working tree is dirty or if the tag is not reachable in the clone. `GIT_DEPTH: 0` and the `.go/` entry in `.gitignore` take care of this.

## Build configuration

`.config/.goreleaser.yaml` defines the build targets (Linux, macOS, Windows on amd64, arm64, 386, arm) and generates SHA256 checksums.

To test the release build locally without publishing:

```bash
goreleaser release --snapshot --clean --config .config/.goreleaser.yaml
```
