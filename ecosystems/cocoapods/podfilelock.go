package cocoapods

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Alge/tillit/ecosystems"
)

// PodfileLock is the adapter for CocoaPods' `Podfile.lock` files.
// The lockfile is YAML-shaped but uses a few oddities (mixed string/
// map list entries, indent-sensitive sections) that make decoding
// with a plain YAML parser awkward, so we line-scan the relevant
// sections directly.
type PodfileLock struct{ cocoapodsCommon }

func (PodfileLock) Name() string { return "Podfile.lock" }

// CanParse matches CocoaPods' canonical lockfile name. CocoaPods
// emits `Podfile.lock` with a capital P; lowercase variants are not
// produced and we don't accept them, since picking up a stray file
// would surprise users.
func (PodfileLock) CanParse(p string) bool {
	if p == "" {
		return false
	}
	return filepath.Base(p) == "Podfile.lock"
}

// podLineRE matches a top-level pod line in the PODS section. The
// indent is exactly two spaces followed by `- `; the pod name and
// version are captured. Sub-deps under a pod are at four-space
// indent (`    -`) and don't match this regex.
var podLineRE = regexp.MustCompile(`^  - ([A-Za-z0-9._/+-]+) \(([^)]+)\):?\s*$`)

// checksumLineRE matches a SPEC CHECKSUMS entry: two-space indent,
// pod name, colon, hex hash.
var checksumLineRE = regexp.MustCompile(`^  ([A-Za-z0-9._/+-]+):\s*([A-Fa-f0-9]+)\s*$`)

// checkoutPodRE matches a top-level pod entry in the CHECKOUT
// OPTIONS section: `  PodName:` with no value on the same line.
var checkoutPodRE = regexp.MustCompile(`^  ([A-Za-z0-9._/+-]+):\s*$`)

func (PodfileLock) Parse(fsys fs.FS, lockfilePath string) (ecosystems.ParseResult, error) {
	f, err := fsys.Open(lockfilePath)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("open %s: %w", lockfilePath, err)
	}
	defer f.Close()

	pkgs, warnings := parsePodfileLock(f)
	return ecosystems.ParseResult{Packages: pkgs, Warnings: warnings}, nil
}

func parsePodfileLock(r io.Reader) ([]ecosystems.PackageRef, []string) {
	type podKey struct{ Name, Version string }
	type pod struct {
		Name, Version string
	}

	var pods []pod
	checksums := map[string]string{}
	checkoutPods := map[string]bool{}

	const (
		sectionNone = iota
		sectionPods
		sectionChecksums
		sectionCheckout
	)
	section := sectionNone

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			section = sectionNone
			continue
		}
		// Section headers start at column 0 and end with `:`. We
		// only care about three sections; the rest reset state to
		// none.
		if line[0] != ' ' && strings.HasSuffix(line, ":") {
			switch line {
			case "PODS:":
				section = sectionPods
			case "SPEC CHECKSUMS:":
				section = sectionChecksums
			case "CHECKOUT OPTIONS:":
				section = sectionCheckout
			default:
				section = sectionNone
			}
			continue
		}

		switch section {
		case sectionPods:
			if m := podLineRE.FindStringSubmatch(line); m != nil {
				pods = append(pods, pod{Name: m[1], Version: m[2]})
			}
		case sectionChecksums:
			if m := checksumLineRE.FindStringSubmatch(line); m != nil {
				checksums[m[1]] = m[2]
			}
		case sectionCheckout:
			if m := checkoutPodRE.FindStringSubmatch(line); m != nil {
				checkoutPods[m[1]] = true
			}
		}
	}

	seen := map[podKey]bool{}
	var pkgs []ecosystems.PackageRef
	var warnings []string

	for _, p := range pods {
		k := podKey{Name: p.Name, Version: p.Version}
		if seen[k] {
			continue
		}
		seen[k] = true
		// CHECKOUT OPTIONS keys are typically the parent pod (e.g.,
		// `Firebase`); subspecs (e.g., `Firebase/Core`) inherit the
		// external-source flag from their parent.
		base := p.Name
		if i := strings.Index(base, "/"); i >= 0 {
			base = base[:i]
		}
		if checkoutPods[p.Name] || checkoutPods[base] {
			warnings = append(warnings, fmt.Sprintf("Podfile.lock: skipping %s %s (external source via CHECKOUT OPTIONS)", p.Name, p.Version))
			continue
		}
		pkgs = append(pkgs, ecosystems.PackageRef{
			Ecosystem: "cocoapods",
			PackageID: p.Name,
			Version:   p.Version,
			Hash:      checksums[base], // hash is keyed on the umbrella pod, not the subspec
		})
	}
	return pkgs, warnings
}
