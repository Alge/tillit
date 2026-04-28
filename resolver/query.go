package resolver

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/Alge/tillit/models"
)

// Package returns the viewer's verdict on the package as a sorted list of
// merged spans. Exact decisions contribute single-version spans; delta
// decisions contribute spans covering [FromVersion, ToVersion] (when the
// chain is trusted). Same-status spans that overlap are merged, so a
// long delta chain shows as one continuous range.
func (r *Resolver) Package(viewer, ecosystem, packageID string) (PackageVerdict, error) {
	allSigs, err := r.collectMatching(viewer, ecosystem, packageID)
	if err != nil {
		return PackageVerdict{}, err
	}

	resolveCache := map[string]Verdict{}
	resolving := map[string]bool{}
	versionVerdict := func(version string) Verdict {
		return r.resolveVersion(version, allSigs, resolveCache, resolving)
	}

	// Build raw spans — one per applicable sig.
	var spans []VersionSpan
	for _, sig := range allSigs {
		switch {
		case !sig.isDelta:
			spans = append(spans, VersionSpan{
				From:      sig.exactVersion,
				To:        sig.exactVersion,
				Status:    statusFromLevel(sig.decision.Level),
				Decisions: []ContributingDecision{sig.decision},
			})
		case sig.decision.Level == string(models.DecisionRejected):
			spans = append(spans, VersionSpan{
				From:      sig.fromVersion,
				To:        sig.toVersion,
				Status:    StatusRejected,
				Decisions: []ContributingDecision{sig.decision},
			})
		default:
			// Vetted/allowed delta — chain only counts when from is trusted.
			from := versionVerdict(sig.fromVersion)
			if from.Status == StatusAllowed || from.Status == StatusVetted {
				spans = append(spans, VersionSpan{
					From:      sig.fromVersion,
					To:        sig.toVersion,
					Status:    statusFromLevel(sig.decision.Level),
					Decisions: []ContributingDecision{sig.decision},
				})
			}
		}
	}

	merged := mergeSpans(spans)
	return PackageVerdict{Ecosystem: ecosystem, PackageID: packageID, Spans: merged}, nil
}

// Version returns the verdict for one specific version, considering both
// exact decisions matching it and delta decisions whose [from, to]
// covers it.
func (r *Resolver) Version(viewer, ecosystem, packageID, version string) (Verdict, error) {
	allSigs, err := r.collectMatching(viewer, ecosystem, packageID)
	if err != nil {
		return Verdict{}, err
	}
	return r.resolveVersion(version, allSigs, map[string]Verdict{}, map[string]bool{}), nil
}

// resolveVersion answers: given the trust set's signatures, what's the
// verdict on this specific version? Considers exact matches and delta
// coverage. Memoised against repeated lookups; cycles return Unknown.
func (r *Resolver) resolveVersion(version string, allSigs []sigInfo, cache map[string]Verdict, resolving map[string]bool) Verdict {
	if v, ok := cache[version]; ok {
		return v
	}
	if resolving[version] {
		return Verdict{Status: StatusUnknown}
	}
	resolving[version] = true
	defer delete(resolving, version)

	var decisions []ContributingDecision
	for _, sig := range allSigs {
		if !sig.isDelta {
			if sig.exactVersion == version {
				decisions = append(decisions, sig.decision)
			}
			continue
		}
		// Delta: version must fall in [from, to].
		if CompareVersions(sig.fromVersion, version) > 0 || CompareVersions(version, sig.toVersion) > 0 {
			continue
		}
		if sig.decision.Level == string(models.DecisionRejected) {
			decisions = append(decisions, sig.decision)
			continue
		}
		// Vetted/allowed: chain requires from to be trusted.
		from := r.resolveVersion(sig.fromVersion, allSigs, cache, resolving)
		if from.Status == StatusAllowed || from.Status == StatusVetted {
			decisions = append(decisions, sig.decision)
		}
	}
	v := Verdict{Status: aggregate(decisions), Decisions: decisions}
	cache[version] = v
	return v
}

// sigInfo is one trusted signature about the package, parsed and
// pre-categorised so the resolver doesn't re-parse on every lookup.
type sigInfo struct {
	decision     ContributingDecision
	isDelta      bool
	exactVersion string
	fromVersion  string
	toVersion    string
}

// collectMatching builds the trust set, then returns every parsed
// signature within it that matches (ecosystem, packageID).
func (r *Resolver) collectMatching(viewer, ecosystem, packageID string) ([]sigInfo, error) {
	trustSet, err := r.buildTrustSet(viewer)
	if err != nil {
		return nil, err
	}

	var out []sigInfo
	for signer, entry := range trustSet {
		sigs, err := r.store.GetCachedSignaturesBySigner(signer)
		if err != nil {
			return nil, fmt.Errorf("get signatures for %s: %w", signer, err)
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
				out = append(out, sigInfo{decision: d, exactVersion: p.Version})
			case models.PayloadTypeDeltaDecision:
				out = append(out, sigInfo{
					decision: d, isDelta: true,
					fromVersion: p.FromVersion, toVersion: p.ToVersion,
				})
			}
		}
	}
	return out, nil
}

// mergeSpans sorts spans by From and merges adjacent same-status spans
// whose ranges overlap or touch (next.From ≤ prev.To). Different-status
// spans aren't merged here — callers see overlapping ranges in their
// raw form, which the CLI renders as separate rows.
func mergeSpans(spans []VersionSpan) []VersionSpan {
	if len(spans) == 0 {
		return nil
	}
	sort.Slice(spans, func(i, j int) bool {
		if c := CompareVersions(spans[i].From, spans[j].From); c != 0 {
			return c < 0
		}
		return CompareVersions(spans[i].To, spans[j].To) < 0
	})

	out := []VersionSpan{spans[0]}
	for _, s := range spans[1:] {
		last := &out[len(out)-1]
		if last.Status == s.Status && CompareVersions(s.From, last.To) <= 0 {
			if CompareVersions(s.To, last.To) > 0 {
				last.To = s.To
			}
			last.Decisions = append(last.Decisions, s.Decisions...)
			continue
		}
		out = append(out, s)
	}
	return out
}

// statusFromLevel maps a payload DecisionLevel string to a resolver Status.
// Unknown levels (shouldn't happen with validated payloads) map to Unknown.
func statusFromLevel(level string) Status {
	switch level {
	case string(models.DecisionRejected):
		return StatusRejected
	case string(models.DecisionVetted):
		return StatusVetted
	case string(models.DecisionAllowed):
		return StatusAllowed
	default:
		return StatusUnknown
	}
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
