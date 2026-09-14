<!-- English translation of DEPLOYMENT.md. The Chinese DEPLOYMENT.md is the primary copy and is
     maintained by the documentation toolchain ("gsd-doc-writer"); keep this translation in
     sync whenever the primary copy changes. Not generated — do not add a gsd-doc-writer marker. -->

# wesh Deployment Guide

**English** | [简体中文](DEPLOYMENT.md)

wesh is a single binary (the frontend page is embedded via `go:embed`), and its deployment philosophy matches scp: **copy one file over and it runs**. This document covers deployment targets and forms, the build/release pipeline, production setup, rollback, and monitoring. For the full semantics of configuration items (flag/TOML/environment variables), see [CONFIGURATION.md](CONFIGURATION.en.md).

## Deployment targets

| Form | Config file | Notes |
|------|-------------|-------|
| Single binary, run directly | — | Prebuilt artifact, unzip and run (scp deployment) |
| systemd long-running service | `deploy/wesh.service` | Recommended for production: credential EnvironmentFile channel + crash self-heal |
| Docker container | `Dockerfile` | Reference image (scratch + tini), user-built — **no images are published** |
| Behind a reverse proxy | — (recipe in the "Reverse proxy" section of this document) | nginx/Caddy verified; Cloudflare not tested |
| UNIX socket + reverse proxy | — | One of the recommended production forms: filesystem permissions are the auth boundary |

Platform boundary: linux/darwin × amd64/arm64, four platforms (Windows is out of support — the PTY layer only carries the linux/darwin build tags).

### Running the single binary directly

