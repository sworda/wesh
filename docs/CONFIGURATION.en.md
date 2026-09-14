<!-- English translation of CONFIGURATION.md. The Chinese CONFIGURATION.md is the primary copy and is
     maintained by the documentation toolchain ("gsd-doc-writer"); keep this translation in
     sync whenever the primary copy changes. Not generated — do not add a gsd-doc-writer marker. -->

# wesh Configuration Reference

**English** | [简体中文](CONFIGURATION.md)

wesh is a single-binary Go program with no implicit config-file search and no registry — all configuration is supplied through three channels, **CLI flags, environment variables, and a TOML config file**, merged in a fixed precedence order:

```
CLI flag > environment variable (WESH_CREDENTIAL) > config file (--config) > built-in defaults
```

The full flag list is available at any time via `wesh --help`; this document takes `cmd/wesh/main.go` (`parseArgs`/`validateStartup`) and `cmd/wesh/config.go` (`fileConfig`) as its source of truth.

## Environment variables

wesh consumes exactly one configuration environment variable; two further read-only probe variables affect `--open` behavior:

| Variable | Required | Default | Description |
|------|------|------|------|
| `WESH_CREDENTIAL` | optional | empty (unset) | A single Basic auth credential `user:pass`. Takes effect only when the `--credential` flag supplies no credential at all (when the flag is non-empty the env var is ignored entirely); when non-empty, the config file's `credential` list is likewise not applied. A malformed value (missing `:`) errors at startup. **Recommended production channel** — flag values are visible to other users on the same host (`ps`), while env has no such exposure surface |
| `DISPLAY` | — | — | Read-only probe: on Linux, when this and `WAYLAND_DISPLAY` are both empty, `--open` deems the session headless, prints a notice and skips launching the browser (non-blocking) |
| `WAYLAND_DISPLAY` | — | — | Same as above, Wayland session detection |

`WESH_REMOTE_USER` is not in the table above — it is an output-type variable that wesh **injects into the child process** (written into the child process env in per-client mode with `--auth-header`), not a configuration input channel; see "per-client behavior notes".

The typical injection channel for the credential env is systemd's `EnvironmentFile=` (file `chmod 600`); see the repository template `deploy/wesh.service`:

```ini
EnvironmentFile=-/etc/wesh/credentials   # content is WESH_CREDENTIAL=user:pass; the - prefix = a missing file does not refuse startup
```

## Config file format (TOML)

Specified **explicitly** via `--config /path/to/wesh.toml` — zero implicit default-path search; a bare `wesh -- bash` behaves byte-for-byte identically to having no config file.

The shape is flat `key = value` (grouped sections are rejected), and **key names = flag names** (hyphenated form). 30 keys in total: 28 keys sharing the names of long-running flags + the `command` exec array + the `index-max-size` config-only key.

```toml
# /etc/wesh/wesh.toml —— recommended chmod 600 when the credential key is present
bind = "127.0.0.1"
port = 7681
credential = ["alice:pw-of-alice"]   # repeatable flag ↔ TOML array
base-path = "/wesh"
index = "/srv/wesh/index.html"       # custom home page (--index, same-name key)
index-max-size = 33554432            # config-only key: integer bytes, default 16MiB
max-clients = 16
ping-interval = "5s"                 # duration keys are in string form
exit-when-empty = "30s"              # "true"/"0"/"30s" carry the same semantics as the three CLI forms
command = ["bash", "-l"]             # exec array; overridden when argv after CLI `--` is non-empty
```

### All 30 config keys

