// Package cliapp implements the sidesave command-line interface: direct
// offline operations against the local database, plus daemon start for
// headless operation — porting bin/sidesave.js.
package cliapp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bytethecookie/sidesave/internal/api"
	"github.com/bytethecookie/sidesave/internal/daemon"
	"github.com/bytethecookie/sidesave/internal/presets"
	"github.com/bytethecookie/sidesave/internal/store"
	"github.com/bytethecookie/sidesave/internal/sysintegration/upnp"
)

// Run dispatches CLI arguments; returns a process exit code.
func Run(args []string) int {
	// Bare `sidesave` shows what the system is doing right now. Someone
	// typing the command with no arguments wants to know whether it's
	// working, not to read the manual — that's what --help is for.
	if len(args) == 0 {
		return cmdOverview(nil)
	}

	// `sidesave --json` on its own means "the overview, machine-readable" —
	// the flag is documented as working on every command, so it has to work
	// on the default one too.
	if len(args) == 1 && args[0] == "--json" {
		return cmdOverview(args)
	}

	cmd, rest := args[0], args[1:]

	// These talk to the running daemon (or manage it) rather than opening the
	// database, so they must not construct a second daemon — doing so would
	// contend for the same SQLite file and the same port.
	switch cmd {
	case "daemon":
		return cmdDaemon(rest)
	case "upnp":
		return cmdUpnp(rest)
	case "sync":
		return cmdSync(rest)
	case "peers":
		return cmdPeers(rest)
	case "pair":
		return cmdPair(rest)
	case "unpair":
		return cmdUnpair(rest)
	case "relay":
		return cmdRelay(rest)
	case "conflicts":
		return cmdConflicts(rest)
	case "resolve":
		return cmdResolve(rest)
	case "backup":
		return cmdBackup(rest)
	case "prune":
		return cmdPrune(rest)
	case "snapshot-delete":
		return cmdSnapshotDelete(rest)
	case "branch-delete":
		return cmdBranchDelete(rest)
	case "launch":
		return cmdLaunch(rest)
	case "cloud":
		return cmdCloud(rest)
	case "files":
		return cmdFiles(rest)
	case "probe":
		return cmdProbe(rest)
	case "forget":
		return cmdForget(rest)
	case "service":
		return cmdService(rest)
	case "install":
		return cmdInstall(rest)
	case "completion":
		return cmdCompletion(rest)
	case "update":
		return cmdUpdate(rest)
	case "version", "--version", "-v":
		return cmdVersion(rest)
	case "help", "--help", "-h":
		printUsage()
		return 0
	}

	d, err := daemon.New(daemon.Options{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	defer d.Stop()

	// withWatchReload tells an already-running daemon on this machine to
	// re-read the games table, so a change made here takes effect there
	// without waiting for a restart. Best-effort by design: when no daemon is
	// running there is nothing to tell and nothing to report, and failing to
	// reach one must not turn a change that did succeed into an error.
	withWatchReload := func(code int) int {
		if code == 0 {
			_, _ = daemonRequest("POST", "/api/watch/reload", map[string]any{})
		}
		return code
	}

	// Commands below run against a short-lived daemon of our own and write
	// the database directly, so a daemon already running on this machine —
	// the desktop app, or `sidesave daemon start` — never hears about the
	// change. Its watch list is built at startup, so a game added here is
	// tracked but watched by nobody: no auto-snapshots, no auto-sync, and no
	// sign of it until the app is restarted. Anything that alters which games
	// exist, where they live, or whether they auto-sync tells it to catch up.
	switch cmd {
	case "scan":
		return cmdScan(d)
	case "add":
		return withWatchReload(cmdAdd(d, rest))
	case "status":
		return cmdStatus(d, rest)
	case "snapshot":
		return cmdSnapshot(d, rest)
	case "rollback":
		return cmdRollback(d, rest)
	case "branch":
		return cmdBranch(d, rest)
	case "checkout":
		return cmdCheckout(d, rest)
	case "remove":
		return withWatchReload(cmdRemove(d, rest))
	case "snapshots":
		return cmdSnapshots(d, rest)
	case "export":
		return cmdExport(d, rest)
	case "exclude":
		return cmdExclude(d, rest)
	case "link":
		return cmdLink(d, rest)
	case "unlink":
		return cmdUnlinkGame(d, rest)
	case "links":
		return cmdLinks(d, rest)
	case "config":
		return cmdConfig(d, rest)
	case "game":
		return withWatchReload(cmdGame(d, rest))
	case "scanpath":
		return cmdScanPath(d, rest)
	case "untrack-all":
		return withWatchReload(cmdUntrackAll(d, rest))
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", cmd)
		printUsage()
		return 1
	}
}

func runDaemon(args []string) int {
	port := 0 // resolved from settings below
	for i := 0; i < len(args); i++ {
		if args[i] == "--port" && i+1 < len(args) {
			fmt.Sscanf(args[i+1], "%d", &port)
			i++
		}
	}

	d, err := daemon.New(daemon.Options{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	defer d.Stop()

	// Nothing stopped two `daemon start` invocations from racing for the
	// same port and database — whichever lost the race silently fell back
	// to an ephemeral port, leaving peers, the Decky panel, and the
	// desktop app each potentially talking to a different one. Refuse
	// outright when a previous CLI-started daemon is still alive.
	pidPath := daemonPIDPath(d.Paths.HomeDir)
	if pid, alive := aliveDaemonPID(pidPath); alive {
		fmt.Fprintf(os.Stderr, "error: a daemon is already running (pid %d) — `sidesave daemon status` to check it, or `sidesave daemon stop` first\n", pid)
		return 1
	}

	if port == 0 {
		settings, err := d.Store.GetSettings()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		port = settings.Port
	}

	if err := d.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	server := api.New(d)
	addr, err := server.Start(port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	defer server.Stop()
	// The bound port is persisted by api.Server.Start, so `--port` and the
	// ephemeral fallback both advertise something peers can actually reach.

	// Record the PID so `sidesave daemon stop` can find *this* daemon. Only
	// the CLI writes it: the desktop app serves the same API, and stopping it
	// out from under its window would look like a crash.
	_ = os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0o666)
	defer os.Remove(pidPath)

	fmt.Printf("SideSave daemon listening on http://%s\n", addr)
	fmt.Println("Press Ctrl+C to stop.")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	fmt.Println("\nshutting down...")
	return 0
}

// daemonPIDPath is where a CLI-started daemon records its process id.
func daemonPIDPath(homeDir string) string {
	return filepath.Join(homeDir, "daemon.pid")
}

// aliveDaemonPID reads a daemon.pid file and reports whether the process
// it names is still running. A pidfile left over from a daemon that
// didn't exit cleanly (killed, crashed) names a dead PID — that's not
// "already running", just stale housekeeping runDaemon will overwrite.
func aliveDaemonPID(pidPath string) (pid int, alive bool) {
	raw, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, false
	}
	pid, err = strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || pid <= 0 {
		return 0, false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return 0, false
	}
	// Signal 0 checks liveness without actually sending a signal.
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return 0, false
	}
	return pid, true
}

func cmdUpnp(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: sidesave upnp <port> [--delete]")
		return 1
	}
	port := 0
	fmt.Sscanf(args[0], "%d", &port)
	if port <= 0 || port > 65535 {
		fmt.Fprintf(os.Stderr, "invalid port %q\n", args[0])
		return 1
	}
	remove := len(args) > 1 && args[1] == "--delete"

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if remove {
		if err := upnp.Remove(ctx, port); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		fmt.Printf("Removed UPnP mapping for port %d\n", port)
		return 0
	}

	externalIP, err := upnp.Forward(ctx, port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Printf("Port %d forwarded via UPnP", port)
	if externalIP != "" {
		fmt.Printf(" — external address %s:%d", externalIP, port)
	}
	fmt.Println()
	return 0
}

func cmdScan(d *daemon.Daemon) int {
	settings, err := d.Store.GetSettings()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	found := d.Scanner.Scan(settings.CustomScanPaths)
	found = presets.FilterExcluded(found, settings.ExcludePaths)
	if len(found) == 0 {
		section("Auto-scan")
		note("No game saves detected.")
		hint("sidesave add <name> <path>     track a folder yourself")
		fmt.Println()
		return 0
	}

	// Every result is a non-Steam shortcut save — nothing left to group by
	// kind, so a flat numbered list is all `add <n>` needs to resolve.
	numbered := found

	section(fmt.Sprintf("Auto-scan %s %d save location(s)", symDot(), len(found)))
	fmt.Println()
	for i, f := range numbered {
		fmt.Printf("    %s %s\n", accent(fmt.Sprintf("[%d]", i+1)), bold(f.Name))
		fmt.Printf("        %s\n", faint(f.SavePath))
	}

	// Persist what was shown so `sidesave add <n>` means the entry the user is
	// looking at. Re-scanning to resolve the number would be simpler and
	// wrong: the numbering has to survive a save appearing or disappearing
	// between the two commands, and a path retyped by hand from a screen is
	// the papercut this exists to remove.
	saveScanResults(d.Paths.HomeDir, numbered)

	hint("sidesave add <number>          track one of these",
		"sidesave add <name> <path>     track something else")
	fmt.Println()
	return 0
}

// scanResultsPath is where the last listing is remembered for `add <n>`.
func scanResultsPath(homeDir string) string {
	return filepath.Join(homeDir, "last-scan.json")
}

type scanChoice struct {
	Name     string `json:"name"`
	SavePath string `json:"savePath"`
	AppID    string `json:"appId,omitempty"`
}

func saveScanResults(homeDir string, found []presets.DiscoveredSave) {
	out := make([]scanChoice, 0, len(found))
	for _, f := range found {
		out = append(out, scanChoice{Name: f.Name, SavePath: f.SavePath, AppID: f.AppID})
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return // best effort: the two-argument form still works
	}
	_ = os.WriteFile(scanResultsPath(homeDir), raw, 0o666)
}

func loadScanResults(homeDir string) []scanChoice {
	raw, err := os.ReadFile(scanResultsPath(homeDir))
	if err != nil {
		return nil
	}
	var out []scanChoice
	if json.Unmarshal(raw, &out) != nil {
		return nil
	}
	return out
}

func cmdAdd(d *daemon.Daemon, args []string) int {
	// A single numeric argument means "the nth thing the last scan showed".
	// Unambiguous against the two-argument form, so a game genuinely called
	// "3" is still trackable by naming its path.
	if len(args) == 1 {
		n, err := strconv.Atoi(strings.TrimSpace(args[0]))
		if err != nil {
			fmt.Fprintln(os.Stderr, "usage: sidesave add <name> <path>\n       sidesave add <number>   (from the last `sidesave scan`)")
			return 1
		}
		choices := loadScanResults(d.Paths.HomeDir)
		if len(choices) == 0 {
			fmt.Fprintln(os.Stderr, "error: no scan results to pick from — run `sidesave scan` first")
			return 1
		}
		if n < 1 || n > len(choices) {
			fmt.Fprintf(os.Stderr, "error: %d is out of range — the last scan found %d location(s)\n", n, len(choices))
			return 1
		}
		pick := choices[n-1]
		return trackGame(d, pick.Name, pick.SavePath)
	}

	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: sidesave add <name> <path>\n       sidesave add <number>   (from the last `sidesave scan`)")
		return 1
	}
	return trackGame(d, args[0], args[1])
}

// trackGame is the shared tail of both `add` forms, so picking a scan result
// and typing a path by hand cannot drift apart in what they report.
func trackGame(d *daemon.Daemon, name, savePath string) int {
	abs, err := filepath.Abs(savePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	game, err := d.TrackGame(store.Game{Name: name, SavePath: abs})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	success("Now tracking %s", bold(game.Name))
	note("id:   " + game.ID)
	note("path: " + game.SavePath)
	return 0
}

// statusReport is the machine-readable form of the status panel. Named
// fields rather than the raw store rows, so the shape stays a deliberate
// contract instead of whatever the database happens to hold.
type statusReport struct {
	Device string             `json:"device"`
	Games  []statusReportGame `json:"games"`
	Peers  []statusReportPeer `json:"peers"`
}

type statusReportGame struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	SavePath     string         `json:"savePath"`
	AppID        string         `json:"appId,omitempty"`
	ActiveBranch string         `json:"activeBranch"`
	AutoSync     bool           `json:"autoSync"`
	Branches     map[string]int `json:"branches"` // branch -> snapshot count
	// Retention budgets. Automatic and manual snapshots are counted
	// separately so a game that auto-saves constantly cannot evict the
	// snapshots the user took on purpose; 0 means unlimited for automatic
	// ones and "keep forever" for manual ones. Reported here because a
	// headless install has no other way to confirm what it just set.
	MaxSnapshots       int `json:"maxSnapshots"`
	MaxManualSnapshots int `json:"maxManualSnapshots"`
}

type statusReportPeer struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"deviceType"`
	Address string `json:"address"`
	Port    int    `json:"port"`
	Status  string `json:"status"`
}

// cmdStatus is the bare-command overview. It took no arguments at all, which
// meant `--json` was accepted and silently ignored — the flag is advertised on
// every command, and a script asking for JSON got a drawn table with box
// characters in it and no error to notice.
func cmdStatus(d *daemon.Daemon, args []string) int {
	asJSON, _ := jsonFlag(args)

	games, err := d.Store.ListGames()
	if err != nil {
		if asJSON {
			return fail(asJSON, err)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if asJSON {
		report := statusReport{Games: []statusReportGame{}, Peers: []statusReportPeer{}}
		if settings, err := d.Store.GetSettings(); err == nil {
			report.Device = settings.DeviceName
		}
		for _, g := range games {
			entry := statusReportGame{
				ID: g.ID, Name: g.Name, SavePath: g.SavePath, AppID: g.AppID,
				ActiveBranch: g.ActiveBranch, AutoSync: g.AutoSync,
				Branches:           map[string]int{},
				MaxSnapshots:       g.MaxSnapshots,
				MaxManualSnapshots: g.MaxManualSnapshots,
			}
			branches, _ := d.Store.ListBranches(g.ID)
			for _, b := range branches {
				snaps, _ := d.Store.ListSnapshots(g.ID, b)
				entry.Branches[b] = len(snaps)
			}
			report.Games = append(report.Games, entry)
		}
		if peers, err := d.Store.ListPeers(); err == nil {
			for _, p := range peers {
				report.Peers = append(report.Peers, statusReportPeer{
					ID: p.ID, Name: p.Name, Type: p.DeviceType,
					Address: p.Address, Port: p.Port, Status: p.Status,
				})
			}
		}
		return emitJSON(report)
	}

	if len(games) == 0 {
		section("Tracked games")
		note("Nothing tracked yet.")
		hint("sidesave scan", "sidesave add <name> <path>")
		fmt.Println()
		return 0
	}

	section(fmt.Sprintf("Tracked games %s %d", symDot(), len(games)))
	for _, g := range games {
		fmt.Printf("  %s %s  %s\n", symBullet(), bold(g.Name), faint(g.ID))
		fmt.Printf("      %s\n", faint(g.SavePath))

		branches, _ := d.Store.ListBranches(g.ID)
		for _, b := range branches {
			snaps, _ := d.Store.ListSnapshots(g.ID, b)
			label := faint(b)
			if b == g.ActiveBranch {
				label = accent(b) + faint(" (active)")
			}
			fmt.Printf("      %s %s\n", padRight(label, 28),
				faint(fmt.Sprintf("%d snapshot(s)", len(snaps))))
		}
		fmt.Println()
	}

	peers, err := d.Store.ListPeers()
	if err == nil && len(peers) > 0 {
		section("Paired devices")
		t := newTable("device", "type", "address", "status")
		for _, p := range peers {
			status := faint(p.Status)
			if p.Status == "online" {
				status = okText(p.Status)
			}
			t.add(bold(p.Name), faint(p.DeviceType),
				faint(fmt.Sprintf("%s:%d", p.Address, p.Port)), status)
		}
		t.render()
		fmt.Println()
	}
	return 0
}

func cmdSnapshot(d *daemon.Daemon, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: sidesave snapshot <gameId> [comment]")
		return 1
	}
	comment := ""
	if len(args) > 1 {
		comment = strings.Join(args[1:], " ")
	}

	snap, err := d.Snapshots.Create(args[0], comment, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	success("Snapshot %s on branch %s", accent(snap.ID), bold(snap.BranchName))
	note(humanBytes(snap.SizeBytes))
	return 0
}

func cmdRollback(d *daemon.Daemon, args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: sidesave rollback <gameId> <snapshotId>")
		return 1
	}
	snap, err := d.Snapshots.Restore(args[0], args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	success("Restored %s", accent(snap.ID))
	note("taken " + snap.Timestamp)
	return 0
}

func cmdBranch(d *daemon.Daemon, args []string) int {
	// --empty starts the branch with no save at all, for a fresh run. The
	// default copies the current save, so switching to the new branch never
	// leaves the folder empty by surprise.
	copyCurrentSave := true
	rest := make([]string, 0, len(args))
	for _, a := range args {
		if a == "--empty" {
			copyCurrentSave = false
			continue
		}
		rest = append(rest, a)
	}
	if len(rest) < 2 {
		fmt.Fprintln(os.Stderr, "usage: sidesave branch <gameId> <name> [--empty]\n\n"+
			"  Starts from your current save. --empty starts the branch with no\n"+
			"  save, for a fresh run — switching to it clears the save folder\n"+
			"  (your current save is snapshotted first).")
		return 1
	}
	clean, err := d.Snapshots.CreateBranch(rest[0], rest[1], copyCurrentSave)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if copyCurrentSave {
		success("Created branch %s from your current save.", bold(clean))
	} else {
		success("Created empty branch %s.", bold(clean))
		note("Switching to it clears the save folder; your current save is snapshotted first.")
	}
	return 0
}

func cmdCheckout(d *daemon.Daemon, args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: sidesave checkout <gameId> <name>")
		return 1
	}
	if err := d.Snapshots.SwitchBranch(args[0], args[1]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Printf("Switched to branch %q\n", args[1])
	return 0
}

func cmdRemove(d *daemon.Daemon, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: sidesave remove <gameId>")
		return 1
	}
	if err := d.UntrackGame(args[0]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Printf("No longer tracking %q (snapshot files kept on disk)\n", args[0])
	return 0
}
