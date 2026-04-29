package swiftpm

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/Alge/tillit/ecosystems"
)

// PackageResolved is the adapter for Swift Package Manager's
// `Package.resolved` files. It supports both the v1 nested schema
// (`{ "object": { "pins": [...] } }`) and the v2 flat schema
// (`{ "pins": [...] }`). Branch-pinned, commit-pinned, and
// fileSystem entries are warn-skipped — only tagged releases
// (`state.version`) are emitted as PackageRefs.
type PackageResolved struct{ swiftpmCommon }

func (PackageResolved) Name() string { return "Package.resolved" }

func (PackageResolved) CanParse(p string) bool {
	if p == "" {
		return false
	}
	return filepath.Base(p) == "Package.resolved"
}

// packageResolvedFile decodes both v1 and v2 layouts. The Pins field
// is populated from the top level when v2; the Object.Pins fallback
// catches v1.
type packageResolvedFile struct {
	Version int                   `json:"version"`
	Pins    []packageResolvedPin  `json:"pins,omitempty"`
	Object  *packageResolvedV1Obj `json:"object,omitempty"`
}

type packageResolvedV1Obj struct {
	Pins []packageResolvedPin `json:"pins"`
}

type packageResolvedPin struct {
	// V2 fields.
	Identity string               `json:"identity,omitempty"`
	Kind     string               `json:"kind,omitempty"`
	Location string               `json:"location,omitempty"`
	State    packageResolvedState `json:"state"`

	// V1 fallbacks (older Xcode-emitted files).
	Package       string `json:"package,omitempty"`
	RepositoryURL string `json:"repositoryURL,omitempty"`
}

type packageResolvedState struct {
	Branch   string `json:"branch,omitempty"`
	Revision string `json:"revision,omitempty"`
	Version  string `json:"version,omitempty"`
}

func (PackageResolved) Parse(fsys fs.FS, lockfilePath string) (ecosystems.ParseResult, error) {
	f, err := fsys.Open(lockfilePath)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("open %s: %w", lockfilePath, err)
	}
	defer f.Close()
	body, err := io.ReadAll(f)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("read %s: %w", lockfilePath, err)
	}

	var doc packageResolvedFile
	if err := json.Unmarshal(body, &doc); err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("parse %s: %w", lockfilePath, err)
	}

	pins := doc.Pins
	if len(pins) == 0 && doc.Object != nil {
		pins = doc.Object.Pins
	}

	type key struct{ ID, Version string }
	seen := map[key]bool{}
	var pkgs []ecosystems.PackageRef
	var warnings []string

	for _, p := range pins {
		// V1 used `package` for the display name; V2 uses
		// `identity` for the canonical lowercased form. Take
		// whichever is set.
		id := p.Identity
		if id == "" {
			id = p.Package
		}
		if id == "" {
			continue
		}

		// fileSystem (and similarly local) kinds are local paths —
		// no remote artifact to vet.
		if p.Kind == "fileSystem" || p.Kind == "local" {
			warnings = append(warnings, fmt.Sprintf("Package.resolved: skipping %s (%s source)", id, p.Kind))
			continue
		}

		if p.State.Version == "" {
			// Branch-pin or commit-pin: useful for development but
			// not vetable through any version-keyed mechanism.
			reason := "no tagged version"
			switch {
			case p.State.Branch != "":
				reason = "branch pin (" + p.State.Branch + ")"
			case p.State.Revision != "":
				reason = "commit-revision pin (no version tag)"
			}
			warnings = append(warnings, fmt.Sprintf("Package.resolved: skipping %s (%s)", id, reason))
			continue
		}

		k := key{ID: id, Version: p.State.Version}
		if seen[k] {
			continue
		}
		seen[k] = true

		// State.Revision is the resolved git commit; we record it
		// as the hash so users can compare against what their build
		// actually fetched, even though it isn't a registry sha256.
		pkgs = append(pkgs, ecosystems.PackageRef{
			Ecosystem: "swiftpm",
			PackageID: id,
			Version:   p.State.Version,
			Hash:      p.State.Revision,
		})
	}
	return ecosystems.ParseResult{Packages: pkgs, Warnings: warnings}, nil
}
