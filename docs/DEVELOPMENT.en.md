<!-- English translation of DEVELOPMENT.md. The Chinese DEVELOPMENT.md is the primary copy and is
     maintained by the documentation toolchain ("gsd-doc-writer"); keep this translation in
     sync whenever the primary copy changes. Not generated — do not add a gsd-doc-writer marker. -->

# Development Guide

**English** | [简体中文](DEVELOPMENT.md)

The local development environment, build flow, and collaboration conventions for wesh contributors. For first-time setup (install and run) see [GETTING-STARTED.md](GETTING-STARTED.en.md); for system structure see [ARCHITECTURE.md](ARCHITECTURE.en.md); for testing details see [TESTING.md](TESTING.en.md).

## Local environment setup

**Prerequisites**

| Tool | Version | Notes |
|------|------|------|
| Go | >= 1.26.3 | Defer to `go.mod` (CI pins the version via `go-version-file: go.mod`) |
| Node.js | 24 | CI pins node 24; frontend unit tests rely on Node 24's built-in type stripping to run `.ts` directly |
| pnpm | 11.21.0 | CI-pinned (lockfileVersion 9.0, kept consistent with local) |

**Clone and install**

```sh
git clone https://github.com/sworda/wesh.git
cd wesh
pnpm -C web install
```

No environment variables or config files are needed — a local bare run of `wesh --bind 127.0.0.1 -- bash` is not subject to startup validation and is the shortest verification path during development (loopback traffic never leaves the machine).

**Build order (hard dependency)**

The frontend build must precede `go build` — `//go:embed all:dist` in `web/embed.go` requires `web/dist/` to exist at compile time:

```sh
pnpm -C web build && go build -o wesh ./cmd/wesh
```

Two key facts:

- The repo commits the `web/dist/index.html` build artifact (the real terminal page, a full vite single-file build) — after a bare clone you can `go build` / `go test ./...` and run directly without running the frontend build;
- **After modifying frontend source under `web/src/`, you must re-run `pnpm -C web build` before `go build`**, otherwise the binary still embeds the old artifact (the `.gz` pre-compressed artifact is produced by the build and not committed; `.gitignore` ignores `web/dist/*.gz`).

Package layout: `web/`, `web/uat/`, `web/uat/pw/`, and `scripts/` are four **mutually independent** pnpm packages (each with its own `package.json` and lockfile) — the app dependency tree, the jsdom/headless UAT dependency tree, the Playwright browser live-test layer, and the docs validation tooling (the jsdom + mermaid required by `check-mermaid.mjs`, `pnpm -C scripts install`) are deliberately isolated; `web/pnpm-workspace.yaml` only carries overrides (`js-base64` pinned to 3.9.2) and is not a workspace definition.

## Build and common commands

