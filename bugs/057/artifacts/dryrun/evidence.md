# Phase 3.5 — Dry-Run Fly Deployment Evidence (pgx probe)

Date: 2026-08-02

## Setup
- **Pgx probe** `build/dryrun/main.go` (gitignored `build/`): `db.NewDBDriver(&profile.Profile{Driver:"cockroach", Mode:"prod", DSN: COCKROACH_DSN})` — the app's exact driver init (pgx, simple_protocol, TLS verify-full) → `SELECT 1` → keepalive SELECT 1 every 30s for 20 min (no HTTP listener, simulating the no-listen migration window)
- **Dry-run app** `bchat-crdb-dryrun` (Fly, sjc): static binary copied into debian:bookworm-slim; `[http_service]` mirroring fly_cockroach.toml (grace 60m, auto_stop per variant, min_machines_running=0)
- COCKROACH_DSN set as Fly secret → same Cloud cluster (`great-goat`)

## Run 1 — auto_stop_machines = 'stop' (attempt-1 config, grace 60m)
- Machine `82d394b793e928` created 03:44:55Z; pgx driver init OK (2ms locally; from Fly: SELECT 1 OK)
- keepalive OK through 332s, then:
  - `03:52:18Z stop` event, `03:52:23Z SIGINT + "reboot: Restarting system"`, exit, state=stopped
  - **Death at ~7.4 min — despite grace_period 60m**
- Deploy with `--wait-timeout 3m` errored ("timeout reached waiting for health checks") but machine kept running — informational semantics confirmed
- **Finding:** Fly proxy autostops machines after ~5 min of NO ACTIVE HTTP CONNECTIONS (idle timeout not configurable, community-verified). During migration the app has no listener, so no connections exist → machine dies regardless of grace_period. **grace_period 60m alone does NOT fix attempt-1.**

## Run 2 — auto_stop_machines = 'off' (grace 60m)
- Machine restarted 03:54:49Z with new config (grace 1h0m0s verified in machine JSON)
- keepalive OK at 451s, 511s, 901s, 931s (15.5 min) — machine survived
- Probe exited naturally at 20 min (1202s): `machine exited with exit code 0, not restarting`, state=stopped — clean, designed exit
- **Fix proven:** `auto_stop_machines='off'` keeps the machine alive through arbitrarily long no-listen windows

## Wait/Health Semantics Confirmed
- (a) pgx SELECT 1 from Fly: OK — DSN parse, TLS verify-full, pool, QueryExecMode all work via app code path
- (b) Machine lifetime: 'stop' → ~5-7 min; 'off' → indefinite
- (c) Health-check behavior in no-listen window: grace 60m → no kill (checks fail silently); grace 15s + 'stop' → machine dies (attempt-1)
- (d) `--wait-timeout` expiry: fly deploy returns error but machine continues — deploy stage is informational; poll stage authoritative
- (e) Autostop × long grace: **autostop idle timeout is the binding constraint, NOT the health-check grace**

## Action Taken
- `fly_cockroach.toml`: `auto_stop_machines = 'stop'` → **'off'** (with comment), grace 60m kept
- Phase 5 gains a step: flip `auto_stop_machines` back to 'stop' after first successful boot
- Dry-run app destroyed; `build/dryrun/` kept for reference (gitignored)