| Key | TOML type | Default | Description |
|----|-----------|--------|------|
| `port` | integer | `7681` | Listen port; `0` = random port, the actual port is printed at startup |
| `bind` | string | `"0.0.0.0"` | Listen address |
| `writable` | boolean | `false` | Master gate for client input (read-only by default) |
| `write-policy` | string | `"owner"` | `owner` (first writer holds exclusive access, promotion on disconnect) or `all` (everyone may write); only meaningful when `writable` is enabled |
| `session-mode` | string | `"shared"` | `shared` (multiple clients share one process, default) or `per-client` (an independent PTY process per WS client, terminated on disconnect); an invalid enum value is rejected at parse time (the same gate for both CLI and TOML sources; the error text echoes the invalid value — enum values are not sensitive); the underscore form `session_mode` is not accepted and refuses startup as an unknown key |
| `max-clients` | integer | `32` | Maximum number of concurrently attached clients; a new client beyond capacity receives 503 |
| `once` | boolean | `false` | Accept only one client and exit after it disconnects (≡ `max-clients=1` + `exit-when-empty` immediate exit) |
| `exit-when-empty` | string | off | Exit after all clients disconnect: `"true"`/`"0"` = immediately; `"30s"` = reconnect grace period |
| `ping-interval` | duration string | `"5s"` | WS ping keepalive interval (prevents a reverse proxy's idle timeout from dropping the connection); `"0"` = disabled |
| `credential` | string array | — | Basic auth credential `user:pass`, multiple entries for per-person revocation |
| `origin` | string array | — | Whitelist of allowed Origin `scheme://host[:port]` |
| `client-option` | string array | — | Client preference `key=value` (whitelisted keys, values are JSON) |
| `tls-cert` | string | — | TLS certificate file path (TLS is enabled only when paired with `tls-key`) |
| `tls-key` | string | — | TLS private key file path (paired with `tls-cert`) |
| `osc52` | boolean | `false` | OSC52 clipboard write switch (write-only, never read) |
| `socket` | string | — | UNIX socket listen path (mutually exclusive with an explicit `port`/`bind`) |
| `socket-mode` | octal string | `"0660"` | Socket permission bits (string form, e.g. `"0660"`); only meaningful together with `socket` |
| `socket-owner` | string | — | Socket owner `user[:group]`; only meaningful together with `socket` |
| `base-path` | string | — | Reverse-proxy sub-path prefix (e.g. `/wesh`; starts with `/`, no trailing slash) |
| `index` | string | — | Path to a custom home page HTML file (replaces the built-in page wholesale) |
| `auth-header` | string | — | Trusted reverse-proxy user header name (e.g. `X-Remote-User`): under shared it is only for audit attribution; under per-client the sanitized value is additionally injected into the child process env as `WESH_REMOTE_USER` (see "per-client behavior notes"). It has no authentication effect in either mode. The four credential-carrier header names Authorization/Proxy-Authorization/Cookie/Set-Cookie are rejected at parse time (case-insensitive, the same gate for CLI and TOML; the error text does not echo the input value) |
| `cwd` | string | inherited | Child process working directory |
| `term` | string | `xterm-256color` semantics | Child process TERM; an empty string is treated as unset |
| `stop-signal` | string | `"HUP"` | Signal sent to the child process group on shutdown: `HUP`/`TERM`/`INT`/`KILL` |
| `stop-timeout` | duration string | diverges by mode (see the "Defaults" table) | Grace period before a follow-up SIGKILL after stop-signal: `shared` defaults to `"0"` (no follow-up); `per-client` defaults to `"5s"` when not set explicitly (SIGKILL as a fallback reaper after disconnect), while an explicit `"0"` respects user intent but warns at startup about leak risk |
| `uid` | integer | `-1` (no privilege drop) | Target uid to drop privileges to (enforced in pairs with `gid`) |
| `gid` | integer | `-1` (no privilege drop) | Target gid to drop privileges to (enforced in pairs with `uid`) |
| `open` | boolean | `false` | Automatically open the share link after startup (skipped after a headless notice) |
| `command` | string array | — | **Config-only key**: child command exec array; overridden when argv after CLI `--` is non-empty; an empty array is equivalent to absence |
| `index-max-size` | integer | `16777216` (16MiB) | **Config-only key**: read-in limit for the custom home page (bytes); no corresponding CLI flag |

### CLI flags only (not accepted in the config file)

The five escape-hatch/informational keys can only be given explicitly via the CLI; writing them into the config file refuses startup as **unknown keys** (exit 2) — an escape hatch must be spoken explicitly, and writing it in the config file is the same as not saying it:

| Flag | Default | Description |
|------|--------|------|
| `--no-auth` | `false` | Escape hatch: allow listening on a non-loopback address without credentials (an explicit declaration of "I know I'm running bare") |
| `--insecure-http` | `false` | Escape hatch: allow non-loopback plaintext HTTP to carry credentials (typical scenario: behind a TLS-terminating reverse proxy) |
| `--config` | — | Specify the TOML config file path (see the sections of this document; both `--config=<path>` and `--config <path>` forms are supported) |
| `--version` | — | Print the version and exit: `wesh <version> (commit <hash>, built <time>)` — release builds have the triple injected via ldflags (time in RFC3339 in the local timezone); local builds carry the version string `dev`, with commit/time backfilled from buildinfo VCS information |
| `--help` | — | Print usage |

### Strict mode and loading semantics

- **fail-fast**: a missing file, a TOML parse failure, and an unknown key all refuse startup with exit 2.
- **Error-text value stripping**: errors contain only "category + key name + line number" (e.g. `invalid config file /etc/wesh/wesh.toml: unknown keys (no-auth)`), and **never echo a config value** — credentials never land in stderr/journald.
- **List replacement semantics**: for the three list keys `credential`/`origin`/`client-option` — if the CLI flag is given, the entire list replaces the config value (the config is not applied and not validated); if the CLI does not give one and the config key exists, each item goes through the same validation as the CLI.
- **Explicit-bit config keys**: `port`/`bind`/`socket-mode`/`socket-owner`/`write-policy`/`session-mode` count as "explicitly set" as soon as they appear in the config file, and take part in mutual-exclusion/combination validation on equal footing with the CLI (e.g. a config that writes both `socket` and `port` refuses startup).
- **In-file self-contradiction rejection**: within the same file, `once = true` together with `max-clients ≠ 1` or an `exit-when-empty` grace ≠ 0 is rejected.
- **Permission warning**: when the file contains the `credential` key and its permissions are not 600/400, a stderr warning is emitted and startup proceeds (non-blocking); for production credentials prefer the `WESH_CREDENTIAL` env and do not write them into the config file.

## Required and optional settings

wesh adopts an "explicit philosophy": the vast majority of keys are optional and have defaults, and the following cases **fail at startup** (config validation error, exit 2):

| Validation | Failure condition | Error message (category) |
|------|----------|------------------|
| Child command required | argv after the CLI `--` is empty and the config `command` key is absent/an empty array | `missing command` |
| Non-loopback requires credentials | bind is non-loopback, no credentials at all, `--no-auth` not given | refusing to listen on non-loopback address without credentials |
| Non-loopback plaintext gate | non-loopback + credentials + no TLS, `--insecure-http` not given | refusing to serve credentials over plaintext HTTP |
| TLS pairing | only one of `--tls-cert`/`--tls-key` given | must give both --tls-cert and --tls-key |
| Privilege-drop pairing | only one of `--uid`/`--gid` given | --uid and --gid must be given together |
| socket mutual exclusion | `--socket` given together with an explicit `--port`/`--bind` | --socket conflicts with --port/--bind |
| socket family given alone | `--socket-mode`/`--socket-owner` given without `--socket` | --socket-mode/--socket-owner require --socket |
| `--open` × `--socket` | both given (a unix socket has no http URL to open) | --open conflicts with --socket |
| Write-policy combination | explicit `--write-policy` without `--writable` enabled | --write-policy is set but --writable is not |
| `--once` contradictory values | `--once` given together with an explicit `--max-clients ≠ 1` or a non-zero grace `--exit-when-empty` | --once conflicts with … |
| `--max-clients` positive | ≤ 0 | --max-clients must be positive |
| `--index` preflight | file missing / not a regular file (directory, device, socket) | invalid --index … |
| `index-max-size` value range | ≤ 0 or > 2GiB | invalid index-max-size … |
| `--cwd` preflight | directory missing | invalid --cwd … |
| per-client command preflight | in `per-client` mode the command is not executable before startup — a path containing `/` (missing/directory/no execute bit; relative paths are resolved against `--cwd`) or a bare name (not in PATH) | invalid command … (per-client startup preflight) |
| Value range/enum | `--write-policy`/`--stop-signal`/`--session-mode` enums, `--socket-mode` octal, `--uid`/`--gid` 0..4294967295, non-negative duration keys, etc. | invalid … (values may be echoed; they are not sensitive) |

**Allowed but warned** (prominent stderr notice, startup not blocked):

- `--no-auth` non-loopback: anyone who can reach the port gets a terminal.
- `--insecure-http` non-loopback: credentials travel over plaintext HTTP (the typical legitimate scenario behind a TLS-terminating reverse proxy).
- `--no-auth` + `--auth-header` non-loopback: directly connecting clients can forge the audit header.
- explicit `--write-policy` × `--session-mode=per-client`: `owner`/`all` arbitration and promotion semantics are not wired in under per-client, so a stderr warning is emitted and startup proceeds (ro/rw permission levels still take effect per ticket).

**Exit code conventions** (for systemd `Restart=` and script orchestration reference):

| Exit code | Semantics |
|--------|------|
| Child process exit code N | The child process exits naturally (e.g. the user types `exit`) → wesh passes the child's real exit code through verbatim (`exit 42` → 42, a normal `exit` → 0) |
| `0` | `--version`/`--help` informational paths |
| `2` | Config/argument validation failure (parse + startup validation matrix + config file loading + custom home page read-in) |
| `1` | Runtime I/O error (TLS certificate loading, listen failure, serve failure — failure paths roll back already spawned child processes, leaving no orphans) |
| `255` | Child process died from a signal (`ExitCode=-1`), plus `--once`/`--exit-when-empty` self-termination and SIGTERM graceful shutdown (the latter two make the child die from a signal via the stop-signal sequence and close out the same way) — `os.Exit(-1)` is truncated by Unix to exit status 255 (systemd `Restart=on-failure` treats it as a failure and self-heals by restarting) |

## Defaults

Built-in defaults when unset (`cmd/wesh/main.go` `parseArgs` lays the groundwork):

| Setting | Default | Description |
|------|--------|------|
| `port` | `7681` | `0` = random port |
| `bind` | `0.0.0.0` | All interfaces (non-loopback — triggers the credential/plaintext validation matrix) |
| `writable` | `false` | Read-only session |
| `write-policy` | `owner` | First writer holds exclusive access + ordered promotion |
| `session-mode` | `shared` | Under `per-client` every WS client gets an independent PTY process (terminated on disconnect; reconnect = a brand-new process) |
| `max-clients` | `32` | 503 when full; under `per-client` it doubles as the concurrent process limit — besides the handshake 503 gate (counting the WS registry) there are two more, a pre-spawn capacity gate and a re-check at the registration point (counting `pcSessions`, including linger sessions awaiting reaping after disconnect; rejection = Error frame + close 1011, text `server is at capacity`), so the number of concurrent child processes is always ≤ max-clients |
| `ping-interval` | `5s` | `0` = keepalive disabled |
| `osc52` | `false` | Clipboard writes off by default |
| `socket-mode` | `0660` | Achieved via an explicit Chmod after listen; does not drift with umask |
| `stop-signal` | `HUP` | Pure single-signal shutdown |
| `stop-timeout` | `shared`: `0`; `per-client`: `5s` | Dual defaults: `shared` sends no follow-up SIGKILL (disconnect does not exit; the child process keeps running); `per-client` defaults to `5s` when not set explicitly (SIGKILL as a fallback reaper of SIGHUP-immune processes after the client disconnects), while an explicit `0` is respected and warns about leak risk |
| `term` | `xterm-256color` | An empty string is treated as unset |
| `uid`/`gid` | `-1` | No privilege drop (the `-1` sentinel; `0` is a legal value for root) |
| `index-max-size` | `16777216` (16MiB) | Hard read-in cap for the custom home page (upper bound 2GiB) |
| `credential`/`origin`/`client-option`/`command` | empty | unset |
| `exit-when-empty` | off | the child process keeps running when there are no clients |
| `tls-cert`/`tls-key`/`socket`/`socket-owner`/`base-path`/`auth-header`/`cwd`/`index` | empty string | unset |

When TLS is not configured (enabled only when `--tls-cert`/`--tls-key` are given as a pair) the server speaks plaintext HTTP — a non-loopback + credentials scenario is then rejected by the startup validation matrix unless `--insecure-http` is given.

### `ping-interval` and disconnect timing

The WS keepalive ping is sent at the configured interval, and the connection is dropped only on pong timeout (the read path never has a deadline; `"0"` disables keepalive). One timing detail worth knowing: **pong timeout (1006) precedes slow-client eviction (1013)** —

- With the default `--ping-interval=5s`, a connection that stops reading entirely at the TCP level (not even returning pong) is closed with 1006 (pong-timeout semantics) within the window of "5s~10s after it stops reading": the built-in 5s write timeout for the server's ping control-frame write is likewise read as a pong timeout once the connection's send window is full, whereas the `per-client` slow-client watchdog (dwell 10s → 1013 `slow_consumer`) structurally arrives later for such connections.
- **Real browsers structurally never trigger it**: the browser WebSocket network stack replies to pong automatically, and page JS throttling or background-tab read suspension makes no difference.
- **Clients that manage their own socket** (e.g. herdr-like programs holding the WebSocket socket directly) are subject to this timing if they stop reading — a connection that does not return pong is a dead connection to begin with, so closing it out earlier with 1006 is correct behavior.
- 1006 triggers the front end's automatic reconnect; under `per-client` mode a reconnect obtains a brand-new process — in a genuinely dead-connection scenario this is a reasonable recovery path, so do not mistake it for watchdog failure.
- Testing note: to isolate and verify dwell watchdog behavior, disable keepalive with `--ping-interval=0` (the established form in the repository UAT `web/uat/phase12.mjs` S6).

### `session-mode=per-client` behavior notes

per-client mode defers spawn until the first client attaches (zero child processes at startup) and introduces four fixed behaviors that take effect only in this mode (none of them has a corresponding flag/config key, other than the capacity gate consuming `--max-clients`):

- **Capacity dual gates, same value, different faces**: `--max-clients` constrains two counting surfaces at once — the 503 gate during the HTTP handshake (counting the WS registry) and the 1011 gate at spawn time (pre-spawn capacity gate + re-check at the registration point, counting `pcSessions`, including linger sessions awaiting reaping after disconnect). When the registry has a free slot but linger processes have not exited, a new client receives a WS-level 1011 (`server is at capacity`) rather than a 503.
- **spawn dual token-bucket throttling** (against network-outage thundering herds and single-point churn): a global bucket of 8 spawn/s (burst 16) + a per-IP bucket of 1 spawn/s (burst 4; the throttling key takes the first entry of the XFF chain when trust is enabled via `--auth-header`). An attach that fails to obtain a token has the same wire form as a capacity rejection (1011 + `server is at capacity`), and 1011 is not in the front end's automatic-reconnect trigger set — a rejection does not amplify into a reconnect storm. The four values are internal constants and are not configurable.
- **`WESH_REMOTE_USER` injection**: when `--auth-header` is configured and the request carries that header, per-client writes the sanitized product of the header value into the env of that client's exclusive child process on every spawn (appending `WESH_REMOTE_USER=<value>` to the tail of the env whitelist) — the header value has C0/C1/DEL control characters stripped and is truncated to 128 runes. Three forms do not inject: `--auth-header` not configured; the request does not carry the header, or the cleaned header value is an empty string (an empty string produces no key); shared mode (zero drift in the child process env). The header has no authentication effect in either mode — per-client injection merely lets programs inside the child process read the reverse proxy's authenticated username; it is identity information passthrough, not authentication.
- **Startup command preflight**: deferring spawn to attach means that a missing/non-executable command, if not surfaced at startup, degrades into an attach-time failure — under per-client the preflight happens at startup (exit 2 fail-fast; see the per-client command preflight row in the "Required and optional settings" table; shared mode keeps the runtime pty.Start error channel unchanged).

## Per-environment overrides

wesh is a single binary and has **no multi-environment file mechanism such as `.env.development`/`.env.production`** — environment differentiation is carried by the combination of "different `--config` paths + env variable injection":

```bash
# Development: loopback + random port + defaults are enough
wesh -- bash

# Production (systemd): TOML carries the long-running parameters + EnvironmentFile carries the credentials
ExecStart=/usr/local/bin/wesh --config /etc/wesh/wesh.toml
EnvironmentFile=-/etc/wesh/credentials   # WESH_CREDENTIAL=user:pass, chmod 600
```

How each deployment form carries configuration:

| Form | Config channel | Reference file |
|------|----------|----------|
| systemd | `--config` TOML + `EnvironmentFile=` (credentials) | `deploy/wesh.service` |
| Docker | command-line flags passed directly (a scratch image has no shell config-customization surface) | `Dockerfile` (ENTRYPOINT `/tini -- /wesh`; flags follow the image command) |
| Manual/scripted | CLI flags passed directly, or `--config` pointing at a per-environment TOML | — |

Notes:

- **Credentials prefer env**: `WESH_CREDENTIAL` injected via systemd `EnvironmentFile=` (chmod 600) takes precedence over plaintext in the config file; a plaintext `credential` in the config file takes effect only when neither env nor flag is given.
- **Changing the custom home page file requires a restart**: `--index` is read into memory once at startup, with zero disk dependency at runtime.
- **Leftover socket self-cleanup**: when the `--socket` path holds a leftover socket endpoint it is cleaned up automatically (zero manual intervention in the systemd `Restart=` scenario); if a non-socket file exists there, startup is refused.
- **Share tokens die on restart**: ro/rw share link tokens are re-randomized on every startup — the semantics of revoking all old links is simply restarting the process.
