package resolver

import (
	"encoding/json"
	"fmt"

	"github.com/Alge/tillit/models"
)

// Package returns the viewer's verdict on every known version of the
// package. Versions with no contributing decision from a trusted signer
// are absent from the result; callers should treat absence as Unknown.
//
// The resolver handles two payload types:
//
//   - PayloadTypeDecision: an exact-version vetting. Always counts.
//
//   - PayloadTypeDeltaDecision: a vetting of the changes between
//     FromVersion and ToVersion. A REJECTED diff applies unconditionally
//     to ToVersion. An ALLOWED or VETTED diff only applies if
//     FromVersion has a non-rejected verdict from the same trust set
//     (i.e. the chain extends from an already-trusted base).
func (r *Resolver) Package(viewer, ecosystem, packageID string) (PackageVerdict, error) {
	trustSet, err := r.buildTrustSet(viewer)
	if err != nil {
		return PackageVerdict{}, err
	}

	type sigInfo struct {
		decision ContributingDecision
		fromVer  string // empty for exact decisions
	}
	sigsByVersion := map[string][]sigInfo{}

	for signer, entry := range trustSet {
		sigs, err := r.store.GetCachedSignaturesBySigner(signer)
		if err != nil {
			return PackageVerdict{}, fmt.Errorf("get signatures for %s: %w", signer, err)
		}
		for _, sig := range sigs {
			if sig.Revoked {
				continue
			}
			var p models.Payload
			if err := json.Unmarshal([]byte(sig.Payload), &p); err != nil {
				continue
			}
			if p.Ecosystem != ecosystem || p.PackageID != packageID {
				continue
			}
			if entry.VetoOnly && p.Level != models.DecisionRejected {
				continue
			}

			d := ContributingDecision{
				SignerID:    signer,
				Path:        copyPath(entry.Path),
				Level:       string(p.Level),
				Reason:      p.Reason,
				SignatureID: sig.ID,
				VetoOnly:    entry.VetoOnly,
			}

			switch p.Type {
			case models.PayloadTypeDecision:
				sigsByVersion[p.Version] = append(sigsByVersion[p.Version], sigInfo{decision: d})
			case models.PayloadTypeDeltaDecision:
				sigsByVersion[p.ToVersion] = append(sigsByVersion[p.ToVersion], sigInfo{
					decision: d, fromVer: p.FromVersion,
				})
			}
		}
	}

	// Resolve every version with at least one signature, walking diff
	// chains via memoized recursion. resolving tracks the current
	// recursion stack so cycles return Unknown without caching.
	cache := map[string]Verdict{}
	resolving := map[string]bool{}
	var resolve func(version string) Verdict
	resolve = func(version string) Verdict {
		if v, ok := cache[version]; ok {
			return v
		}
		if resolving[version] {
			return Verdict{Status: StatusUnknown}
		}
		resolving[version] = true
		defer delete(resolving, version)

		var decisions []ContributingDecision
		for _, info := range sigsByVersion[version] {
			if info.fromVer == "" {
				decisions = append(decisions, info.decision)
				continue
			}
			// Diff sig — rejection applies unconditionally; approval
			// requires the from-version to be trusted.
			if info.decision.Level == string(models.DecisionRejected) {
				decisions = append(decisions, info.decision)
				continue
			}
			from := resolve(info.fromVer)
			if from.Status == StatusAllowed || from.Status == StatusVetted {
				decisions = append(decisions, info.decision)
			}
		}
		v := Verdict{Status: aggregate(decisions), Decisions: decisions}
		cache[version] = v
		return v
	}

	versions := map[string]Verdict{}
	for version := range sigsByVersion {
		v := resolve(version)
		if len(v.Decisions) > 0 {
			versions[version] = v
		}
	}
	return PackageVerdict{
		Ecosystem: ecosystem,
		PackageID: packageID,
		Versions:  versions,
	}, nil
}

// Version returns the verdict for one specific version. Equivalent to
// looking the version up in Package(...).Versions, but returns an
// explicit Unknown verdict for absent versions.
func (r *Resolver) Version(viewer, ecosystem, packageID, version string) (Verdict, error) {
	pv, err := r.Package(viewer, ecosystem, packageID)
	if err != nil {
		return Verdict{}, err
	}
	if v, ok := pv.Versions[version]; ok {
		return v, nil
	}
	return Verdict{Status: StatusUnknown}, nil
}

// aggregate applies the verdict precedence: any rejected wins, else any
// vetted, else any allowed. Empty decision list defaults to Unknown.
func aggregate(decisions []ContributingDecision) Status {
	hasVetted, hasAllowed := false, false
	for _, d := range decisions {
		switch d.Level {
		case string(models.DecisionRejected):
			return StatusRejected
		case string(models.DecisionVetted):
			hasVetted = true
		case string(models.DecisionAllowed):
			hasAllowed = true
		}
	}
	switch {
	case hasVetted:
		return StatusVetted
	case hasAllowed:
		return StatusAllowed
	default:
		return StatusUnknown
	}
}

func copyPath(p []string) []string {
	out := make([]string, len(p))
	copy(out, p)
	return out
}
