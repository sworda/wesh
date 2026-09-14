<!-- English translation of CONTRIBUTING.md. The Chinese CONTRIBUTING.md is the primary copy and is
     maintained by the documentation toolchain ("gsd-doc-writer"); keep this translation in
     sync whenever the primary copy changes. Not generated — do not add a gsd-doc-writer marker. -->

# Contributing

**English** | [简体中文](CONTRIBUTING.md)

Thanks for your interest in wesh! This document covers the development conventions, the CI gate, and the collaboration workflow. Before you start, please read:

- [docs/GETTING-STARTED.md](docs/GETTING-STARTED.en.md) —— prerequisites, installation, and first run
- [docs/DEVELOPMENT.md](docs/DEVELOPMENT.en.md) —— local development environment and build workflow
- [docs/TESTING.md](docs/TESTING.en.md) —— test layering and how to run it

Repository: [github.com/sworda/wesh](https://github.com/sworda/wesh).

## Platform boundary

wesh supports **linux/darwin (amd64/arm64)** only; Windows is out of scope —— `internal/pty` is constrained by the `//go:build linux` / `//go:build darwin` build tags, so the server cannot be built or run on Windows. Develop and verify backend changes on Linux or macOS; frontend-only (`web/`) and documentation changes are unaffected.

## Development environment

| Tool | Version | Notes |
|------|---------|-------|
| Go | >= 1.26.3 | `go.mod` is authoritative, and CI pins to it |
| Node.js | 24 | pinned by CI |
| pnpm | 11.21.0 | pinned by CI (`web/package.json` has no `packageManager` field, so align explicitly); always use pnpm, never npm, on the web side, to avoid lockfile drift |

The key build discipline —— **the frontend build must precede `go build`**:

```sh
pnpm -C web install && pnpm -C web build && go build -o wesh ./cmd/wesh
```

`//go:embed all:dist` in `web/embed.go` requires `web/dist/` to exist at compile time. The repository commits the `web/dist/index.html` build artifact (a real terminal page), so a bare clone can compile and run tests directly; but after modifying frontend sources under `web/` you must re-run `pnpm -C web build` before `go build`, otherwise the binary still embeds the stale artifact. See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.en.md) for details.

## Code style

- **Go**: standard `gofmt` formatting, but **use the gofmt from GOROOT (go1.26.3)** —— an older gofmt on `PATH` (the version shipped by some systems) misses CJK comment continuation lines, and this repository's comments are largely Chinese; check with `$(go env GOROOT)/bin/gofmt -l .`. CI gates on `go vet ./...` plus a full `-race` test run, with no additional lint toolchain.
- **Zero-new-dependency discipline**: `go.mod` carries only five external dependencies (`coder/websocket`, `creack/pty`, `golang.org/x/sys`, `golang.org/x/time`, `go-toml/v2`). Adding a dependency requires solid justification —— do not pull in a third party when the standard library suffices, and explain the purpose and the reason for rejecting the standard library or a hand-rolled implementation in the PR description.
- **Frontend**: TypeScript type checking is embedded in the build (`pnpm -C web build` is a single `tsc && vite build && gzip` script) and enforced by the CI web job; there is no separate ESLint/Prettier configuration.
- **Commit messages**: Conventional Commits form `type(scope): subject`. The types actually in use are `feat` / `fix` / `docs` / `test` / `chore` / `style`. Note that the release changelog automatically drops commits prefixed `docs:` / `test:` / `chore:` / `ci:` / `style:` (see `.goreleaser.yml`) —— only substantive changes such as `feat` / `fix` reach the release notes.

## PR guidelines

- **Target branch is `main`**; branch naming has no enforced convention, but the custom is short-lived topic branches (e.g. `fix/xxx`).
- **All three CI jobs green** is the precondition for merging (triggered on both `push` and `pull_request`):

| Job | Runner | Contents |
|-----|--------|----------|
| `go` | ubuntu + macos matrix | `go vet ./...` + `go test -race -count=1 -v ./...` |
| `web` | ubuntu | `pnpm -C web install --frozen-lockfile` + `pnpm -C web build` (type check + build + precompression) |
| `fuzz` | ubuntu | 60s short regression runs of `FuzzDecodeHello` (`./internal/proto/`) and `FuzzDecodeFileConfig` (`./cmd/wesh/`) |

