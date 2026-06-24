// Command azdo-ward serves a local web app for visualizing Azure DevOps
// audit-log changes. It holds the PAT in the local config and proxies
// queries to the Audit API, so the token never reaches the browser.
//
// Usage:
//
//	azdo-ward                       # serve the web UI (default :7878)
//	azdo-ward serve --addr :9000    # serve on a custom address
//	azdo-ward connect <org> <pat>   # store/switch an organization + PAT
//	azdo-ward orgs                  # list configured organizations
//	azdo-ward switch <org>          # change the active organization
//	azdo-ward version               # print the version
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dutraph/azdo-ward/internal/api"
	"github.com/dutraph/azdo-ward/internal/auth"
	"github.com/dutraph/azdo-ward/internal/config"
	"github.com/dutraph/azdo-ward/internal/server"
	"github.com/dutraph/azdo-ward/internal/version"
)

const defaultAddr = "127.0.0.1:7878"

func main() {
	args := os.Args[1:]
	cmd := "serve"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}

	switch cmd {
	case "serve":
		cmdServe(args)
	case "connect", "login":
		cmdConnect(args)
	case "orgs", "accounts":
		cmdOrgs()
	case "switch", "ctx":
		cmdSwitch(args)
	case "query", "test":
		cmdQuery(args)
	case "version", "-v", "--version":
		fmt.Println("azdo-ward", version.String())
	case "help", "-h", "--help":
		usage(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		usage(os.Stderr)
		os.Exit(2)
	}
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", "", "address to bind (default "+defaultAddr+")")
	_ = fs.Parse(args)

	cfg := mustLoad()
	bind := *addr
	if bind == "" {
		bind = cfg.Listen
	}
	if bind == "" {
		bind = defaultAddr
	}

	srv, err := server.New(cfg)
	if err != nil {
		fatal(err)
	}
	if cfg.ActiveOrg() == nil {
		fmt.Println("No organization configured yet — open the UI and connect, or run:")
		fmt.Println("  azdo-ward connect <org> <pat>")
	} else {
		fmt.Printf("Active organization: %s\n", cfg.Active)
	}
	fmt.Printf("azdo-ward %s  →  http://%s\n", version.String(), bind)
	if err := srv.Listen(bind); err != nil {
		fatal(err)
	}
}

// cmdQuery runs the audit query headless and prints a summary. It exercises
// the exact same client path the web server uses, so it isolates backend
// issues (auth, permissions, empty window) from the browser/frontend.
func cmdQuery(args []string) {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	days := fs.Int("days", 30, "look back this many days")
	max := fs.Int("max", 50, "max entries to fetch")
	verbose := fs.Bool("v", false, "print each entry's action and timestamp")
	_ = fs.Parse(args)

	cfg := mustLoad()
	active := cfg.ActiveOrg()
	if active == nil {
		fatal(fmt.Errorf("no organization configured — run: azdo-ward connect <org> <pat>"))
	}

	var authz api.Authorizer
	switch active.Mode() {
	case config.AuthEntra:
		authz = api.BearerAuth(auth.EntraToken)
	default:
		if active.PAT == "" {
			fatal(fmt.Errorf("no PAT stored for %q — reconnect: azdo-ward connect %s <pat>", active.Name, active.Name))
		}
		authz = api.PATAuth(active.PAT)
	}

	end := time.Now().UTC()
	start := end.Add(-time.Duration(*days) * 24 * time.Hour)
	fmt.Printf("Org %q (%s auth), window %s → %s\n", active.Name, active.Mode(),
		start.Format("2006-01-02"), end.Format("2006-01-02"))

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	entries, err := api.New(active.Name, authz).QueryAll(ctx,
		api.QueryOptions{StartTime: start, EndTime: end, BatchSize: 1000}, *max)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("Fetched %d entries.\n", len(entries))
	if *verbose {
		for _, e := range entries {
			fmt.Printf("  %s  %-32s  %s\n", e.Timestamp, e.ActionID, e.ActorDisplayName)
		}
	} else if len(entries) > 0 {
		e := entries[0]
		fmt.Printf("Newest: %s  %s  by %s\n", e.Timestamp, e.ActionID, e.ActorDisplayName)
	}
}

func cmdConnect(args []string) {
	fs := flag.NewFlagSet("connect", flag.ExitOnError)
	entra := fs.Bool("entra", false, "use Azure Entra ID auth (az login) instead of a PAT")
	_ = fs.Parse(args)
	rest := fs.Args()

	cfg := mustLoad()
	authMode := config.AuthPAT
	if *entra {
		authMode = config.AuthEntra
	}

	var org, pat string
	if len(rest) > 0 {
		org = rest[0]
	} else {
		org = prompt("Organization: ")
	}
	if org == "" {
		fatal(fmt.Errorf("organization is required"))
	}

	if authMode == config.AuthEntra {
		fmt.Println("Using Azure Entra auth — make sure you've run 'az login'. No PAT stored.")
	} else {
		if len(rest) > 1 {
			pat = rest[1]
		} else {
			pat = prompt("PAT (Audit Log Read scope): ")
		}
	}

	cfg.Upsert(org, pat, authMode)
	if err := cfg.Save(); err != nil {
		fatal(err)
	}
	fmt.Printf("Saved %q (%s auth). It is now the active organization.\n", org, authMode)
}

func cmdOrgs() {
	cfg := mustLoad()
	if len(cfg.Orgs) == 0 {
		fmt.Println("No organizations configured. Run: azdo-ward connect <org> <pat>")
		return
	}
	for _, o := range cfg.Orgs {
		star := "  "
		if o.Name == cfg.Active {
			star = "★ "
		}
		fmt.Printf("%s%-30s [%s]\n", star, o.Name, o.Mode())
	}
}

func cmdSwitch(args []string) {
	if len(args) < 1 {
		fatal(fmt.Errorf("usage: azdo-ward switch <org>"))
	}
	cfg := mustLoad()
	found := false
	for _, o := range cfg.Orgs {
		if o.Name == args[0] {
			found = true
			break
		}
	}
	if !found {
		fatal(fmt.Errorf("unknown organization %q — add it with: azdo-ward connect %s <pat>", args[0], args[0]))
	}
	cfg.Active = args[0]
	if err := cfg.Save(); err != nil {
		fatal(err)
	}
	fmt.Printf("Active organization is now %q.\n", args[0])
}

func mustLoad() *config.Config {
	cfg, err := config.Load()
	if err != nil {
		fatal(err)
	}
	return cfg
}

func prompt(label string) string {
	fmt.Print(label)
	sc := bufio.NewScanner(os.Stdin)
	if sc.Scan() {
		return strings.TrimSpace(sc.Text())
	}
	return ""
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

func usage(w *os.File) {
	fmt.Fprintf(w, `azdo-ward %s — visualize Azure DevOps audit-log changes

Usage:
  azdo-ward [serve] [--addr host:port]   serve the web UI (default %s)
  azdo-ward connect <org> <pat>          store/switch an organization (PAT auth)
  azdo-ward connect <org> --entra        store/switch an organization (Entra auth)
  azdo-ward orgs                         list configured organizations
  azdo-ward switch <org>                 change the active organization
  azdo-ward version                      print the version

Auth modes:
  PAT    — token with the "Audit Log (Read)" scope (vso.auditlog), stored
           locally and never sent to the browser.
  Entra  — for orgs that block PAT auth: tokens are minted on demand via
           the Azure CLI ('az login' once; nothing is stored).
`, version.String(), defaultAddr)
}
