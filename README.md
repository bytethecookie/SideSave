<div align="center">

# SideSave

### Save sync for the games Steam Cloud doesn't cover.

**SideSave** is a personal fork of [OpenSave](https://github.com/Liquid-co/OpenSave), narrowed to one job: syncing save data for non-Steam games added to Steam as shortcuts, between two specific machines, peer-to-peer.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go&logoColor=white)](https://go.dev)
![Platforms](https://img.shields.io/badge/platform-Linux%20%7C%20Steam%20Deck-lightgrey)

</div>

---

## Why this exists

Steam Cloud syncs your save for any game you bought on Steam. It has no idea a
non-Steam game exists — a game added via **Add a Non-Steam Game**, running
through Proton, gets its own prefix under `steamapps/compatdata/<appid>/` and
Valve never looks at it again. Two machines, two separate saves, no sync,
forever.

[OpenSave](https://github.com/Liquid-co/OpenSave) solves that generally — it
scans Steam, a dozen emulators, cracked-game repack conventions, and a
20,000-title community database, then syncs whatever it finds. That's more
than one household actually needs: real Steam games already have Steam Cloud,
and everything else this fork cares about is specifically the non-Steam
shortcuts.

SideSave is what's left after cutting everything down to that one case.

## What's different from upstream

- **Identifies non-Steam shortcuts correctly.** Steam computes a CRC-based
  AppID for a shortcut from its own exe path when the shortcut is created,
  and writes the real game name right next to it in `shortcuts.vdf`. Upstream
  never read that file — a shortcut's save location showed up under whatever
  its save folder happened to be named (`SB`, `GSE Saves`, a hex hash) with no
  way to tell what game it belonged to. SideSave reads `shortcuts.vdf`
  directly: no network lookup, no guessing.
- **Only tracks actual save data.** A game following Unreal's `Saved/`
  convention keeps its real progress in `Saved/SaveGames/`, right next to
  `Saved/Config/` (resolution, graphics quality, key bindings —
  device-specific, and has no business following a save from a Steam Deck to
  a desktop or back) and `Saved/Logs/`/`Saved/CrashReportClient/` (never save
  data at all). Scan now offers only `SaveGames`. The engine-wide
  `AppData/Local/UnrealEngine` crash-reporter cache — not nested under any
  one game — and shader/translation-layer caches like `dxvk` are excluded
  entirely.
- **Cover art works for non-Steam shortcuts.** Upstream only ever asked
  Steam's CDN for box art, keyed by AppID — which 404s for a shortcut's
  locally-computed id, since the CDN has never heard of it. Steam already has
  the art you configured sitting in `userdata/<user>/config/grid/` on disk;
  SideSave checks there first, offline-capable, before ever touching the
  network.
- **Scan shows only what this fork cares about.** Real Steam-Cloud games,
  emulator saves, repack-wrapper detection, portable installs, and the
  Ludusavi community manifest are gone, not just filtered — Steam Cloud
  already covers the first, and the rest was scope this fork doesn't need.
  `internal/presets` went from ~5,900 lines to ~1,200.
- **Won't silently overwrite itself.** The update-check banner is disabled,
  and `sidesave update` points at this fork's own (currently empty) release
  page instead of upstream's — so nothing in normal use will ever replace
  this build with a stock OpenSave release.
- **Warns instead of silently duplicating.** A game synced here from a peer
  before this device had it installed gets tracked under a placeholder path
  (a non-Steam shortcut's AppID means nothing on another device). If this
  device later finds the same game installed for real, SideSave now points
  you at relinking the existing entry instead of quietly tracking a second,
  disconnected copy.
- **Won't silently delete a synced save.** A sync that would delete a large
  share of a save's previously-known files at once now raises a conflict
  instead of applying automatically. Caught live: launching a tracked game on
  a device that had never run it recreates only a handful of "session" files
  in a fresh Proton prefix — the rest simply isn't there yet, not deleted —
  and the lineage-based delete logic used to read that gap as a deliberate
  peer deletion and act on it. The two-sided decision (keep local, keep
  remote) now always goes through you instead.
- **Treats identical paths as identical, even through a symlink.** Steam's
  own `~/.steam/steam -> ~/.local/share/Steam` convention (and equivalents
  elsewhere) means two different-looking paths can be the exact same files on
  disk. Scanning and duplicate-tracking checks now resolve symlinks before
  comparing, so a scan result reached through the symlink can't create a
  second, disconnected copy of an already-tracked save.
- **Renamed throughout** — module path, binaries, Flatpak app ID, Decky
  plugin, data directory. A migration handles `~/.opensave` → `~/.sidesave`
  automatically on first run.

## What's unchanged

The sync engine itself is untouched: block-level delta sync, snapshot
history with branches, LAN auto-discovery and WAN sync via a relay room
code, conflict detection by sync lineage (not wall-clock timestamps), and
optional cloud-backup mirroring. The full CLI, the desktop app, and the
Decky Loader Game Mode plugin all still work — this fork changed what gets
*found* and *shown*, not how syncing itself works once something is tracked.

## Install

This fork doesn't publish releases — build from source:

```bash
# CLI + daemon (this is all a headless install needs)
go build -o sidesave-cli ./cmd/sidesave-cli

# Desktop app (needs Node, the Wails CLI, and libgtk-3-dev + libwebkit2gtk-4.1-dev)
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
cd cmd/sidesave-app && wails build -tags webkit2_41

# Relay server, if self-hosting
go build -o sidesave-relay ./cmd/sidesave-relay
```

### Running it in the background

```bash
mkdir -p ~/.config/systemd/user
cp packaging/systemd/sidesave-daemon-native.service ~/.config/systemd/user/sidesave-daemon.service
# edit ExecStart to point at your built sidesave-cli if it's not on PATH
systemctl --user daemon-reload
systemctl --user enable --now sidesave-daemon
```

On a Steam Deck, also run `sudo loginctl enable-linger $USER` — without it,
SteamOS stops user background processes once the session that started them
ends, which is exactly what you don't want for a sync daemon.

### Steam Deck: Flatpak + Decky plugin

The Flatpak is the practical route on stock SteamOS (no WebKitGTK otherwise).
Build the bundle from `packaging/flatpak/io.github.bytethecookie.SideSave.yml`
with `flatpak-builder` (stage the built `sidesave` and `sidesave-cli`
binaries plus the desktop/icon/metainfo files first), then:

```bash
flatpak install --user sidesave.flatpak
```

For a Game Mode panel, build [`sidesave-decky-plugin/`](sidesave-decky-plugin/)
(`npm install && npm run build`) and copy `plugin.json`, `package.json`,
`main.py`, and `dist/index.js` into `~/homebrew/plugins/SideSave/` on the
Deck, then `sudo systemctl restart plugin_loader`. It can start the daemon
itself from the panel if it isn't already running as a service.

Use `packaging/systemd/sidesave-daemon.service` instead of the `-native` one
for a Flatpak install — its `ExecStart` goes through `flatpak run`.

## How it works

```
   Device A                        Device B
 ┌──────────┐   LAN (auto-discovery)   ┌──────────┐
 │ watcher  │◀───────────────────────▶│ watcher  │
 │ snapshot │                          │ snapshot │
 │  delta   │   WAN via relay room     │  delta   │
 └────┬─────┘   (TLS to the relay)     └────┬─────┘
      │            ┌───────────┐            │
      └───────────▶│   relay   │◀───────────┘
                   │ (routes,  │
                   │  no data) │
                   └───────────┘
```

1. **Identify** — a Proton compatdata prefix is only offered if its AppID
   resolves to a name in `shortcuts.vdf`; real Steam games (matched via
   `appmanifest_*.acf`) are excluded, not just deprioritized.
2. **Watch** — a filesystem watcher notices `Saved/SaveGames` changed (safe-
   write and file-lock aware).
3. **Delta** — the save is chunked into content-defined blocks and SHA-256
   hashed; only changed blocks are ever transferred.
4. **Snapshot & sync** — the new state becomes an immutable, versioned
   snapshot, then syncs to paired peers over LAN or through a relay room.
   Sync lineage tells a receiver a genuine conflict from a fast-forward.

## Command line

`sidesave-cli` is a complete client — scan, add, pair, sync, resolve
conflicts, manage snapshots and branches, run as a background service. Run it
with no arguments for a status panel and suggested next commands.

```bash
sidesave-cli scan                       # non-Steam shortcut saves on this machine
sidesave-cli add <number>               # track one from the scan results
sidesave-cli daemon start                # foreground; use the systemd service for real use
sidesave-cli pair 192.168.1.42          # pair another device on the LAN
sidesave-cli sync --all
sidesave-cli conflicts                  # anything waiting on a decision
sidesave-cli resolve <gameId> keep-both|keep-local|keep-remote
sidesave-cli game <gameId> set path <newPath>   # relink after an install moves
```

Every command accepts `--json` for scripting. Full command reference:
`sidesave-cli --help`, or `packaging/man/sidesave.1`.

## Architecture

```
cmd/sidesave-app       Wails desktop app (daemon embedded + Svelte UI)
cmd/sidesave-cli       Headless daemon & CLI
cmd/sidesave-relay     Stateless WAN relay (room broker + OAuth proxy)
internal/
  store                SQLite persistence + legacy JSON import
  delta                Block hashing, manifest diff, patching
  snapshot             ZIP snapshots, branches, retention
  watcher              Save-change detection (safe-write aware, lock guard)
  p2p                  Discovery, pairing, sync engine, LAN/WAN transports
  cloud                Backup providers + PKCE OAuth
  presets              Non-Steam shortcut detection (shortcuts.vdf, compatdata)
  api                  Local REST + WebSocket dashboard API
  daemon               Long-running service orchestration
  sysintegration       Tray, notifications, autostart
sidesave-decky-plugin  Steam Deck Game Mode plugin (Decky Loader)
```

## Data

Everything lives under `~/.sidesave/` (migrated automatically from
`~/.opensave` on first run of this fork, and from `~/.savesync` before that):

| Path | What |
|---|---|
| `sidesave.db` | SQLite store — tracked games, snapshots, pairings, settings |
| `backups/` | Versioned save snapshots |
| `covers/` | Cached cover art |
| `sidesave.log` | Activity log |

No accounts, no telemetry. See [PRIVACY.md](PRIVACY.md), inherited from
upstream and still accurate.

## Credit

This is a fork of [Liquid-co/OpenSave](https://github.com/Liquid-co/OpenSave)
— all of the actual sync engine (delta hashing, P2P transport, snapshot/branch
history, conflict resolution) is their work, unmodified. This fork only
changed what gets detected and shown. [MIT licensed](LICENSE), retaining the
original author's copyright.
