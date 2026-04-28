package resolver

import (
	"encoding/json"
	"fmt"

	"github.com/Alge/tillit/models"
)

// trustEntry is one node in the resolved trust set: a signer reachable
// from the viewer, with the path traced back and any veto-only stickiness
// inherited along the way.
type trustEntry struct {
	Path     []string // viewer → ... → SignerID (excluding viewer itself)
	VetoOnly bool     // any edge in the path was veto-only
}

// buildTrustSet performs a BFS from viewer outward, returning every
// signer reachable within the trust depth limits. Direct edges from the
// active local user come from the peers table (so veto-only / distrusted
// flags apply); transitive edges and edges from non-active viewers come
// from cached_connections.
//
// Cycles are skipped; the first time we visit a node we record its path
// and we don't reduce its depth budget on subsequent visits (BFS yields
// the shortest path first).
func (r *Resolver) buildTrustSet(viewer string) (map[string]trustEntry, error) {
	type frontier struct {
		signer    string
		path      []string
		depthLeft int  // edges still allowed beyond this node
		vetoOnly  bool // path went through a veto-only edge
	}

	// Local distrust set (only meaningful when viewer is the active user —
	// other users' distrust is private to them and never published).
	distrusted := map[string]bool{}
	if viewer == r.activeUserID && r.activeUserID != "" {
		peers, err := r.store.ListPeers()
		if err != nil {
			return nil, fmt.Errorf("list peers: %w", err)
		}
		for _, p := range peers {
			if p.Distrusted {
				distrusted[p.ID] = true
			}
		}
	}

	result := map[string]trustEntry{}
	queue := []frontier{}

	// Seed from direct edges.
	if viewer == r.activeUserID && r.activeUserID != "" {
		peers, err := r.store.ListPeers()
		if err != nil {
			return nil, fmt.Errorf("list peers: %w", err)
		}
		// TrustDepth is the number of *additional* transitive hops allowed
		// past the direct peer. depth=0 means direct only.
		for _, p := range peers {
			if p.Distrusted {
				continue
			}
			queue = append(queue, frontier{
				signer:    p.ID,
				path:      []string{p.ID},
				depthLeft: p.TrustDepth,
				vetoOnly:  p.VetoOnly,
			})
		}
	} else {
		// Non-active viewer — use their public connections.
		conns, err := r.signerOutgoing(viewer)
		if err != nil {
			return nil, err
		}
		for _, c := range conns {
			queue = append(queue, frontier{
				signer:    c.OtherID,
				path:      []string{c.OtherID},
				depthLeft: c.TrustExtends,
				vetoOnly:  false, // viewer's veto-only flags aren't visible to us
			})
		}
	}

	for len(queue) > 0 {
		f := queue[0]
		queue = queue[1:]

		if distrusted[f.signer] {
			continue
		}
		if _, seen := result[f.signer]; seen {
			continue
		}
		result[f.signer] = trustEntry{Path: f.path, VetoOnly: f.vetoOnly}

		if f.depthLeft <= 0 {
			continue
		}

		// Expand: their public outgoing connections.
		conns, err := r.signerOutgoing(f.signer)
		if err != nil {
			return nil, err
		}
		for _, c := range conns {
			if distrusted[c.OtherID] {
				continue
			}
			// The next node's depth budget is min(f.depthLeft - 1, c.TrustExtends).
			nextDepth := f.depthLeft - 1
			if c.TrustExtends < nextDepth {
				nextDepth = c.TrustExtends
			}
			newPath := make([]string, len(f.path), len(f.path)+1)
			copy(newPath, f.path)
			newPath = append(newPath, c.OtherID)
			queue = append(queue, frontier{
				signer:    c.OtherID,
				path:      newPath,
				depthLeft: nextDepth,
				vetoOnly:  f.vetoOnly, // veto-only stickiness
			})
		}
	}
	return result, nil
}

// outgoingConn captures the parsed (other_id, trust_extends, public, trust)
// view of a signer's connections — the parts the trust walk cares about.
type outgoingConn struct {
	OtherID      string
	TrustExtends int
}

// signerOutgoing returns the active (non-revoked, trust=true,
// type=connection) outgoing edges declared by signer.
func (r *Resolver) signerOutgoing(signer string) ([]outgoingConn, error) {
	rows, err := r.store.GetCachedConnectionsBySigner(signer)
	if err != nil {
		return nil, fmt.Errorf("get connections for %s: %w", signer, err)
	}
	var out []outgoingConn
	for _, row := range rows {
		if row.Revoked {
			continue
		}
		var p models.Payload
		if err := json.Unmarshal([]byte(row.Payload), &p); err != nil {
			continue // skip malformed rows silently — sync should reject them
		}
		if p.Type != models.PayloadTypeConnection || !p.Trust {
			continue
		}
		out = append(out, outgoingConn{OtherID: p.OtherID, TrustExtends: p.TrustExtends})
	}
	return out, nil
}
