# The SideSave command line

`sidesave` is a complete client, not a companion to the desktop app. Everything
the app can do, it can do — track games, pair devices, sync, snapshot, roll
back, manage cloud backup. A headless server or a Steam Deck living in Game
Mode never needs the window.

- **Every command in one screen:** `sidesave --help`
- **Full reference page:** `man sidesave` (Linux tarball), or
  [`packaging/man/sidesave.1`](../packaging/man/sidesave.1)
- **This guide:** how the pieces fit together, and the sequences for actual
  tasks

---

## Two things that explain most confusion

### 1. Commands run on the machine they affect

There is no remote control. `sidesave` always acts on **the device you type it
on** — its library, its settings, its saves.

This catches people out with internet sync in particular. If you run a relay on
a VPS, that VPS is a *server*: it introduces your devices to each other. It has
no library and joins no room. `sidesave relay join` belongs on your **gaming
machines**, one at a time, not on the relay.

> **The relay has no SideSave client on it and needs none.** Its whole
> configuration is a port. See [RELAY.md](RELAY.md).

Setting up three devices means running the setup commands three times, once on
each.

### 2. Some commands need a running daemon, most do not

The **daemon** is the background process that watches save folders, talks to
peers and serves the local API. The desktop app runs one; on a headless box you
run one yourself.

- **Needs a daemon** — anything about live state: `sync`, `peers`, `pair`,
  `relay`, `conflicts`, `resolve`, `cloud`, `daemon status`, `probe`, `launch`,
  `prune`, `files`, `backup`.
  Without one you get: *the SideSave daemon isn't reachable at … — start it
  with `sidesave daemon start`*.
- **Works either way** — anything that only touches local data: `scan`, `add`,
  `remove`, `status`, `snapshot`, `snapshots`, `rollback`, `branch`,
  `checkout`, `export`, `config`, `locations`, `ignore`, `link`, `scanpath`,
  `exclude`.

**It is safe to run these while the desktop app is open.** They open the
database directly and then tell the running daemon to re-read it, so a game you
add here starts being watched there without a restart.

The CLI finds the daemon through `~/.sidesave/daemon.addr`, written on start —
so it works even when the port is not the default.

---

## Installing

The CLI ships in the Linux tarball and alongside the Windows build. To put it
on your `PATH`:

```bash
sidesave install              # ~/.local/bin, or %LOCALAPPDATA%\Programs on Windows
sidesave install --dir /usr/local/bin
```

Then shell completion, if you want it:

```bash
sidesave completion bash > /etc/bash_completion.d/sidesave
sidesave completion zsh  > ~/.zsh/completions/_sidesave
sidesave completion fish > ~/.config/fish/completions/sidesave.fish
```

Check it works: `sidesave version`.

---

## Task: set up your first device

```bash
sidesave daemon start &        # or run the desktop app; you need one or the other
sidesave scan                  # see what is on this machine
sidesave add 3                 # track the third result
sidesave add "Elden Ring" ~/.local/share/EldenRing    # or name a folder yourself
sidesave status                # confirm it took
```

`scan` lists one game at a time, with each folder's size and when it was last
written — that last part is how you tell the save you play from one an old
install left behind. A game found in several places shows its other folders
indented underneath. Every folder has its own number, and `add <n>` takes the
one you are looking at.

Empty folders are hidden, because Steam makes one for every game you own.
`sidesave scan --all` shows them.

## Task: pair a second device on the same network

On **device B**, find device A's local IP and ask to pair:

```bash
sidesave pair 192.168.1.42
```

Then on **device A**, approve it:

```bash
sidesave pair requests
sidesave pair approve <peerId>
```

Pairing is mutual by design — nothing connects to your machine without you
saying so on that machine. Check with `sidesave peers`.

Not sure a device is reachable? `sidesave probe 192.168.1.42` answers before
you involve pairing.

## Task: sync between networks (relay)

**On each gaming device** — not on the relay:

```bash
sidesave config set relay-url wss://relay.example.com   # only if self-hosting
sidesave relay join purple-otter-42                     # SAME code everywhere
sidesave relay status
sidesave peers                                          # they should find each other
```

Then pair and approve exactly as on a LAN.

The room code is the only thing deciding who can find whom, so treat it like a
password. Somebody who has it can send your devices a pairing request — they
still cannot sync anything without you approving it.

Leaving the default `relay-url` alone uses the free hosted relay, which is
fine. Running your own: [RELAY.md](RELAY.md).

`sidesave relay leave` stops using internet sync on that device.

## Task: run it permanently on a server or Steam Deck

Linux, as your own user — no root daemon:

```bash
sidesave service install
systemctl --user enable --now sidesave-daemon
sudo loginctl enable-linger $USER     # keeps running when you are not logged in
```