Releases are tar.gz archives for four platforms (each containing the trio `wesh` + `LICENSE` + `README.md`); download from [GitHub Releases](https://github.com/sworda/wesh/releases):

```sh
curl -LO https://github.com/sworda/wesh/releases/download/v1.0.0/wesh_v1.0.0_linux_amd64.tar.gz
tar xzf wesh_v1.0.0_linux_amd64.tar.gz
sha256sum -c checksums.txt --ignore-missing   # integrity check (verifies only artifacts already downloaded on this machine)
./wesh --version   # check the version (release builds inject it via ldflags)
```

### systemd (deploy/wesh.service)

The complete unit template is checked into `deploy/wesh.service` (verified on a real machine over the systemctl channel: 255 resurrection semantics / draining window / no resurrection after stop):

```sh
cp deploy/wesh.service /etc/systemd/system/ && systemctl daemon-reload && systemctl enable --now wesh
```

Template highlights (the `[Service]` section):

```ini
EnvironmentFile=-/etc/wesh/credentials   # chmod 600, contents WESH_CREDENTIAL=user:pass (a leading "-" means a missing file is not rejected)
ExecStart=/usr/local/bin/wesh --config /etc/wesh/wesh.toml
Restart=on-failure
RestartSec=2
TimeoutStopSec=15s      # covers the 1001 broadcast + the built-in 5s+5s upper bound for closing stalled clients (does not hit the 90s default)
LimitNOFILE=65536
```

**Exit code 255 and its interaction with Restart**: wesh exits with 255 both on graceful shutdown (SIGTERM) and on session termination (`--once`/`--exit-when-empty`) (the Unix truncation of `os.Exit(-1)`; for the full semantics see the exit code table in [CONFIGURATION.md](CONFIGURATION.en.md)). `Restart=on-failure` classifies any non-zero as a failure, so self-initiated termination restarts too — the expected behavior for the long-running service form (both crashes and self-termination self-heal); whereas a stop initiated by `systemctl stop`/`restart` **never triggers a restart** (systemd knows the stop was its own — measured on systemd 239: after stop, ActiveState=failed but the unit does not come back; the failed state is the normal texture of 255 semantics, not an anomaly).

- Want "stop once the session ends" → change to `Restart=no`.
- Want 255 treated as a normal shutdown → set `SuccessExitStatus=255`.

**Credential channel discipline**: the unit file is world-readable, so credentials must never be written into the unit itself — always use `EnvironmentFile=` (chmod 600).

**UNIX socket unit variant** (socket + reverse proxy, recommended for production):

```ini
[Service]
RuntimeDirectory=wesh
EnvironmentFile=/etc/wesh/credentials
ExecStart=/usr/local/bin/wesh --socket /run/wesh/wesh.sock --socket-owner www-data:www-data -- bash
```

**per-client mode unit tweaks** (`--session-mode=per-client`):

- `TimeoutStopSec=15s` needs no adjustment: the per-client graceful shutdown upper bound is ~7s (the SIGKILL fallback after the stop-signal and the bounded join start counting in parallel; the join bound = `--stop-timeout` default 5s + 2s wrap-up margin), shorter than shared's ~10s (stalled client Close built-in 5s+5s) — so the 15s margin is actually wider.
- `LimitNOFILE=65536` is ample for per-client: the measured per-session fd delta is 4 (128 total across 32 sessions), far below the limit.
- Process budget: the number of concurrent child processes is always ≤ `--max-clients` (default 32), which systemd's default `TasksMax` does not constrain; on low-spec environments, tightening `--max-clients` tightens the process budget proportionally (see the capacity recommendation table under "per-client deployment semantics" below).

### Docker (user-built)

The `Dockerfile` at the repository root is the reference image: `FROM scratch` + a static binary + tini as PID 1 (tini pulled by a pinned sha256). **No images are published** — consistent with the single-binary scp philosophy, users build their own. The local docker build and PID 1 reaping behavior have been measured (the discriminating form: zero zombies in the positive case plus 5 zombies in the no-init negative control).

```sh
# Build prerequisite: produce the static binary at the repository root first (scratch has no dynamic libraries, so wesh must be fully static)
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o wesh ./cmd/wesh
docker build -t wesh .
docker run --rm wesh --version
```

Key points:

- **This image contains no executable commands at all** — scratch is empty, so commands after `--` must come from a bind-mount (e.g. `-v /bin:/bin:ro -v /lib:/lib:ro -v /lib64:/lib64:ro`, the measured form) or from a derived image built `FROM` this one.
- **PID 1 = tini, without `-g`**: tini forwards signals only to its direct child (wesh) — wesh manages the stop-signal process-group sequence itself, and `-g` would double-signal; orphaned grandchildren are reaped by tini.
- **Flags passed straight through**: `ENTRYPOINT ["/tini", "--", "/wesh"]`, with flags following the image command.
- `--socket` inside a container needs a volume to expose the socket file to a host-side reverse proxy; the in-container loopback credential-exempt matrix also applies.
- arm64 build: `docker build --build-arg TARGETARCH=arm64 --build-arg TINI_SHA256=eae1d3aa... .`
- **per-client mode adaptation** (`--session-mode=per-client`): the process-ceiling semantics are unchanged — the number of concurrent child processes is always ≤ `--max-clients`, and tini's duty of reaping orphaned grandchildren stays the same under per-client (on the normal path wesh `Wait`s on and reaps each session's child process itself, so the orphan surface does not grow); memory-constrained containers should tighten `--max-clients` per the capacity recommendation table in the "per-client deployment semantics" section (the real per-session cost = the child process's own RSS; editors/build tools are far above bash's ~3.7MiB).

## Reverse proxy

### nginx (verified)

Subpath mount recipe (`--base-path /wesh`) — note the two critical points, the `Host` header and the WS upgrade:

```nginx
# map the Connection header to upgrade/close (required for the WS upgrade)
map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

server {
    # exact block: bare /wesh (no trailing slash) is explicitly 308-normalized to /wesh/ — 308 preserves the method, and
    # the behavior is independent of the handler form (the automatic 301 trailing-slash add of proxy_pass-family handlers is a special case)
    location = /wesh { return 308 /wesh/; }

    location /wesh/ {
        proxy_pass http://127.0.0.1:7681;
        proxy_http_version 1.1;
        # Host must be forwarded verbatim: nginx forwards $proxy_host (127.0.0.1:port) by default, which is a different
        # origin from the browser's Origin and gets a 403 from wesh's WS same-origin check; $host strips the port, so when
        # the Origin carries a non-default port it still does not match — you must use $http_host (verified end-to-end)
        proxy_set_header Host $http_host;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_read_timeout 3600s;
    }
}
```

