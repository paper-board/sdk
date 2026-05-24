# Operations

## SemVer policy

`paper-board/sdk` follows Semantic Versioning 2.0 strictly.

| Change type                                                   | Version bump |
| ------------------------------------------------------------- | ------------ |
| New exported symbol, new struct field, new subcommand         | minor        |
| Bug fix, documentation, performance with no API change        | patch        |
| Removed or renamed export, changed function signature or type | **major**    |

Pre-launch (Phase 1-4), breaking changes land as minor bumps when all consumers
are updated in the same PR wave. Starting Phase 5 (production cutover), the
major-bump rule is enforced without exception.

## Release process

Releases are automated via [release-please](https://github.com/googleapis/release-please)
triggered on every merge to `main`.

1. Merge a PR with Conventional Commit messages (`feat:`, `fix:`, `feat!:`, etc.).
2. release-please opens or updates a Release PR accumulating the changelog.
3. Merge the Release PR to create a GitHub Release + tag (e.g. `v0.5.0`).
4. Renovate detects the new tag and opens auto-PRs in each dependent service repo.

Commit message prefixes that affect the version bump:

| Prefix                                | Effect     |
| ------------------------------------- | ---------- |
| `fix:`                                | patch      |
| `feat:`                               | minor      |
| `feat!:` or `BREAKING CHANGE:` footer | major      |
| `chore:`, `docs:`, `test:`            | no release |

Do not manually push tags. release-please is the sole tag authority.

## Breaking change handling

Before introducing a breaking change:

1. Open a GitHub Issue describing the change and the migration path.
2. Implement with a `feat!:` commit. The changelog entry must include a
   `### Migration` section explaining what callers must change.
3. The Release PR description must list every affected service repo.
4. After the tag lands, Renovate auto-PRs appear in service repos. Each service
   engineer reviews, applies the migration, and merges the Renovate PR before
   the service can ship a release that depends on the new SDK version.

A breaking change to the migrator API or auth middleware triggers a cascade
across up to seven service repos. Budget accordingly.

## Consumer guidance: pinning and upgrading

### How services pin the SDK

Each service's `go.mod` pins an exact tag:

```
require github.com/paper-board/sdk v0.4.0
```

Never use a branch pseudo-version (`v0.0.0-20260520...`) in a released service
image. Pseudo-versions are acceptable only on short-lived feature branches that
will be rebased before merge.

### Upgrading via Renovate

Renovate watches `paper-board/sdk` tags and opens a PR per minor/patch bump in
each service repo. The PR includes the release notes. CI runs automatically;
merge when green.

For major bumps, Renovate opens the PR but the migration may require manual
changes. Check the changelog `### Migration` section.

### Manual upgrade

```bash
go get github.com/paper-board/sdk@v0.5.0
go mod tidy
```

Run `go test -race -count=1 ./...` in the service repo before pushing.

### Local development with an unreleased SDK change

Use a `replace` directive during local development only. Never merge a
`replace` directive to `main` — CI fetches the module via GOPRIVATE and a
replace override will break the CI fetch:

```go
// go.mod — LOCAL ONLY, remove before pushing
replace github.com/paper-board/sdk => ../sdk
```

## Dependency on GOPRIVATE

The SDK is a private module. Services and CI must have:

```bash
export GOPRIVATE=github.com/paper-board/*
```

CI workflows inherit this from `paper-board/.github/go-ci.yml` via the
`GITHUB_TOKEN` secret. Local development requires a GitHub PAT with `read:packages`
scope configured in `~/.netrc` or via `git config --global url`.

## Compatibility matrix

| SDK version | Go minimum | Phase shipped |
| ----------- | ---------- | ------------- |
| v0.1.0      | 1.22       | Phase 1.0     |
| v0.2.0      | 1.22       | Phase 1.0     |
| v0.3.0      | 1.26       | Phase 2       |
| v0.4.0      | 1.26       | Phase 3       |

Services on Phase 1-2 may remain on an older minor as long as they do not
need new packages. Patch bumps are always safe to apply.