That last line is the one people miss on a Deck: without it, syncing stops the
moment you leave Desktop Mode.

```bash
systemctl --user status sidesave-daemon
journalctl --user -u sidesave-daemon -f
sidesave service uninstall
```

Without systemd, `sidesave daemon start` runs in the foreground; `daemon stop`
stops one the CLI started. Daemons belonging to the desktop app or to systemd
are deliberately left alone.

## Task: day to day

```bash
sidesave status                        # games, branches, peers
sidesave sync --all                    # sync everything now
sidesave sync elden-ring               # or just one
sidesave conflicts                     # anything waiting on a decision
sidesave resolve elden-ring keep-both  # keep-both | keep-local | keep-remote
```

`keep-both` is the safe one: the other device's save lands on a separate
branch, so nothing is discarded while you work out which you wanted.

Auto-sync means you rarely need `sync` by hand; it is there for when you want
to be sure before shutting a machine down.

## Task: snapshots and going back

```bash
sidesave snapshot elden-ring "before the boss"
sidesave snapshots elden-ring
sidesave rollback elden-ring snap_1730000000000
```

Rolling back snapshots your current save first, so it is always reversible.

One file rather than the lot:

```bash
sidesave files elden-ring snap_1730000000000              # list the contents
sidesave files elden-ring snap_1730000000000 ER0000.sl2   # restore just that
```

Parallel playthroughs:

```bash
sidesave branch elden-ring ng-plus          # starts from your current save
sidesave branch elden-ring fresh --empty    # starts with nothing
sidesave checkout elden-ring ng-plus
```

Switching always snapshots onto the branch you are leaving, so a switch can be
undone by switching back.

Getting saves out, unwrapped:

```bash
sidesave export elden-ring ~/Desktop/er-save      # plain files, as the game wrote them
sidesave backup export saves.sscb                 # portable archive of everything
sidesave backup import saves.sscb
```

## Task: a save split across folders

Some games keep progress in one place and settings or mods in another.

```bash
sidesave locations elden-ring
sidesave locations add elden-ring config "C:/Users/me/Documents/Game"
sidesave locations remove elden-ring config
```

**Use the same name on every device.** The name is what the two sides match on
— the folder lives somewhere different on each machine, so the path cannot be.
A folder that is the game's main save, or inside it, is refused: those files
are covered already, and two locations over one set of files fight over them.

Removing a location never deletes anything.

## Task: stop a file syncing

For device-specific settings that live beside the save and break the game if
copied across:

```bash
sidesave ignore elden-ring                          # what is excluded now
sidesave ignore elden-ring add Config.gs
sidesave ignore elden-ring add '*.log'
sidesave ignore elden-ring test Config.gs           # would this sync?
sidesave ignore elden-ring clear
```

Patterns work like a `.gitignore`: a bare name matches at any depth, a leading
`/` anchors to the top, a trailing `/` means a folder, `!` re-includes. Case is
ignored, so a rule written on a PC works on a Deck.

Set the same list on every device — each applies its own. **Snapshots still
capture excluded files**, so an exclusion can never be what loses one.

## Task: cloud backup

Signing in is two steps, because approving access needs a browser:

```bash
sidesave cloud connect google-drive              # prints a link to open
# approve in the browser; it lands on a localhost page that won't load —
# the part you need is code=<something> in its address bar
sidesave cloud connect google-drive --code <code>
sidesave cloud status                            # confirm it took
```

If you open the link on the same machine, it often completes on its own —
`cloud status` says whether it did.

The rest:

```bash
sidesave cloud setup webdav             # WebDAV / webhook / a local folder
sidesave cloud push elden-ring
sidesave cloud list elden-ring
sidesave cloud restore elden-ring <fileName>
sidesave cloud disconnect
```

OneDrive needs your own app registration before it will connect at all; Google
Drive's built-in credentials expire weekly unless you supply your own. Both are
set in the desktop app under **Use your own OAuth app**.

## Task: keep it updated

```bash
sidesave update --check
sidesave update
sidesave config set update-channel beta   # or stable
```

A headless box has no update banner to click, which is why this exists.

---

## Settings

```bash
sidesave config                                   # show everything
sidesave config set device-name "Living Room PC"
sidesave config set port 8390                     # local API + peer port (default 8383)
sidesave config set relay-url wss://relay.example.com
sidesave config set snapshot-limit 20             # automatic snapshots kept per branch
sidesave config set manual-snapshot-limit 0       # 0 = keep yours forever (the default)
sidesave config set match-by-app-id true
sidesave config set update-channel beta
```

### When 8383 is taken

Something else on the machine may already have that port — most often a second
SideSave. Three ways out, depending on how permanent you want it:

```bash
sidesave daemon status                  # is one already running?
sidesave daemon start --port 8390       # this run only
sidesave daemon start --port auto       # any free port the system has
sidesave config set port 8390           # for good; takes effect next start
```

Whatever port it ends up on is written to `~/.sidesave/daemon.addr`, and the
CLI reads that — so `--port auto` does not leave you having to tell it where
the daemon went.

Note that this port is also how **other devices reach this one** over the LAN.
Changing it means the address you give to `sidesave pair <host:port>` changes
with it.

Per game:

```bash
sidesave game elden-ring set auto-sync false
sidesave game elden-ring set max-snapshots 50
sidesave game elden-ring set path /new/location
```

Auto-scan behaviour:

```bash
sidesave scanpath add /mnt/games        # also look here
sidesave exclude add /mnt/old-backups   # never offer this again
```

### Pinning the relay without a database

Settings normally live in SQLite, which is awkward for a machine that is
provisioned rather than configured — a container, or an image rebuilt onto a
fresh volume, where "run a command once after first boot" is not a step you get
to take.

One setting can therefore come from the environment:

```bash
OPENSAVE_RELAY_URL=wss://relay.example.com sidesave daemon start
```

```yaml
# docker-compose
environment:
  - OPENSAVE_RELAY_URL=wss://relay.example.com
```

While it is set it **wins over the stored value**, on every read, so the app,
the CLI and the sync engine all agree on which relay is in use. The field in
the window shows it and says where it came from; `sidesave config set
relay-url` refuses rather than pretending to save. Your stored setting is left
untouched, so unsetting the variable goes back to whatever you had configured.

This is the only setting that works this way. Everything else is
`sidesave config set …`.

---

## Scripting

Every command takes `--json`, anywhere in the arguments.

```bash
sidesave daemon status --json | jq .gameCount
sidesave snapshots elden-ring --json | jq -r '.[0].id'
sidesave conflicts --json | jq 'keys'

# Games found in more than one place
sidesave scan --json | jq -r 'group_by(.groupId)[] | select(length>1) | .[0].name'

# Track the freshest folder of everything detected
sidesave scan --json | jq -r 'to_entries[] | select(.value.role=="primary") | .key+1' \
  | while read n; do sidesave add "$n"; done
```

`scan --json` lists results in the same order the printed listing numbers them,
so index *n* is what `add n` tracks. Each row carries `fileCount`,
`totalBytes`, `latestMtime`, `measured`, and `groupId`/`role` — `measured:
false` means the folder could not be read, which is **not** the same as empty.

Exit codes are 0 on success and 1 on failure. With `--json`, failures print
`{"error": "..."}`.

The daemon also exposes the REST + WebSocket API the desktop app itself uses,
on the address in `~/.sidesave/daemon.addr`, if you would rather drive that.

---

## Where things live

| Path | What |
| --- | --- |
| `~/.sidesave/sidesave.db` | Games, settings, peers, snapshot metadata |
| `~/.sidesave/backups/` | Snapshot archives |
| `~/.sidesave/daemon.addr` | Address of the running daemon — how the CLI finds it |
| `~/.sidesave/daemon.pid` | Written only by `sidesave daemon start` |
| `~/.sidesave/last-scan.json` | What the last scan showed, so `add <n>` means that row |
| `~/.config/systemd/user/sidesave-daemon.service` | From `service install` |

On Windows, `~` is `%USERPROFILE%`.

---

## When something goes wrong

**"the SideSave daemon isn't reachable at …"** — nothing is running. Start the
desktop app, or `sidesave daemon start`, or `systemctl --user start
sidesave-daemon`. If a daemon *is* running on an unusual port, check
`~/.sidesave/daemon.addr` exists and is readable.

**`sidesave: command not found`** — run `sidesave install`, or call it by full
path. On Linux you may need to reopen the shell for `~/.local/bin` to be on
your `PATH`.

**Devices do not find each other** — `sidesave probe <host>` says whether the
other end answers at all. If it does, the problem is pairing, not networking;
if it does not, it is the firewall or the network profile. Across networks,
check both ran `relay join` with the same code *and* the same `relay-url`.

**A change in the CLI does not show in the app** — it should immediately, but
if it does not, restart the daemon. Nothing is lost; the database is the truth.

**A game syncs but one folder does not** — it is probably an extra location
that is not mapped on this device. `sidesave locations <gameId>` shows which
are placed here.

**`add <n>` tracked the wrong thing** — the numbers belong to the most recent
scan on that machine, and `scan --all` numbers differently from `scan` because
it lists more. Re-run the scan and read the numbers from that listing.

---

Still stuck? **[Discord](https://discord.gg/hvBv92DZvn)** is the fastest place
to ask, or open a [GitHub issue](https://github.com/Liquid-co/OpenSave/issues).