**`proxy_read_timeout` must be greater than `--ping-interval` (default `5s`)**: the reverse proxy's idle timeout looks at application-layer traffic — once the WS is established, if no data flows, the proxy drops the connection when the timeout expires. The wesh server sends one WS ping frame per ping interval (application-layer traffic), so as long as the proxy timeout exceeds the ping interval it will not wrongly drop an idle connection; `3600s` is a generous value. With `--ping-interval 0` disabling keepalive, the liveness of idle connections depends entirely on the reverse-proxy timeout.

### Caddy (verified)

Verified end-to-end (2026-08-30, Caddy v2.11.4; seven protocol-layer assertions on Linux + a two-machine end-to-end run with a Windows browser). A root-path reverse proxy needs just one directive:

```caddyfile
wesh.example.com {
    reverse_proxy 127.0.0.1:7681
}
```

The three key differences from nginx (the two platforms' defaults are opposite, so **copying a recipe across them is guaranteed to be wrong**):

- **Host is passed through verbatim by default** — wesh's Origin same-origin check passes naturally, and **no Host configuration line is needed** (nginx must explicitly set `proxy_set_header Host $http_host`).
- **WS upgrade is handled automatically, built in** — zero upgrade configuration lines (nginx needs the `Upgrade`/`Connection` mapping).
- **Site-address semantics are opposite**: writing `http://0.0.0.0:PORT` as a Caddyfile site address is a **literal Host match** (only requests with Host: 0.0.0.0 match), not nginx's "bind all interfaces" listen semantics — a LAN-listening site address must be written as a **bare `:PORT`**; the domain-name form is unaffected.

XFF is added by default (optionally consumed by `--auth-header`). **Idle timeout**: Caddy has no default idle timeout for WS connections after hijacking — wesh's default `--ping-interval 5s` is far below any middlebox idle threshold, so no extra configuration is needed.

### Cloudflare (not tested)

**This section is written from Cloudflare's official documentation and has not been tested** (there is no way to reproduce a SaaS reverse proxy locally; both nginx and Caddy were verified end-to-end). Key points:

- **DNS orange-cloud proxy**: turning on the proxy (orange cloud) for the wesh domain record routes traffic through the CF edge; WebSockets are on by default (Network panel).
- **Idle timeout**: community consensus is about **~100s with no traffic before the connection is closed** (could not be taken directly from official documentation; it is multi-source community consensus)<!-- VERIFY: Cloudflare's idle timeout for WebSocket connections with no traffic is about 100s (community consensus, not taken directly from official documentation) --> — wesh's default `--ping-interval 5s` application-layer ping keeps traffic flowing on the connection, so **the default is safe**; with `--ping-interval 0` disabling keepalive, an idle connection will be closed by CF at ~100s (the frontend's automatic 1006 reconnect recovers it).
- **TLS**: CF terminates at the edge; the origin should use Full (strict) (wesh configured with real certificates via `--tls-cert`/`--tls-key`), or the origin can listen only on loopback with CF pulling from it (the origin is not directly exposed).
- **Host is preserved by default** (same as Caddy); `--base-path` subpath mounting works as usual behind CF.
- **`/s/{token}/` is visible to CF in plaintext**: the share token passes through the CF edge and access logs (CF-side logs are outside your control — be aware before forwarding a share link).

### Reverse-proxy identity passthrough and per-IP rate-limit keys

Giving `--auth-header X-Remote-User` is the master switch for "trust the reverse proxy" (**deploy only behind a reverse proxy**, never expose directly — a direct client can forge that header and pollute audit attribution; a forged header cannot bypass authentication, and the semantics of the identity/ticket/token three channels are unchanged):

- **Audit attribution**: the username injected by the reverse proxy (authelia/oauth2-proxy and other SSO) is sanitized (control characters stripped, truncated to 128 characters) and recorded in the server log's `remote_user` field.
- **per-client extra effect**: the sanitized product is also injected into that client's child-process env (`WESH_REMOTE_USER`) — see the "per-client deployment semantics" section for a per-user shell integration recipe under SSO.
- **XFF chain head swapped in as the per-IP counting key**: once configured, the first IP in the `X-Forwarded-For` chain also serves as the counting key for both throttling and the per-IP half-open connection limit (`throttle`/`halfOpen` share the same clientIP key; the half-open limit defaults to 8, and exceeding it returns 429) — behind a reverse proxy, per-IP counting no longer collapses onto the proxy IP.
- **When unconfigured, XFF is ignored entirely**: without `--auth-header` behind a reverse proxy, throttling and half-open limiting collapse all clients onto the single proxy-IP key — high-concurrency legitimate users may be caught in the crossfire (429).

## per-client deployment semantics

`--session-mode=per-client` (default `shared`) switches the session model from "multiple clients share one PTY child process" to "each WS connection gets its own PTY child process" (ttyd-style lifecycle: disconnect terminates it, reconnect is a brand-new process). For flag/TOML configuration semantics see [CONFIGURATION.md](CONFIGURATION.en.md) "per-client behavior notes"; this section covers only the deployment/operations side.

**Zero child processes at startup**: spawn is deferred until the first client attaches — the startup failure surface narrows to the service itself; a missing/non-executable command is pre-checked and errors at startup (exit 2 fail-fast), and a spawn failure during attach is visible via the `spawn_failed` audit event + a 1011 close.

### `--max-clients` doubles as the concurrent process ceiling

Under per-client the number of clients == the number of child processes, so `--max-clients` (default `32`) doubles as the concurrent process ceiling; two gates, same value, different faces:

| Gate | Trigger point | Counting face | Rejection form |
|------|---------------|---------------|----------------|
| Handshake 503 gate | Before the HTTP upgrade | WS registry (online clients) | HTTP 503 `server is full` |
| spawn gate (pre-spawn check + re-check at the registration point) | After ticket redemption, before/after spawn | `pcSessions` (including linger sessions that disconnected and await reaping) | Error frame + close 1011, text `server is at capacity` |

The number of concurrent child processes is always ≤ `--max-clients` (the re-check at the registration point closes the concurrent handshake race). 1011 is **not in the frontend's automatic-reconnect trigger set** (only 1006 triggers a reconnect) — a capacity rejection will not trigger an amplifying reconnect loop.

### spawn dual token-bucket throttling

per-client has a fixed set of constant throttles ahead of the spawn call site (no flag/TOML keys at all, not configurable):

- **Global bucket 8 spawn/s (burst 16)** — absorbs network-outage thundering herds: N browsers auto-reconnecting simultaneously = N fork+exec, flattened back to 8 spawn/s in steady state;
- **per-IP bucket 1 spawn/s (burst 4)** — absorbs single-point churn (a ticket connect→spawn→disconnect death loop); the per-IP verdict comes first, the global one second.

The throttle keys come from the same source as the "Reverse-proxy identity passthrough and per-IP rate-limit keys" section (the XFF chain head when `--auth-header` trust is on). A throttled attach has the same wire form as a capacity rejection (1011 + fixed text); the audit event is named `spawn_throttled` and the metric is `wesh_pty_spawn_throttled_total` — aggregated on the wire, itemized in the logs.

### `--stop-timeout` dual defaults and leak defense

- `shared` defaults to `0` (disconnect does not exit; the child process keeps running — the existing semantics).
- `per-client` **defaults to `5s` when not set explicitly**: after the client disconnects, `--stop-signal` (default SIGHUP) is sent to the child process group, and if it has not exited within 5s a SIGKILL follow-up reclaims it.
- An explicit `--stop-timeout=0` respects the user's intent (no silent rewriting), but warns about the leak risk at startup — SIGHUP-immune processes (nohup, `trap '' HUP`) linger after disconnect, and combined with disconnect/reconnect churn they can circumvent `--max-clients` and make residency unbounded.

### Resource profile and capacity recommendations

Measured benchmark calibration (2026-09-06, `go test -tags=load` load matrix, linux/amd64; the full residency/flood profile tables are in [README](../README.en.md) "per-client resource obligations and benchmark calibration"):

- Per-session cost (bash idle): child process VmRSS ~3.7MiB + ~61KiB residency on the wesh side; goroutine accounting 6N/session (`--ping-interval=5s`), fd delta 4/session.
- Measured at 32 sessions: ~1.9MiB residency increase on the wesh side, 55MiB peak Alloc under flood, ~115MiB for the child processes combined — **32 concurrent shells is a heavy load**, and the default value has been measured to carry it.
- The real cost depends on the command itself after `--`: the RSS of editors/build tools is far above bash's.

Recommended `--max-clients` values by deployment form (the default 32 stays put; the recommendations are resource-profile tiers, not hard thresholds):

| Deployment form | Recommended `--max-clients` | Basis |
|-----------------|----------------------------|-------|
| Regular server (default) | 32 — measured to carry it, leave unchanged | The full-chain 32-session measurement stays within the accounting lines |
| Low-spec VPS / memory-constrained container | 8 | Computed from ~3.7MiB child process + ~61KiB wesh side per session, a conservative tier |
| Personal multi-device (herdr/tmux desktop + phone aggregation) | 4 | Personal multi-device is typically 1-4 clients; tightens the process amplification surface if a share link leaks |

### Reverse-proxy SSO scenario: `WESH_REMOTE_USER` injection

When `--auth-header X-Remote-User` (the master switch for trusting the reverse proxy) is configured, the difference between per-client and shared goes beyond audit attribution:

- **per-client**: every spawn writes the sanitized injected header value (C0/C1/DEL stripped + truncated to 128 runes) into **that client's own child process** env (`WESH_REMOTE_USER=<value>`) — programs inside each user's shell behind SSO (authelia/oauth2-proxy, etc.) can read the authenticated username directly, enabling per-user home/working-directory initialization. When the request carries no header, or the sanitized value is an empty string, nothing is injected (an empty string produces no key).
- **shared**: only recorded in the server audit log's `remote_user` field, with zero drift in the child process env (a single process shared by multiple users makes injecting an identity meaningless and a privilege overreach).
- In both modes the header has **no authentication power** — authentication is still carried by the credential/ticket/token three channels; `--auth-header` is for reverse-proxy deployments only and must not be exposed directly.

Another note: under per-client an explicit `--write-policy` has no effect (the owner/all arbitration and promotion semantics are not wired, and startup warns and lets it through; ro/rw permission levels still apply per ticket) — read/write control in the SSO scenario shifts instead to the distribution scope of the rw share link.

## Build and release pipeline

### CI (.github/workflows/ci.yml)

Triggers: push + pull_request. Three jobs:

| Job | Content |
|-----|---------|
| `go` | ubuntu/macos two-platform matrix (the darwin leg also covers the kqueue runtime decision): `go vet ./...` + `go test -race -count=1 -v ./...` (**CGO_ENABLED is not set** — `-race` needs cgo) |
| `web` | ubuntu: `pnpm -C web install --frozen-lockfile` + `pnpm -C web build` (tsc type checking and vite build in one) |
| `fuzz` | ubuntu: `FuzzDecodeHello` and `FuzzDecodeFileConfig` each get a 60s short-run regression gate (two targets, two independent invocations) |

### Release (scripts/release.sh → tag push → release.yml)

**The release process = `scripts/release.sh`** (run it once before releasing; the script is the executable form of the release documentation):

```sh
./scripts/release.sh --dry-run v1.0.0   # dry run: runs only the four pre-flight gates, prints the step list without executing
./scripts/release.sh v1.0.0             # real release: pre-flight checks → full test suite → frontend build
                                        #   → long fuzz ×2 (10 minutes per target) → load matrix
                                        #   → confirmation gate → tag push
```

The four pre-flight gates: tag form (`vX.Y.Z`) / tag does not exist / working tree clean / in sync with the remote (degrades to a skip notice when there is no network or no upstream). A fuzz crash aborts immediately — the crash corpus lands in `testdata/fuzz/` automatically, and you re-run after fixing. The confirmation gate echoes the tag to be created and the last 5 commits; only a `yes` answer creates the tag.

**tag push is the only release trigger** (`v*` tags), after which `.github/workflows/release.yml` takes over:

1. `pnpm -C web install --frozen-lockfile` + `pnpm -C web build` (**pnpm build is explicitly sequenced before goreleaser**, without before hooks; the real dist artifacts enter the release binary via `go:embed`)
2. goreleaser `release --clean` (`.goreleaser.yml`): `CGO_ENABLED=0` fully static + `-trimpath` + `-X main.version={{.Version}}` injection + `mod_timestamp` reproducible build; linux/darwin × amd64/arm64, four platforms
3. Artifacts: `wesh_v<TAG>_<os>_<arch>.tar.gz` ×4 + `checksums.txt` (checksums is the only supply-chain file; no cosign signature/SBOM — a threat-model decision for a personal operations tool)

### Local build

Build order is a hard dependency: **the frontend build must precede `go build`** (`go:embed all:dist` requires `web/dist/` to exist at compile time):

```sh
pnpm -C web install && pnpm -C web build && go build -o wesh ./cmd/wesh
```

The repository commits the `web/dist/index.html` build artifact (the real terminal page; a bare clone can `go build`/`go test ./...` directly); after modifying frontend source under `web/` you must re-run `pnpm -C web build` before `go build`, otherwise the binary still embeds the old artifact. Pre-compressed `.gz` artifacts are not committed; the release build produces them on the CI side.

## Production setup

For the full semantics of configuration items (all flags, the 29 TOML keys, the `WESH_CREDENTIAL` environment variable, the startup validation matrix, the exit code table) see [CONFIGURATION.md](CONFIGURATION.en.md). A typical production deployment layout:

```
/etc/wesh/
├── wesh.toml       # TOML config (explicitly named by --config; long-running parameters)
├── credentials     # chmod 600, contents WESH_CREDENTIAL=user:pass (systemd EnvironmentFile)
├── cert.pem        # TLS certificate (--tls-cert)
└── key.pem         # TLS private key (--tls-key)
```

There is only one configurable environment variable, `WESH_CREDENTIAL` — production credentials **prefer this channel** (systemd `EnvironmentFile=` 600); they are not written into the config file and not passed as flags (flag values are visible to same-machine users in `ps`). Precedence chain: CLI flag > `WESH_CREDENTIAL` env > TOML config file > built-in defaults.

Production checklist:

- [ ] Non-loopback listening must have credentials configured (otherwise startup is refused; `--no-auth` is an explicit escape hatch and logs a warning)
- [ ] Non-loopback + credentials must configure TLS, or explicitly `--insecure-http` (the typical legitimate scenario behind a TLS-terminating reverse proxy)
- [ ] Reverse-proxy deployment: `proxy_read_timeout` > `--ping-interval`; configure `--auth-header` when per-IP rate-limit semantics are needed
- [ ] Subpath mounting: `--base-path /wesh` (starts with `/`, no trailing slash; invalid values refuse startup)
- [ ] Drop privileges: give `--uid`/`--gid` as a pair of numbers (name resolution is unavailable in containers without NSS, so look them up first with `id -u`/`id -g`)
- [ ] Combining the `--once`/`--exit-when-empty` session termination form with systemd `Restart=on-failure` restarts in a loop — for a long-running service, confirm this is the desired behavior, otherwise use `Restart=no`
- [ ] per-client mode: size `--max-clients` to the deployment form (default 32 = the value measured to carry heavy load; a low-spec VPS should drop to 8, personal multi-device to 4 — see the capacity recommendation table under "per-client deployment semantics")
- [ ] per-client mode: an explicit `--stop-timeout=0` produces a startup leak warning (SIGHUP-immune processes linger after disconnect) — confirm this is intended before letting it through
- [ ] TLS deployments: beware of HSTS stickiness (`max-age` two years — a browser that has visited the TLS instance forces HTTPS on the same host:port until it expires; going back to HTTP requires clearing the browser's HSTS cache or changing the port)

## Rollback

wesh has no stateful persistence layer (no database, no migrations), so rollback = **swap back the old binary and restart**:

1. Download the previous version's tar.gz from [Releases](https://github.com/sworda/wesh/releases), verify it with `sha256sum -c checksums.txt --ignore-missing`, then replace `/usr/local/bin/wesh`;
2. `systemctl restart wesh` — the restart itself completes the rollback (the frontend is embedded in the binary via `go:embed`, so frontend and backend share one version and there is no separate-rollback problem);
3. **Restart side effect**: the ro/rw share tokens are randomly regenerated on every startup — a rollback (or any restart) automatically revokes all old share links, which must then be redistributed.

systemd semantics in addition:

- A stop initiated by `systemctl stop`/`restart` never triggers a `Restart=` resurrection (systemd knows the stop was its own) — a rollback operation will not race with a self-heal restart.
- A crash of its own (non-zero exit) is brought back automatically under `Restart=on-failure` (RestartSec=2s) — repeated crashes caused by a corrupt/missing binary will hit systemd's start rate limit, and the failed state is visible in `systemctl status wesh`.

## Monitoring

Observability is built into the binary in three faces: `/healthz` health check, `/metrics` Prometheus metrics, and the stderr JSON structured audit log. Both endpoints share the **same port** as the main service and have **fixed root paths** — unaffected by `--base-path` (health checkers/scrapers connect directly to the backend port without going through the reverse proxy, so the paths are constant and can be hardcoded into k8s probes and Prometheus static config).

### Health check (/healthz)

`GET /healthz` returns 200 + a status JSON:

```json
{"status":"ok","clients":2,"max_clients":32,"session_active":true}
```

- `session_active`: the PTY session is alive (flips to false after the child process exits); `clients`/`max_clients`: the current attach count and capacity. **Under per-client mode `session_active` is always `true`** — there is no "session death = service termination" state (session birth and death are per-client-granularity events, and service availability changes are carried only by the draining 503), so orchestrator health checks that look only at the 200/503 status are unaffected.
- **Returns 503 + `"draining"` while a graceful shutdown is in progress** — reverse proxies/orchestrator health checks stop steering new streams to the dying instance during the shutdown window.
- `/healthz` bypassing authentication is the only exception to the site-wide Basic auth gate (a health checker structurally cannot carry credentials, and the endpoint holds zero sensitive information); `/metrics` and the remaining paths go through the auth gate as usual.

### Metrics (/metrics)

`GET /metrics` returns a Prometheus text 0.0.4 exposition (`Content-Type: text/plain; version=0.0.4; charset=utf-8`), 21 series, all gauge/counter, stdlib with zero external dependencies:

| Series | Type | Semantics |
|--------|------|-----------|
| `wesh_clients_connected` | gauge | Number of currently attached WS clients |
| `wesh_clients_total` | counter | Cumulative attaches since process start |
| `wesh_clients_kicked_total` | counter | 1013 slow-consumer evictions |
| `wesh_session_active` | gauge | shared: PTY session alive (1/0); per-client: active session count (HELP text generated per deployment mode) |
| `wesh_outbox_depth_bytes_max` | gauge | Aggregated max of per-client outbox depth (the slow-client detection signal) |
| `wesh_outbox_depth_bytes_sum` | gauge | Aggregated sum of outbox depth |
| `wesh_pty_output_bytes_total` | counter | Bytes read from the PTY source (the fan-out source, counted once) |
| `wesh_ws_sent_bytes_total` | counter | WS downstream bytes (the real bandwidth of fan-out ×N; divide by pty_output for the throughput amplification ratio) |
| `wesh_ws_recv_bytes_total` | counter | WS upstream bytes |
| `wesh_auth_failed_total` | counter | Authentication failures (HTTP 401 + WS Hello ticket redemption failures) |
| `wesh_auth_throttled_total` | counter | Throttle gate rejections (HTTP 429) |
| `wesh_input_rate_dropped_total` | counter | INPUT frames dropped by per-client input rate limiting |
| `wesh_input_queue_dropped_total` | counter | INPUT payloads dropped by the bounded session input queue |
| `wesh_credit_gate_transitions_total` | counter | Number of global credit gate open/close transitions |
| `wesh_goroutines` | gauge | Current goroutine count (leak observability) |
| `wesh_mem_alloc_bytes` | gauge | Heap memory usage |
| `wesh_build_info{version="..."}` | gauge(=1) | Build metadata (injected by release builds; `dev` for development builds) |
| `wesh_pty_spawn_total` | counter | per-client successful spawns (always 0 for shared) |
| `wesh_pty_spawn_failures_total` | counter | per-client spawn start failures (always 0 for shared) |
| `wesh_pty_kills_total` | counter | Number of SIGKILL fallbacks actually delivered to per-client process groups (the teardown + orphan-reap two paths; always 0 for shared) |
| `wesh_pty_spawn_throttled_total` | counter | spawn throttle gate rejections (the only metrics-face signal that the churn defense has "ever fired"; always 0 for shared) |

The last four spawn counters are per-client-only series (under shared they stay at 0, and the series is kept rather than removed) — from these you observe whether the churn/thundering-herd defenses have ever intercepted, and the health of child-process reaping (the spawn−kill difference).

No series carries identity labels such as remote/remote_user/client_id (privacy + label cardinality discipline) — look up per-IP/per-connection detail in the log events; metrics show only totals and aggregates. Once credentials are configured, `/metrics` goes through the same Basic auth gate, which Prometheus supports natively:

```yaml
scrape_configs:
  - job_name: wesh
    static_configs:
      - targets: ['127.0.0.1:7681']   # connect directly to the backend port, not through the reverse proxy
    basic_auth:
      username: alice                 # same group as the wesh credentials (recommend managing it from the same source as WESH_CREDENTIAL)
      password: pw-of-alice
```

**⚠️ Wrong credentials trigger site-wide throttling (429) and self-lock the scrape target**: when scrape credentials are misconfigured, the failures count into the same per-IP exponential backoff counter as browser logins (doubling from 1s, capped at 30s) — scrape is a high-frequency automated client, so the scraper's IP stays locked in the backoff window for a long time, showing up as the target being persistently down. After fixing the credentials, wait for the backoff window to expire (30s at most) and it recovers.

### JSON audit log

Runtime events are always single-line JSON on stderr (stdlib slog JSONHandler, always JSON, always INFO, no switch — read it with jq). Under a systemd deployment stderr goes into the journal automatically:

```bash
# authentication failure audit
journalctl -u wesh -o cat | grep '^\{' | jq -c 'select(.event=="auth_failed")'
# correlate a single connection's lifecycle (attach → detach with the same client_id)
journalctl -u wesh -o cat | grep '^\{' | jq -c 'select(.client_id==7)'
```

(`grep '^\{'` pre-filters the JSON event lines — systemd by default merges the stdout startup banner and the stderr JSON into the same journal, and jq aborts the whole pipeline on a non-JSON line.)

The event catalog covers three faces: authentication (`auth_failed`/`throttled`), connection (`attach`/`detach`), and session lifecycle (the `session_start`/`session_end`/`shutdown`/`exit_when_empty` family); credentials, tickets, and share tokens never enter the log in any form.

Under per-client mode the session event family expands to per-session granularity and becomes correlatable: `session_start` fires exactly once per successful spawn (carrying the child process `pid` + `client_id`), `session_end` once per session termination (carrying `exit_code`/`duration_seconds`/`client_id`, plus a `signal` key when it died from a signal) — `attach`/`detach`/`session_start`/`session_end` are chained by the same `client_id` to trace a single client's entire lifecycle; spawn failure/throttling/capacity rejection land as single-line `spawn_failed`/`spawn_throttled`/`max_clients` events respectively (they are indistinguishable on the wire, all sharing code 1011; the resolution is carried by the event name).

### Graceful shutdown

`SIGTERM`/`SIGINT` (including `systemctl stop`/`restart`) triggers the graceful shutdown sequence: broadcast **1001 Going Away** to all online clients (the frontend shows a terminal-state panel and does not enter automatic reconnect) → run the stop-signal sequence on the child process groups → the process exits (255). `TimeoutStopSec=15s` covers this sequence's upper bound.

per-client branch difference: the stop-signal snapshot source is the whole `pcSessions` set rather than the online registry — leftover sessions in the "client already disconnected, SIGHUP-immune, awaiting reaping" state are covered too (the 1001 broadcast only drives the detach→teardown chain and cannot reach sessions with no client); `--stop-signal` (default SIGHUP) is sent group by group, then SIGKILL is added when `--stop-timeout` (default 5s) expires, starting in parallel with the **bounded join** (upper bound = stopTimeout + 2s wrap-up margin) — it waits for all sessions to converge (upper bound ~7s), D-state unkillable remnants do not drag the shutdown out, and it exits unconditionally once the deadline passes. `TimeoutStopSec=15s` covers it generously.