- **Self-check locally before committing**, matching the CI recipe: `go vet ./...`, `go test -race -count=1 ./...`, `pnpm -C web build`; when your change touches a document containing mermaid diagrams, also run `node scripts/check-mermaid.mjs` (lexical validation; with no arguments it scans every `.md` under `docs/`, or pass a file list for a targeted check; the exit code is 1 if any block FAILs).
- **Artifact discipline for frontend changes**: rebuild after changing `web/`, and if the rebuild rewrites the tracked `web/dist/index.html`, the new artifact must be committed together with the sources —— the release gate verifies that the committed dist matches the build artifact and refuses to release on a mismatch (the dist drift gate in `scripts/release.sh`).
- **Changes touching PTY/signals/process reaping** need verification on both Linux and macOS (the CI matrix already covers this; the macOS leg also carries verification of kqueue runtime behavior).
- **Test layering**: run the layer you changed —— Go logic uses in-package unit tests; the protocol layer uses the zero-dependency `web/uat/phaseNN.mjs` scripts (which assert against a spawned real binary); the browser look-and-feel side uses the `web/uat/pw/` Playwright suite (two-machine model, see its README). See [docs/TESTING.md](docs/TESTING.en.md) for the full layering strategy. Fuzz crash corpora land automatically in the corresponding package's `testdata/fuzz/`, and after a fix they regress with the regular test run.
- **Dual-mode test discipline for internal/server**: that package's tests run twice through the `newTestServer` family of wiring points (`newTestServer` / `newTrackedTestServer` / `newHandleTestServer` / `newSessTestServer` in `harness_test.go`) as `t.Run("mode=shared"/"mode=per-client")` (216 `mode=` subtests at present), both columns executing the same assertion body. **The shared column's expected values remaining verbatim unchanged is the evidence of zero regression itself** —— a PR that changes shared behavior must list, item by item in its description, each modified shared expected value and its justification. New tests go through the same family of wiring points too (an unknown mode always hits `t.Fatalf`); do not bypass the family to build a single-mode harness.

## Reporting issues

The repository ships no issue template; when reporting, please include:

1. **Reproduction steps**: the full startup command (including flags) and the sequence of actions
2. **Expected behavior vs. actual behavior**
3. **Environment**: OS and architecture (only linux/darwin are supported), the output of `wesh --version`, and browser type and version for frontend issues

Security reminder: never paste credentials or the token from a ro/rw share link into an issue —— tokens are regenerated at every startup, so redact before reporting.

## Release process (maintainers)

Releases are performed by maintainers; contributors need do nothing. Knowing the trigger chain helps make sense of the commit-message convention above:

1. `scripts/release.sh vX.Y.Z [--dry-run]`: four precondition gates (tag shape / not a duplicate / clean working tree / in sync with the remote) → full test run (same recipe as CI) → frontend build + dist drift gate → a 10-minute long fuzz for each of the two targets → load matrix (`-tags=load`, 30-minute cap) → manual confirmation → create and push the tag.
2. A `v*` tag push triggers `.github/workflows/release.yml`: CI rebuilds the frontend and goreleaser produces four-platform `tar.gz` archives (linux/darwin × amd64/arm64) plus `checksums.txt`.

## Documentation

The authoritative documents are [README.md](README.en.md) and the `docs/` directory (GETTING-STARTED / ARCHITECTURE / CONFIGURATION / DEPLOYMENT / DEVELOPMENT / TESTING). `docs/` and this file are generated by a toolchain and regenerated wholesale —— **content edited by hand may be overwritten on the next generation**, so file an issue or point it out in the PR description if you spot a problem. `README.md` was deliberately moved out of the auto-generated scope: it carries no `<!-- generated-by: gsd-doc-writer -->` marker and is a hand-maintained file, which the generation flow preserves by default when it meets an unmarked file (or only appends missing sections), so **edit README directly when changing it**. Code comments and notes inside `web/uat/` scripts are outside the auto-generated scope.

Every document also ships an English translation alongside it as `<name>.en.md`, with a language switcher at the top linking the pair (for example [README.en.md](README.en.md) and [docs/ARCHITECTURE.en.md](docs/ARCHITECTURE.en.md)). The primary copy is **Chinese**, at the same path without the suffix, and it is the only language the generation flow maintains; these `.en.md` files are hand-maintained and deliberately carry no `<!-- generated-by: gsd-doc-writer -->` marker, since that marker asserts generator ownership. When the Chinese primary changes, update its `.en.md` translation in the same PR so the pair does not drift apart.

Among these, the README / CONFIGURATION / ARCHITECTURE trio describes the public contract from the same source: **when changing a public contract such as a flag / environment variable / TOML key, you must update both sources —— `cmd/wesh/main.go` (flag definitions) and `cmd/wesh/config.go` (TOML structs) —— and update all three documents in the same PR**, so the documentation does not drift from the implementation.

## License

[MIT](LICENSE) © 2026 sworda. Submitting a PR means you agree that your contribution is released under the MIT license.
