package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/Alge/tillit/ecosystems"
	"github.com/Alge/tillit/ecosystems/gosum"
	"github.com/Alge/tillit/resolver"
)

// adapters lists every lockfile-format parser the CLI knows about. Adding
// a new ecosystem is a one-line change here plus the adapter package.
var adapters = []ecosystems.Adapter{
	gosum.GoSum{},
}

func Check(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: tillit check <lockfile>")
	}
	lockfile := args[0]

	adapter, err := pickAdapter(lockfile)
	if err != nil {
		return err
	}

	pkgs, warnings, err := parseLockfile(adapter, lockfile)
	if err != nil {
		return err
	}
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
	if len(pkgs) == 0 {
		fmt.Println("No packages found in lockfile.")
		return nil
	}

	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	_, userID, err := activeSignerAndID(s)
	if err != nil {
		return err
	}

	r := resolver.New(s, userID)
	rows := make([]row, 0, len(pkgs))
	counts := map[resolver.Status]int{}
	for _, p := range pkgs {
		v, err := r.Version(userID, p.Ecosystem, p.PackageID, p.Version)
		if err != nil {
			return fmt.Errorf("resolve %s/%s@%s: %w", p.Ecosystem, p.PackageID, p.Version, err)
		}
		rows = append(rows, row{Pkg: p, Status: v.Status, Verdict: v})
		counts[v.Status]++
	}

	fmt.Printf("Checking %s (%s, %d package(s))\n\n", lockfile, adapter.Name(), len(pkgs))

	// Group output by status, in order: rejected, unknown, allowed, vetted.
	for _, st := range []resolver.Status{
		resolver.StatusRejected,
		resolver.StatusUnknown,
		resolver.StatusAllowed,
		resolver.StatusVetted,
	} {
		printStatusGroup(rows, st)
	}

	fmt.Printf("Summary: %d rejected, %d unknown, %d allowed, %d vetted\n",
		counts[resolver.StatusRejected],
		counts[resolver.StatusUnknown],
		counts[resolver.StatusAllowed],
		counts[resolver.StatusVetted],
	)

	if counts[resolver.StatusRejected] > 0 {
		os.Exit(1)
	}
	return nil
}

func pickAdapter(lockfile string) (ecosystems.Adapter, error) {
	base := filepath.Base(lockfile)
	for _, a := range adapters {
		if a.CanParse(base) {
			return a, nil
		}
	}
	return nil, fmt.Errorf("no adapter recognises %q (known formats: %s)",
		base, knownFormats())
}

func knownFormats() string {
	out := ""
	for i, a := range adapters {
		if i > 0 {
			out += ", "
		}
		out += a.Name()
	}
	return out
}

func parseLockfile(adapter ecosystems.Adapter, lockfile string) ([]ecosystems.PackageRef, []string, error) {
	abs, err := filepath.Abs(lockfile)
	if err != nil {
		return nil, nil, err
	}
	dir, name := filepath.Split(abs)
	fsys := os.DirFS(dir)
	res, err := adapter.Parse(fsys, name)
	if err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", lockfile, err)
	}
	return res.Packages, res.Warnings, nil
}

type row struct {
	Pkg     ecosystems.PackageRef
	Status  resolver.Status
	Verdict resolver.Verdict
}

func printStatusGroup(rows []row, status resolver.Status) {
	var matching []row
	for _, r := range rows {
		if r.Status == status {
			matching = append(matching, r)
		}
	}
	if len(matching) == 0 {
		return
	}
	sort.Slice(matching, func(i, j int) bool {
		if matching[i].Pkg.PackageID != matching[j].Pkg.PackageID {
			return matching[i].Pkg.PackageID < matching[j].Pkg.PackageID
		}
		return resolver.CompareVersions(matching[i].Pkg.Version, matching[j].Pkg.Version) < 0
	})
	fmt.Printf("%s (%d):\n", upperStatus(status), len(matching))
	for _, r := range matching {
		marker := " "
		if r.Pkg.Direct {
			marker = "*"
		}
		summary := decisionsInline(r.Verdict.Decisions)
		fmt.Printf("  %s %s %s%s\n", marker, r.Pkg.PackageID, r.Pkg.Version, summary)
	}
	fmt.Println()
}

func upperStatus(s resolver.Status) string {
	switch s {
	case resolver.StatusRejected:
		return "REJECTED"
	case resolver.StatusUnknown:
		return "UNKNOWN"
	case resolver.StatusAllowed:
		return "ALLOWED"
	case resolver.StatusVetted:
		return "VETTED"
	default:
		return string(s)
	}
}

func decisionsInline(ds []resolver.ContributingDecision) string {
	if len(ds) == 0 {
		return ""
	}
	signers := map[string]bool{}
	for _, d := range ds {
		signers[d.SignerID] = true
	}
	names := make([]string, 0, len(signers))
	for s := range signers {
		names = append(names, shortID(s))
	}
	sort.Strings(names)
	out := " ("
	for i, n := range names {
		if i > 0 {
			out += ", "
		}
		out += n
	}
	out += ")"
	return out
}
