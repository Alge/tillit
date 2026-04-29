package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Alge/tillit/ecosystems"
	"github.com/Alge/tillit/ecosystems/cargo"
	"github.com/Alge/tillit/ecosystems/composer"
	"github.com/Alge/tillit/ecosystems/gosum"
	"github.com/Alge/tillit/ecosystems/hexpm"
	"github.com/Alge/tillit/ecosystems/npmlock"
	"github.com/Alge/tillit/ecosystems/nuget"
	"github.com/Alge/tillit/ecosystems/pub"
	"github.com/Alge/tillit/ecosystems/pypi"
	"github.com/Alge/tillit/resolver"
)

// adapters lists every lockfile-format parser the CLI knows about. Adding
// a new ecosystem is a one-line change here plus the adapter package.
var adapters = []ecosystems.Adapter{
	gosum.GoSum{},
	npmlock.NpmLock{},
	pypi.Requirements{},
	pypi.UvLock{},
	pypi.PoetryLock{},
	pypi.PipfileLock{},
	pypi.PdmLock{},
	hexpm.MixLock{},
	cargo.CargoLock{},
	composer.ComposerLock{},
	nuget.PackagesLock{},
	pub.PubspecLock{},
}

func Check(args []string) error {
	ecosystem, target, err := parseCheckArgs(args)
	if err != nil {
		return err
	}

	// .tillit fills in missing flags. Anything passed on the command
	// line wins so users can override the project default ad-hoc.
	configDir := target
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		configDir = filepath.Dir(target)
	}
	cfg, err := loadTillitConfig(configDir)
	if err != nil {
		return err
	}
	if cfg != nil && ecosystem == "" {
		ecosystem = cfg.Ecosystem
	}

	if ecosystem == "" {
		return fmt.Errorf("ecosystem is required — pass -e <name> or add a .tillit file with 'ecosystem: <name>'.\n%s\n  example: tillit check -e go",
			ecosystemList("  "))
	}

	candidates := adaptersForEcosystem(ecosystem)
	if len(candidates) == 0 {
		return fmt.Errorf("unknown ecosystem %q (known: %s)", ecosystem, knownEcosystems())
	}

	lockfile, err := resolveCheckTarget(target, candidates)
	if err != nil {
		return err
	}

	adapter, err := pickAdapterFrom(lockfile, candidates)
	if err != nil {
		return err
	}

	pkgs, edges, warnings, err := parseLockfile(adapter, lockfile)
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

	userID, err := activeUserID(s)
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

	if edges != nil {
		renderTree(os.Stdout, rows, edges)
	} else {
		// Group output by status, in order: rejected, unknown, allowed, vetted.
		for _, st := range []resolver.Status{
			resolver.StatusRejected,
			resolver.StatusUnknown,
			resolver.StatusAllowed,
			resolver.StatusVetted,
		} {
			printStatusGroup(rows, st)
		}
	}

	fmt.Print(formatSummary(rows))

	if counts[resolver.StatusRejected] > 0 {
		os.Exit(1)
	}
	return nil
}

// parseCheckArgs extracts the optional ecosystem (-e / --ecosystem) and
// path positional from raw args. Both default to empty strings — the
// caller decides whether to require ecosystem (until .tillit support
// lands, the caller does require it). Path defaults to "." when the
// positional is missing.
func parseCheckArgs(args []string) (ecosystem, target string, err error) {
	target = "."
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-e" || a == "--ecosystem":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("%s requires a value (e.g. %s go)", a, a)
			}
			ecosystem = args[i+1]
			i++
		case strings.HasPrefix(a, "--ecosystem="):
			ecosystem = strings.TrimPrefix(a, "--ecosystem=")
		case strings.HasPrefix(a, "-e="):
			ecosystem = strings.TrimPrefix(a, "-e=")
		case strings.HasPrefix(a, "-"):
			return "", "", fmt.Errorf("unknown flag %q", a)
		default:
			target = a
		}
	}
	return ecosystem, target, nil
}

