# Release Process

Releases are published by the GitHub Actions workflow (`.github/workflows/ci.yml`) whenever a semantic version tag (`vX.Y.Z`) is pushed. [GoReleaser](https://goreleaser.com/) builds the binaries and creates a GitHub Release with the archives and checksums attached.

## Version consistency

The version the plugin reports to Vault (`RunningVersion`, visible in `vault plugin list` and `sys/plugins/catalog`) is not stored in the source. It is injected at build time from the git tag via `-ldflags -X .../plugin.Version=...`:

- Release builds (GoReleaser) use the tag being released, so the binary and the GitHub Release always carry the same version.
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

4. The tag workflow runs `test` and then `release`. The release, with its archives and checksums, appears under **Releases** on the GitHub repository.

Only tags matching `vX.Y.Z` trigger the workflow. Pre-release tags such as `v0.3.0-rc1` do not run it.

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

On pull requests, the `commitlint` job validates every commit against `.config/.commitlintrc.json` (types `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`; header up to 100 characters; no trailing period). If you squash-merge, make sure the squash commit message follows the same format.

## Workflow requirements

- The `release` job authenticates with the built-in `GITHUB_TOKEN` (granted `contents: write`), so it needs no extra secrets.
- The job fails if the working tree is dirty or if the tag is not reachable in the clone. The full-history checkout (`fetch-depth: 0`) takes care of the latter.

## Build configuration

`.config/.goreleaser.yaml` defines the build target (`linux/arm64` only, the platform the Vault nodes run on) and generates SHA256 checksums. Add targets there if another platform is needed; each one compiles the large Datadog client package again, at roughly 5 GB peak memory.

To test the release build locally without publishing:

```bash
goreleaser release --snapshot --clean --config .config/.goreleaser.yaml
```
