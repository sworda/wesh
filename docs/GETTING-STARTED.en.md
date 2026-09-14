<!-- English translation of GETTING-STARTED.md. The Chinese GETTING-STARTED.md is the primary copy and is
     maintained by the documentation toolchain ("gsd-doc-writer"); keep this translation in
     sync whenever the primary copy changes. Not generated — do not add a gsd-doc-writer marker. -->

# Getting started

**English** | [简体中文](GETTING-STARTED.md)

From zero to seeing your own terminal in a browser in three steps: install the dependencies → build → launch on loopback. No admin privileges are needed at any point, and there are no extra system library dependencies (a pure static Go build, with the frontend page already embedded in the binary).

## Prerequisites

| Dependency | Version requirement | Purpose |
|------|----------|------|
| Operating system | Linux or macOS | wesh depends on the PTY subsystem and only implements the linux/darwin platforms (the `internal/pty` platform files are `*_linux.go`/`*_darwin.go`); Windows is not supported |
| Git | any recent version | Clone the repository |
| Go | >= 1.26.3 (see `go.mod`) | Build and run the server |
| Node.js + pnpm | CI-pinned Node 24 / pnpm 11.21.0 (reference values) | **Only needed when modifying the `web/` frontend**; backend development can skip it |

## Installation

```sh
git clone https://github.com/sworda/wesh.git
cd wesh
pnpm -C web install   # frontend dependencies, only needed when changing the frontend
```

The repository commits the `web/dist/index.html` build artifact (the real terminal page, satisfying the compile-time requirement of `go:embed`), so a bare clone can run `go build` / `go test ./...` directly without pnpm installed and still get a fully functional terminal; `pnpm -C web install && pnpm -C web build` is only needed when modifying the `web/` frontend source.

## First run

1. **Build**:

   ```sh
   pnpm -C web build && go build -o wesh ./cmd/wesh
   ```

   After modifying the `web/` frontend source you must run `pnpm -C web build` before `go build`, otherwise the binary still embeds the old snapshot (a bare clone does not need this step, since dist already contains a real build artifact).

2. **Start** (loopback, no credential required):

   ```sh
   ./wesh --bind 127.0.0.1 -- bash
   ```

   > With the default `--bind 0.0.0.0`, startup is refused when no credential is configured (secure default — see FAQ item 1 below). `--bind 127.0.0.1` keeps traffic on the machine, so it works without a credential.

3. **Open a browser** and visit the address printed by `listening on` (default port 7681); you will see a complete interactive terminal running `bash`.

Output after startup (stdout; read-only by default, the terminal title carries the `[ro] ` prefix, and browser keyboard input having no effect is expected behavior):

```
listening on http://127.0.0.1:7681
share read-only:  http://127.0.0.1:7681/s/<ro-token>/
```

`Ctrl+C` to shut down: SIGTERM/SIGINT triggers graceful shutdown, and the child process terminates with the process group. Runtime events are written to stderr as single-line JSON, while the startup line and the share link line stay human-readable text on stdout.

## Session modes (shared / per-client)

As of v1.1, `--session-mode=shared|per-client` selects the session mode; the default is `shared` — the first-run example above is shared mode. One minimal example for each mode:

**shared (default) — multiple people share the same terminal on one screen**:

```sh
./wesh --bind 127.0.0.1 -- bash
```

All browsers that open the share link connect to the same PTY session: output is fanned out in real time ×N clients, and write permission goes through owner arbitration and promotion (the first writable client holds it exclusively, and after a disconnect promotion follows attach order).

**per-client — each browser connection gets its own shell** (ttyd-style lifecycle):

```sh
./wesh --bind 127.0.0.1 --session-mode=per-client --writable -- bash
```

Each client gets an independent PTY process: disconnect terminates it, reconnect starts a brand-new process, and terminal size passes straight through with no arbitration. ro/rw share links are no longer two tickets into the same session — they are per-process tickets by permission level (invisible to each other when the child process is a plain shell); read-only is still a server-side boundary, and keyboard input from ro clients is dropped server-side.

per-client quick notes (full semantics in [README Session modes](../README.en.md) and [CONFIGURATION.md](CONFIGURATION.en.md)):

- `--stop-timeout` defaults diverge by mode: when not set explicitly, per-client defaults to `5s` (SIGKILL as a fallback to reap SIGHUP-immune processes after the client disconnects); an explicit `0` respects user intent but warns at startup about the leak risk.
- `--write-policy` has no effect under per-client (a startup warning) — ro/rw permission levels are still bound to the ticket.
- `--max-clients` (default 32) also serves as the concurrent child process limit — client count == child process count.
- Aggregating the same session through herdr/tmux preserves the "watch the same screen together" experience: `wesh --writable --session-mode=per-client -- herdr --session <name>`.
- TOML equivalent key: `session-mode = "per-client"` (override order flag > config file > built-in default).

## FAQ

**1. `wesh -- bash` refuses to start (exit 2)**

The error looks like `refusing to listen on non-loopback address without credentials; pass --no-auth to disable authentication`. This is a deliberate secure default: with the default `--bind 0.0.0.0`, a bare run without credentials is not allowed. Pick one of the three:

```sh
./wesh --bind 127.0.0.1 -- bash          # local use (recommended)
./wesh --credential user:pass -- bash    # configure a credential to share externally
./wesh --no-auth -- bash                 # explicitly declare "I know I'm running exposed"
```

**2. You modified the `web/` frontend source but the page did not change**

`go:embed` snapshots `web/dist/` into the binary at compile time. After changing the frontend you must re-run `pnpm -C web build` before `go build`, otherwise the binary still embeds the old artifact.

**3. `go test -race` reports cgo-related errors**

The `-race` race detector requires cgo. Do not set `CGO_ENABLED=0` in the test environment (that variable belongs only to release build scenarios; the CI go test channel explicitly does not set it).

**4. Port 7681 is already in use**

`--port 0` lets the kernel assign a random port and prints the actual port at startup:

```sh
./wesh --bind 127.0.0.1 --port 0 -- bash
```

**5. Clipboard (selection copy / Ctrl+Shift+V paste) does not work**

The browser clipboard API is only available over HTTPS or on localhost. Under a plaintext HTTP deployment that is not localhost, selection copy and paste silently stop working (the rest of the terminal is unaffected). See [CONFIGURATION.md](CONFIGURATION.en.md) for details.

**6. Cannot build on Windows**

The error comes from `internal/pty` lacking a platform implementation — Windows is ultimately out of scope. Please build and run in a Linux, macOS, or WSL2 environment.

## Next steps

- [CONFIGURATION.md](CONFIGURATION.en.md) — all CLI flags, the TOML config file, and environment variables (including authentication/TLS/share link semantics)
- [DEPLOYMENT.md](DEPLOYMENT.en.md) — reverse proxy (nginx/Caddy), systemd, Docker, and UNIX socket production deployment
- [DEVELOPMENT.md](DEVELOPMENT.en.md) — local development environment, build flow, and release flow
- [TESTING.md](TESTING.en.md) — test framework, how to run it, and CI integration
- [ARCHITECTURE.md](ARCHITECTURE.en.md) — system architecture, components, and the wesh.v1 protocol
- [CONTRIBUTING.md](../CONTRIBUTING.en.md) — contribution guide
