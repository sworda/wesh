<!-- English translation of README.md. The Chinese README.md is the primary copy and is
     maintained by the documentation toolchain ("gsd-doc-writer"); keep this translation in
     sync whenever the primary copy changes. Not generated — do not add a gsd-doc-writer marker. -->

# wesh

**English** | [简体中文](README.md)

**wesh** = **Web Shell** — share terminal over web.

[![CI](https://github.com/sworda/wesh/actions/workflows/ci.yml/badge.svg)](https://github.com/sworda/wesh/actions/workflows/ci.yml)
[![Release](https://github.com/sworda/wesh/actions/workflows/release.yml/badge.svg)](https://github.com/sworda/wesh/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/github/license/sworda/wesh)](LICENSE)

`wesh` (**We**b **Sh**ell, share terminal over web) is a single-binary command-line tool that shares a terminal over the web: launching `wesh [flags] -- <cmd> [args...]` serves HTTP/WebSocket on the given port, and opening the page in a browser hands you a full interactive terminal running `<cmd>`. Everything after `--` is passed through verbatim as an exec array, never through a shell.

```
wesh [flags] -- <cmd> [args...]
```

Key features:

- **Single-binary deployment**: the frontend page is embedded into the binary via `go:embed` — scp one file and you are done
- **Read-only by default**: without `--writable`, browser input is discarded by the server, so watching is zero-risk
- **Two session modes**: `shared` (default) has multiple clients share one session — ro/rw share links work by copy-paste, slow clients are protectively evicted, unexpected disconnects reconnect automatically; `per-client` gives every client an independent PTY process (ttyd-style lifecycle), and pairs with herdr/tmux to converge on the same session
- **Secure defaults**: Basic auth plus one-time tickets, TLS, Origin allowlisting, authentication-failure throttling, child-process environment whitelisting
- **Operable in production**: `/healthz` liveness, `/metrics` Prometheus metrics (under per-client these include the `wesh_pty_spawn_total`/`wesh_pty_kills_total` spawn-lifecycle series), JSON structured audit logs (under per-client every successful spawn appends a `session_start` event carrying pid/client_id), graceful shutdown
- **Complete deployment shapes**: TOML config file, UNIX socket, reverse-proxy subpath mounting, systemd/Docker reference recipes

For the full flag list and configuration reference, see [docs/CONFIGURATION.md](docs/CONFIGURATION.en.md).

## Installation

### Prebuilt binaries (recommended)

Releases are tar.gz archives covering the four linux/darwin × amd64/arm64 combinations (Windows is out of scope). Each archive contains `wesh`, `LICENSE`, `README.md` (the Chinese primary copy) and `README.en.md` (this English translation); unpack and run:

```sh
curl -LO https://github.com/sworda/wesh/releases/download/v1.0.0/wesh_v1.0.0_linux_amd64.tar.gz
tar xzf wesh_v1.0.0_linux_amd64.tar.gz
./wesh --version
```

See [Releases](https://github.com/sworda/wesh/releases) for every version and artifact; after downloading, verify integrity against `checksums.txt`:

```sh
sha256sum -c checksums.txt --ignore-missing   # verify only the artifacts already downloaded locally
```

### Building from source

Prerequisites: Go >= 1.26.3, Node.js and pnpm (CI pins Node 24 / pnpm 11.21.0).

```sh
git clone https://github.com/sworda/wesh.git
cd wesh
pnpm -C web install && pnpm -C web build && go build -o wesh ./cmd/wesh
```

After modifying frontend sources under `web/`, **the frontend build must precede `go build`** (`go:embed all:dist` requires `web/dist/` to exist at compile time, and what gets embedded is a build-time snapshot). The repository commits the `web/dist/index.html` build artifact (a real terminal page), so a bare clone can run `go build` / `go test ./...` directly; but once you modify frontend sources under `web/`, you must re-run `pnpm -C web build` before `go build`, otherwise the binary still embeds the stale artifact.

## Quick start

```sh
./wesh --bind 127.0.0.1 -- bash
```

> Why add `--bind 127.0.0.1`: with the default `--bind 0.0.0.0`, having no credential **refuses to start** (secure defaults, see below). Traffic on a loopback listener never leaves the machine, so it works without credentials.

Output after startup (default port 7681; with `--writable` there is one extra read-write share link):

```
listening on http://127.0.0.1:7681
share read-only:  http://127.0.0.1:7681/s/<ro-token>/
```

1. Open the `listening on` address in a browser → you get the terminal (read-only spectator by default; the terminal title carries the `[ro] ` prefix)
2. Send the `share read-only` link to a colleague → they open it and watch the same session live
3. `Ctrl+C` to stop: SIGTERM/SIGINT triggers a graceful shutdown, and child processes die with the process group

## Usage examples

**Local read-only watching** (shortest path; see Quick start):

```sh
./wesh --bind 127.0.0.1 -- bash
```

**Public sharing** (credential + TLS; the browser pops a Basic auth dialog on open):

```sh
./wesh --credential alice:password --tls-cert cert.pem --tls-key key.pem -- bash
```

In production, prefer the `WESH_CREDENTIAL` environment variable over the flag (flag values are visible to other users on the same machine via `ps`).

**Writable collaboration** (everyone can write; under the default `owner` policy the first writable client holds write permission exclusively, and on disconnect it is promoted by attach order):

```sh
./wesh --writable --write-policy all --credential alice:password --tls-cert cert.pem --tls-key key.pem -- bash
```

## Session modes

`--session-mode=shared|per-client` selects the session mode (default `shared`). The `--stop-timeout` default forks by mode: `shared` defaults to `0` (no follow-up SIGKILL; the child process keeps running); `per-client` defaults to `5s` when not set explicitly (a SIGKILL backstop reclaims SIGHUP-immune processes after a client disconnects), while an explicit `0` respects user intent but warns at startup about leak risk.

| Mode | Semantics | How to select |
|------|-----------|---------------|
| `shared` (default) | Several people share one PTY process on the same screen: output is fanned out live to ×N clients, and write permission is arbitrated and promoted via owner — wesh's differentiating core | `--session-mode=shared` or TOML `session-mode = "shared"` (this is already the default) |
| `per-client` | An independent PTY process per client (ttyd-style per-connection lifecycle): disconnect ends it, reconnect gets a brand-new process, size passes straight through with no arbitration | `--session-mode=per-client` or TOML `session-mode = "per-client"` |

Four semantics under `per-client` that run counter to `shared` intuition:

- **A share link is an admission ticket to an independent process at that permission level**: ro/rw links no longer point at the same session view — when the child process is a plain shell, each opener gets a private shell invisible to the others; the ro/rw permission level is still bound to the ticket (mechanism unchanged).
- **ro means input gating on one's own process**: read-only visitors also get an independent process, and wesh discards their keystrokes server-side (read-only is a server-side boundary, identical in both modes); the "watch the same session" experience is preserved indirectly through the child program (see the next item).
- **With herdr/tmux, sessions converge through the multiplexer**: each client's independent herdr/tmux client process connects to the same multiplexer session — the sharing experience is preserved, and each client renders independently at its own terminal geometry, so a mobile attach no longer squeezes the desktop.
- **Reverse-proxy user identity is injected into the child environment**: with `--auth-header` configured (the trusted reverse-proxy user header name, e.g. `X-Remote-User`), the environment of each spawned child additionally carries `WESH_REMOTE_USER=<header value>` (sanitized of control characters), letting the child identify the attached user; nothing is injected when `--auth-header` is unset or the request carries no such header, and `shared` mode has no such injection.

**herdr recipe** (an independent process per client + convergence on the same session via the herdr server; the argv shape is verbatim identical to the subject under test in the repo's UAT `web/uat/phase14.mjs` — the docs are the subject under test):

```sh
wesh --writable --session-mode=per-client -- herdr --session <name>
```

Effect: herdr arbitrates the foreground geometry by the last active client (is_foreground) and renders per-client areas — attaching from mobile flips the layout compact, and the desktop restores the full layout the moment a key is pressed, so the desktop panel is no longer squeezed.

tmux comparison: when several clients attach to the same tmux session, under the default `window-size=latest` activity from a small-screen client shrinks the window and squeezes what the desktop sees; `window-size` is configurable, but every setting carries single-window sizing semantics — there is no per-client independent rendering. When you need heterogeneous sizes not to squeeze each other, use the per-client + herdr recipe above.

### per-client resource obligations and benchmark calibration

Under per-client, client count == child process count: `--max-clients` (default `32`) doubles as the concurrent process cap (a 503 gate at handshake plus a pre-spawn count recheck; concurrent child processes are always ≤ max-clients). **32 concurrent shells is a heavy load** — the default of 32 was measured against the load matrix to be sustainable (data below) and stays unchanged.

Measured calibration (2026-09-06, `go test -tags=load` load matrix, linux/amd64; "verify first, change only when falsified" — every measured value fell inside the stated bounds, zero default changes):

**Residency profile** (N sessions of idle bash, two-sample delta):

| Sessions N | wesh-side memory delta | goroutine delta | fd delta | Child process VmRSS total |
|---|---|---|---|---|
| 1 | 81KiB | 6 | 4 | 3.6MiB |
| 4 | 235KiB | 21 | 16 | 14.4MiB |
| 16 | 960KiB | 81 | 64 | 57.2MiB |
| 32 | 1.9MiB | 161 | 128 | 114.7MiB |

**Flood profile** (an independent seq flood of 33.3MiB per session endpoint; every endpoint received the full stream, zero spurious evictions):

| Sessions N | wesh-side peak Alloc | Full stream received on all endpoints | Per-endpoint throughput (derived) |
|---|---|---|---|
| 1 | 2.7MiB | 3.3s | ≈10.0MiB/s |
| 4 | 2.2MiB | 4.6s | ≈7.2MiB/s |
| 16 | 15.2MiB | 6.7s | ≈5.0MiB/s |
| 32 | 55.0MiB | 8.8s | ≈3.8MiB/s |

Calibration basis: memory/goroutine/fd are wesh-server-side two-sample deltas; VmRSS is the measured total from the child processes' `/proc/<pid>/status`. The measured goroutine delta is 5N+1 (UAT with keepalive off; the stated figure under production `--ping-interval=5s` is 6N). The throughput column is measured per-endpoint bytes ÷ measured completion time (a derived value).

Suggested `--max-clients` values by deployment shape (the default 32 stays; these suggestions are resource-profile tiers, not hard thresholds):

| Deployment shape | Suggested `--max-clients` | Basis |
|---|---|---|
| Ordinary server (default) | 32 — measured sustainable, unchanged | Measured at 32 sessions: wesh-side residency delta ~1.9MiB, flood peak Alloc 55MiB, child processes ~115MiB total (bash) |
| Low-spec VPS / memory-constrained container | 8 | Derived from the measured ~3.7MiB bash child process + ~61KiB wesh-side per session; the real cost depends on `<cmd>` itself (editors/build tools are far above bash), so constrained environments get a conservative tier |
| Personal multi-device (herdr/tmux desktop + phone converging) | 4 | Personal multi-device typically means 1-4 clients; a small cap tightens the process amplification surface if a share link leaks |

**Keepalive kills first** (default `--ping-interval=5s`): a connection that stops reading entirely at the TCP level (never answering pong) is closed with 1006 by the pong timeout inside a window of 5s~10s after it stops reading — closing out earlier than the slow-client watchdog (1013). Real browsers structurally cannot trigger this (the network stack answers pong automatically, unaffected by page throttling); it does apply to clients that manage their own WebSocket socket (herdr-like) if they stop reading — a connection that does not answer pong is a dead connection, so reclaiming it earlier with 1006 is correct behavior. A 1006 triggers the frontend's automatic reconnect; under `per-client` a reconnect gets a brand-new process, which is exactly the right recovery path for a genuinely dead connection.

## Secure defaults

What `wesh` hands out is a shell running as you, and the default configuration refuses to run naked:

- **Startup validation**: with the default `--bind 0.0.0.0`, no credential means it refuses to start (you need the explicit `--no-auth` escape hatch); non-loopback + credential + plaintext HTTP also refuses to start (you need `--insecure-http` or TLS configured). A bare local run on `--bind 127.0.0.1` is unrestricted.
- **Read-only by default**: read-only is a server-side boundary — without `--writable`, both browser keystrokes and INPUT frames from raw WS clients are discarded.
- **Share links die on restart**: ro/rw tokens are randomly regenerated at every startup, so restarting revokes every old link if you suspect a leak.
- **Child-process environment whitelist**: a child sees only `TERM`/`COLORTERM` (fixed injection) and `PATH`/`HOME`/`USER`/`LOGNAME`/`SHELL`/`LANG`/`LC_*` (inherited by name/prefix); every other server-side environment variable is withheld.

## Documentation

| Document | Contents |
|------|------|
| [GETTING-STARTED.md](docs/GETTING-STARTED.en.md) | Prerequisites, installation steps, and first run |
| [ARCHITECTURE.md](docs/ARCHITECTURE.en.md) | System architecture, components, and data flow |
| [CONFIGURATION.md](docs/CONFIGURATION.en.md) | Every CLI flag, TOML config file key, and environment variable |
| [DEPLOYMENT.md](docs/DEPLOYMENT.en.md) | Reverse proxy (nginx/Caddy), systemd, Docker, and UNIX socket deployment |
| [DEVELOPMENT.md](docs/DEVELOPMENT.en.md) | Local development environment and build workflow |
| [TESTING.md](docs/TESTING.en.md) | Test framework and how to run it |

Every document above is an English translation. The primary copy is **Chinese** and sits at the same path without the `.en.md` suffix (for example [README.md](README.md) and [docs/CONFIGURATION.md](docs/CONFIGURATION.md)); the language switcher at the top of each file links the pair. The English translations are maintained by hand and follow the Chinese primary.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.en.md).

## License

[MIT](LICENSE) © 2026 sworda