| Command | Description |
|------|------|
| `pnpm -C web install` | Install frontend dependencies (run after changing `web/package.json`) |
| `pnpm -C web build` | Frontend build: `tsc` type check + `vite build` single-file bundling + `gzip -k -9` pre-compression of `dist/index.html` |
| `pnpm -C web dev` | Vite dev server (standalone frontend debugging) |
| `go build -o wesh ./cmd/wesh` | Build the server binary (must come after the frontend build) |
| `go vet ./...` | Go static check (CI gate) |
| `go test -race -count=1 ./...` | Full test run (same footing as CI; **`-race` requires CGO — do not set `CGO_ENABLED=0` in the test environment**) |
| `node --test web/src/lib/*.test.ts` | Frontend unit tests (Node 24's built-in type stripping runs `.ts` directly, zero test-framework dependencies) |
| `node web/uat/phaseNN.mjs [binary path]` | protocol layer UAT (Node >= 22 native WebSocket/fetch zero-dependency script, spawns the real binary and asserts; default path `/tmp/wesh-uat/wesh`) |
| `node web/uat/run-all.mjs [binary path] [filter subset...]` | UAT one-shot matrix runner (17 items = 12 protocol + 4 jsdom + 1 herdr scenario): serial spawn with an aggregated summary table + a per-script 10min timeout guardrail (on timeout, SIGTERM the process group, convert to FAIL, and do not block what follows); the filter subset is space-separated, and when using the default binary pass `''` in the first position as a placeholder — an unknown name exits 2 to prevent a typo from silently running nothing; exit codes are 0 when all green / 1 on any failure / 2 on a usage error; a missing binary triggers an automatic `go build`; the pw layer is not included in the matrix (hard constraint of the dual-machine topology) |
| `node scripts/check-mermaid.mjs [file.md ...]` | mermaid lexical-validation vehicle (jsdom injection + Node-side `mermaid.parse`; mmdc/puppeteer forbidden — the render-legality equivalent under the Linux headless constraint): with no arguments it scans all `.md` under `docs/` by default, and any failing block exits 1; the first run requires `pnpm -C scripts install` |
| `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o wesh ./cmd/wesh` | Purely static build (for scenarios such as a Docker scratch image; `CGO_ENABLED=0` belongs only to release builds — do not set this variable for day-to-day testing) |
| `./scripts/release.sh --dry-run v1.0.0` | Release dry run: performs only the four pre-flight validation gates (tag shape / non-existence / clean working tree / in sync with remote) and prints the step checklist without executing |
| `./scripts/release.sh vX.Y.Z` | Real release: pre-flight validation → full tests → frontend build → long fuzz ×2 (10 minutes per target) → load matrix → confirmation gate → tag push |

There are three fuzz targets: `FuzzDecodeHello` (internal/proto), `FuzzDecodeResize` (internal/proto), and `FuzzDecodeFileConfig` (cmd/wesh) — of these, Hello and FileConfig take part in CI's 60s short run and the release script's 10-minute long run (the two commands below), while `FuzzDecodeResize` only does seed regression as part of the regular `go test`; the crash corpus is automatically dropped into the corresponding package's `testdata/fuzz/`:

```sh
go test -fuzz=FuzzDecodeHello -fuzztime=60s ./internal/proto/
go test -fuzz=FuzzDecodeFileConfig -fuzztime=60s ./cmd/wesh/
```

The browser live-test layer (Playwright driving a real Chromium, covering look-and-feel behavior such as panel copy / reconnect countdown / clear-screen repaint) lives in its own package under `web/uat/pw/`; for the run model see that directory's README.md — it requires a machine with a GUI and is not part of the regular development loop.

## Code style

There is no standalone lint/format toolchain; the discipline is carried by stdlib and CI gates:

- **Go**: `gofmt` standard formatting (the whole repo is already formatted; before committing, `gofmt -l ./cmd ./internal ./web` should print nothing) + `go vet ./...` (enforced by CI). golangci-lint is not configured. The close-out gate must use the toolchain's own `$(go env GOROOT)/bin/gofmt` (go1.26.3, which recognizes comment-block reflow under Go 1.19+ modern doc-comment rules) — an older system gofmt on PATH (such as `/usr/bin/gofmt` on this machine, actually go1.15.3) does not recognize the new rules and will miss violations.
- **TypeScript**: `web/tsconfig.json` strict mode (`strict`, `noUnusedLocals`, `noUnusedParameters`, `noFallthroughCasesInSwitch`, `verbatimModuleSyntax`); type checking runs as the `tsc` step of `pnpm -C web build` (enforced by the CI web job). ESLint/Prettier/Biome are not configured.
- **Test files**: `web/src/lib/*.test.ts` is excluded by tsconfig, does not take part in `tsc`, and is executed only via `node --test` — relative imports must carry the `.ts` extension.
- **Comment language**: existing codebase comments are in Chinese, including registrations of decision rationale; new code stays consistent.
- **Dependency discipline**: zero new dependencies on the Go side — `go.mod` keeps its 5 external dependencies (`coder/websocket`, `creack/pty`, `golang.org/x/sys`, `golang.org/x/time`, `go-toml/v2`) unchanged; the UAT protocol layer and the matrix runner likewise take the zero-dependency form built on Node's native capabilities.

## Test mechanics conventions (Go side)

The multi-mode tests in internal/server follow a three-way classification mechanic (for the test-layering overview see [TESTING.md](TESTING.en.md); what follows is the wiring discipline for changing server-side tests):

- **Single wiring point**: server wiring always goes through the four-variant `newTestServer` family in `internal/server/harness_test.go` — the two-value ordinary form `newTestServer(t, mode, argv, mutate)` (the vast majority of call sites), the three-value `newTrackedTestServer` (+`waitHandlers`, for the stderr-capture kind of synchronization edge), the four-value `newHandleTestServer` (+`waitHandlers`+`srv`, for direct Shutdown calls / white-box surfaces), and the three-value `newSessTestServer` (+`sessions` accessor, for PTY read-back surfaces). Every variant passes both the `mode=shared`/`mode=per-client` branches straight through to the existing parent helpers (no rewriting of wiring behavior; Cleanup discipline preserved), and an unknown mode always `t.Fatalf` fail-fast.
- **t.Run double-run**: tests achieve dual-mode coverage naturally via `t.Run("mode=shared")` / `t.Run("mode=per-client")` subtests (carried with zero changes by the single CI `go test` step); the current scale is 216 `mode=` subtests.
- **Three-way classification**: ① mode-agnostic — the same assertion runs directly under both modes; ② mode-mapped — assertion divergence points are explicitly tabulated (precedent: the dual-mode divergence table in `exit_test.go`: the shared column's expected values are verbatim identical to v1.0 semantics and serve as the zero-regression evidence itself, while the per-client column is listed separately); ③ mode-exclusive — explicitly asserting that a capability is not wired in the other mode, rather than leaving it untested by default.

When adding tests that cover dual-mode behavior: reuse the family entry points for wiring rather than adding scattered new helpers; classify assertions explicitly per the three-way scheme, and do not write implicit `if mode` branches.

## Branch conventions

- The main branch is `main`; the version history and git tags share one source (`v*` tags trigger releases, starting at v1.0.0).
- There is no written branch-naming convention. Current practice in the repo: feature development branches by topic (e.g. `phase08-observability`, `phase-07`), and fixes use the `fix/*` prefix (e.g. `fix/scan-and-macos-ci`).

## PR process

The repo provides no PR template; contributions are gated on all-green CI (`.github/workflows/ci.yml`, triggered on both push and PR):

1. **Commit messages** follow the Conventional Commits form `type(scope): subject` — current types: `feat` / `fix` / `docs` / `test` / `chore` / `ci` / `style` (commits with a `docs:`, `test:`, `chore:`, `ci:`, or `style:` prefix are excluded from the release changelog).
2. **All three CI jobs must be green**:
   - `go` (ubuntu + macos dual-platform matrix): `go vet ./...` + `go test -race -count=1 -v ./...` — the macos leg also carries the runtime verification of kqueue child-process reaping; do not substitute "it passed on my Linux machine" for it;
   - `web` (ubuntu): `pnpm -C web install --frozen-lockfile` + `pnpm -C web build`;
   - `fuzz` (ubuntu): 60s per target, two targets.
3. **Pre-commit self-check**: `$(go env GOROOT)/bin/gofmt -l ./cmd ./internal ./web` produces no output (use the GOROOT version, see "Code style"), `go vet` has zero warnings, `tsc` strict mode reports zero errors, and `pnpm -C web build` has been re-run after any change to `web/src/`.
4. Commits should be as atomic as possible (one commit, one topic), with the scope being a package name or topic domain (at the current granularity, e.g. `fix(09): ...`, `test(server): ...`).

The release process (`scripts/release.sh` integrated in a single script: validation → tests → build → fuzz → load matrix → tag push, with goreleaser taking over the four-platform artifacts after the tag push) is a maintainer operation; contributors need not get involved. For details see [README.md](../README.en.md) and the script's header comments. The version triple injection point is `var version/commit/builtAt` in `cmd/wesh/main.go` — the goreleaser release build injects the tag version, the full commit hash, and the commit timestamp via `-X` (Unix seconds; `--version` displays it in the timezone of the machine it runs on; the timestamp shares a source with goreleaser's `mod_timestamp`, preserving reproducible builds); a local `go build` always shows the version number `dev`, with commit/time automatically backfilled from the VCS build info embedded by the Go toolchain (a build outside the repo shows `none`/`unknown`).

For the collaboration conventions overview see [CONTRIBUTING.md](../CONTRIBUTING.en.md).
