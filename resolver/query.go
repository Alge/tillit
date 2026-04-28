package resolver

import (
	"encoding/json"
	"fmt"

	"github.com/Alge/tillit/models"
)

// Package returns the viewer's verdict on every known version of the
// package. Versions with no contributing decision from a trusted signer
// are absent from the result; callers should treat absence as Unknown.
func (r *Resolver) Package(viewer, ecosystem, packageID string) (PackageVerdict, error) {
	trustSet, err := r.buildTrustSet(viewer)
	if err != nil {
		return PackageVerdict{}, err
	}

	type byVersion = map[string][]ContributingDecision
	collected := byVersion{}

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
				continue // malformed; sync should have rejected it
			}
			if p.Type != models.PayloadTypeDecision {
				continue
			}
			if p.Ecosystem != ecosystem || p.PackageID != packageID {
				continue
			}
			// Veto-only signers contribute only rejected decisions.
			if entry.VetoOnly && p.Level != models.DecisionRejected {
				continue
			}
			collected[p.Version] = append(collected[p.Version], ContributingDecision{
				SignerID:    signer,
				Path:        copyPath(entry.Path),
				Level:       string(p.Level),
				Reason:      p.Reason,
				SignatureID: sig.ID,
				VetoOnly:    entry.VetoOnly,
			})
		}
	}

	versions := make(map[string]Verdict, len(collected))
	for ver, decisions := range collected {
		versions[ver] = Verdict{
			Status:    aggregate(decisions),
			Decisions: decisions,
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
// vetted, else any allowed. Empty decision list is unreachable (callers
// only pass non-empty lists) but defaults to Unknown.
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
