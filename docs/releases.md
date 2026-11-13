# CI and releases

The setup follows [TlgMe](https://github.com/bettertomorrow-dev/tlgme), with
version tags triggering releases instead of a manual version calculation.

Every PR and push to `main` runs CI. Formatting, module consistency, `go vet`,
and regenerated templ/CSS checks run on Linux. Race tests and native builds
run on Linux, macOS, and Windows. Go comes from `go.mod`, with setup-go's
module/build cache. A GoReleaser snapshot also builds all six release targets.
Actions are pinned to commit SHAs and Dependabot checks for updates weekly.
PR jobs have read-only permissions and receive no release secrets.

These choices follow GitHub's [Go CI guidance](https://docs.github.com/en/actions/tutorials/build-and-test-code/go)
and [Actions security guidance](https://docs.github.com/en/actions/reference/security/secure-use).

## Publish

After merging and passing CI on `main`:

```sh
git switch main
git pull --ff-only
git tag -a v0.1.0 -m 'Twikko v0.1.0'
git push origin v0.1.0
```

Only stable `vX.Y.Z` tags on commits reachable from `main` can publish.
The Release workflow reruns the same CI on the tagged commit before publishing.
It uses `GITHUB_TOKEN`, with write permission confined to the publishing job.
Re-run a failed workflow to retry publication. Never move a released tag.

Each release contains amd64 and arm64 builds for macOS, Linux, and Windows.
Assets use `twikko_VERSION_OS_ARCH.tar.gz`, with `.zip` for Windows, and
`checksums.txt` contains SHA-256 digests. Binaries contain embedded assets,
need no Go runtime, and support `twikko --version`.
macOS binaries are currently unsigned and not notarized. The Homebrew cask
uses the same quarantine-removal hook as TlgMe.

## Homebrew

GoReleaser publishes a macOS cask to `bettertomorrow-dev/homebrew-tap`.
Create a fine-grained token restricted to that repository with
**Contents: Read and write**, and store it in Twikko's Actions secrets as
`TAP_GITHUB_TOKEN`. An organization secret shared with this repository also works.
The default `GITHUB_TOKEN` cannot write to another repository.

Without that secret, GitHub releases still publish and the workflow emits a
warning. The generated cask remains in GoReleaser's output. Configure the
secret before the first release to make Homebrew installation available.
Homebrew casks support macOS; Linux users should use the Bash installer.

## Repository rules

Require PRs and these checks on `main`: `Check`, `Test (ubuntu-latest)`,
`Test (macos-latest)`, and `Test (windows-latest)`. Block force pushes and branch
deletion. Do not require an approving reviewer while this is a solo project.
Protect `v*` tags against updates and deletion while allowing new tags.

## Local verification

```sh
go test -race ./...
go vet ./...
make tools generate
git diff --exit-code -- internal/ui
goreleaser check
goreleaser release --snapshot --clean
```

Installers verify checksums before replacing a binary. Bash installs to
`~/.local/bin`; PowerShell installs to `%LOCALAPPDATA%\twikko` and updates the
user PATH. Set `TWIKKO_INSTALL_DIR` to override the directory and
`TWIKKO_VERSION=vX.Y.Z` to pin a release. Both installers can be rerun to update.