// adaptersForEcosystem returns the adapters whose Ecosystem() matches.
// Multiple adapters can serve the same ecosystem (different lockfile
// formats); all of them are returned so the lockfile resolver can
// match on filename.
func adaptersForEcosystem(ecosystem string) []ecosystems.Adapter {
	var out []ecosystems.Adapter
	for _, a := range adapters {
		if a.Ecosystem() == ecosystem {
			out = append(out, a)
		}
	}
	return out
}

func knownEcosystems() string {
	seen := map[string]bool{}
	var out []string
	for _, a := range adapters {
		if seen[a.Ecosystem()] {
			continue
		}
		seen[a.Ecosystem()] = true
		out = append(out, a.Ecosystem())
	}
	return strings.Join(out, ", ")
}

// ecosystemList returns a multi-line bullet list of every known
// ecosystem and which lockfile format(s) the corresponding adapter
// recognises, prefixed by indent for embedding in error messages.
func ecosystemList(indent string) string {
	byEco := map[string][]string{}
	var order []string
	for _, a := range adapters {
		eco := a.Ecosystem()
		if _, ok := byEco[eco]; !ok {
			order = append(order, eco)
		}
		byEco[eco] = append(byEco[eco], a.Name())
	}
	var b strings.Builder
	b.WriteString(indent + "available ecosystems:\n")
	for _, eco := range order {
		fmt.Fprintf(&b, "%s  - %s (%s)\n", indent, eco, strings.Join(byEco[eco], ", "))
	}
	return strings.TrimRight(b.String(), "\n")
}

// resolveCheckTarget interprets the user's path: a directory means
// "discover the lockfile inside"; a file path means "use it directly".
// Either way the resolved file is checked against the candidate
// adapter list so a wrong ecosystem fails loudly.
func resolveCheckTarget(target string, candidates []ecosystems.Adapter) (string, error) {
	info, err := os.Stat(target)
	if err != nil {
		return "", fmt.Errorf("cannot read %q: %w", target, err)
	}
	if !info.IsDir() {
		return target, nil
	}
	return findLockfile(target, candidates)
}

// findLockfile scans dir (one level — no recursion) for files matching
// any candidate adapter. Returns the path to the lockfile if exactly
// one matches; otherwise errors with enough context for the user.
func findLockfile(dir string, candidates []ecosystems.Adapter) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read %q: %w", dir, err)
	}
	var matches []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		for _, a := range candidates {
			if a.CanParse(name) {
				matches = append(matches, filepath.Join(dir, name))
				break
			}
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no lockfile found in %q for ecosystem (looked for: %s)",
			dir, candidateFormats(candidates))
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("multiple lockfiles found in %q: %s — pass one explicitly",
			dir, strings.Join(matches, ", "))
	}
}

func pickAdapterFrom(lockfile string, candidates []ecosystems.Adapter) (ecosystems.Adapter, error) {
	base := filepath.Base(lockfile)
	for _, a := range candidates {
		if a.CanParse(base) {
			return a, nil
		}
	}
	return nil, fmt.Errorf("no adapter for %q in the requested ecosystem (formats: %s)",
		base, candidateFormats(candidates))
}

func candidateFormats(adapters []ecosystems.Adapter) string {
	out := make([]string, len(adapters))
	for i, a := range adapters {
		out[i] = a.Name()
	}
	return strings.Join(out, ", ")
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

func parseLockfile(adapter ecosystems.Adapter, lockfile string) ([]ecosystems.PackageRef, map[string][]string, []string, error) {
	abs, err := filepath.Abs(lockfile)
	if err != nil {
		return nil, nil, nil, err
	}
	dir, name := filepath.Split(abs)
	fsys := os.DirFS(dir)
	res, err := adapter.Parse(fsys, name)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parse %s: %w", lockfile, err)
	}
	warnings := res.Warnings
	edges := res.Edges
	// Adapters that need shell-out for graph data implement GraphResolver
	// separately so Parse can stay pure (and unit-testable with MapFS).
	if edges == nil {
		if g, ok := adapter.(ecosystems.GraphResolver); ok {
			e, gw := g.Graph(strings.TrimSuffix(dir, string(filepath.Separator)))
			edges = e
			warnings = append(warnings, gw...)
		}
	}
	return res.Packages, edges, warnings, nil
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
