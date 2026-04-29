package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Alge/tillit/localstore"
)

// exportFormatVersion is bumped if the on-disk schema changes in a
// way that older imports can't read. Importers refuse formats they
// don't recognise.
const exportFormatVersion = 1

// exportDoc is the file format produced by `tillit export`.
//
// SECURITY: this document includes the user's private key bytes.
// Treat it the same way you'd treat the key itself — anyone with
// this file can sign as you. The CLI prints a warning before writing
// it; encryption is a future hardening.
type exportDoc struct {
	Version    int    `json:"version"`
	ExportedAt string `json:"exported_at"`
	ActiveKey  string `json:"active_key,omitempty"`

	Keys              []*localstore.Key              `json:"keys,omitempty"`
	Peers             []*localstore.Peer             `json:"peers,omitempty"`
	Servers           []*localstore.Server           `json:"servers,omitempty"`
	CachedSignatures  []*localstore.CachedSignature  `json:"cached_signatures,omitempty"`
	CachedConnections []*localstore.CachedConnection `json:"cached_connections,omitempty"`
	CachedUsers       []*localstore.CachedUser       `json:"cached_users,omitempty"`
	PushState         []*localstore.PushStateRow     `json:"push_state,omitempty"`
}

// Export writes a complete snapshot of the local store to a file.
// The output contains private key material — handle accordingly.
func Export(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: tillit export <file>")
	}
	path := args[0]

	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	doc, err := buildExportDoc(s)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create %q: %w", path, err)
	}
	defer f.Close()
	if err := writeExport(f, doc); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "WARNING: %s contains your private key. Anyone with this file can sign as you.\n", path)
	fmt.Printf("Exported %d key(s), %d peer(s), %d server(s), %d signature(s), %d connection(s) to %s\n",
		len(doc.Keys), len(doc.Peers), len(doc.Servers),
		len(doc.CachedSignatures), len(doc.CachedConnections), path)
	return nil
}

// Import reads a snapshot produced by Export and merges it into the
// local store. Conflicts are handled additively:
//
//   - Keys with names that already exist locally are skipped (the
//     local one wins) so you can't accidentally clobber an in-use key.
//   - Peers, servers, cached_users with conflicting IDs are skipped
//     (existing local rows kept).
//   - cached_signatures and cached_connections rely on the cache's
//     write-once semantics — duplicates are silently ignored.
//   - push_state is best-effort; conflicts skip the row.
//   - active_key is set from the import only when no active key is
//     currently configured.
func Import(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: tillit import <file>")
	}
	path := args[0]

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %q: %w", path, err)
	}
	defer f.Close()

	var doc exportDoc
	if err := json.NewDecoder(f).Decode(&doc); err != nil {
		return fmt.Errorf("parse %q: %w", path, err)
	}
	if doc.Version != exportFormatVersion {
		return fmt.Errorf("unsupported export format version %d (this build understands %d)",
			doc.Version, exportFormatVersion)
	}

	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	stats, err := applyImport(s, &doc)
	if err != nil {
		return err
	}
	fmt.Printf("Imported: %d key(s), %d peer(s), %d server(s), %d signature(s), %d connection(s)\n",
		stats.keys, stats.peers, stats.servers, stats.signatures, stats.connections)
	if stats.skipped > 0 {
		fmt.Printf("(%d row(s) skipped because a row with the same id/name was already present)\n", stats.skipped)
	}
	return nil
}

// buildExportDoc gathers every list-able piece of state into a single
// document.
func buildExportDoc(s *localstore.Store) (*exportDoc, error) {
	keys, err := s.ListKeys()
	if err != nil {
		return nil, fmt.Errorf("list keys: %w", err)
	}
	peers, err := s.ListPeers()
	if err != nil {
		return nil, fmt.Errorf("list peers: %w", err)
	}
	servers, err := s.ListServers()
	if err != nil {
		return nil, fmt.Errorf("list servers: %w", err)
	}
	sigs, err := s.ListAllCachedSignatures()
	if err != nil {
		return nil, fmt.Errorf("list signatures: %w", err)
	}
	conns, err := s.ListAllCachedConnections()
	if err != nil {
		return nil, fmt.Errorf("list connections: %w", err)
	}
	users, err := s.ListCachedUsers()
	if err != nil {
		return nil, fmt.Errorf("list cached users: %w", err)
	}
	push, err := s.ListAllPushState()
	if err != nil {
		return nil, fmt.Errorf("list push state: %w", err)
	}
	active, _ := s.GetActiveKey()

	return &exportDoc{
		Version:           exportFormatVersion,
		ExportedAt:        time.Now().UTC().Format(time.RFC3339),
		ActiveKey:         active,
		Keys:              keys,
		Peers:             peers,
		Servers:           servers,
		CachedSignatures:  sigs,
		CachedConnections: conns,
		CachedUsers:       users,
		PushState:         push,
	}, nil
}

func writeExport(w io.Writer, doc *exportDoc) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

type importStats struct {
	keys, peers, servers          int
	signatures, connections       int
	cachedUsers, pushState        int
	skipped                       int
}

func applyImport(s *localstore.Store, doc *exportDoc) (importStats, error) {
	var st importStats

	// Keys: skip on name conflict (local wins).
	for _, k := range doc.Keys {
		if _, err := s.GetKey(k.Name); err == nil {
			st.skipped++
			continue
		}
		if err := s.SaveKey(k); err != nil {
			return st, fmt.Errorf("import key %q: %w", k.Name, err)
		}
		st.keys++
	}

	// Active key: set only if not already set.
	if doc.ActiveKey != "" {
		if cur, _ := s.GetActiveKey(); cur == "" {
			if err := s.SetActiveKey(doc.ActiveKey); err != nil {
				return st, fmt.Errorf("set active key: %w", err)
			}
		}
	}

	// Peers / servers — try to add, count skips.
	for _, p := range doc.Peers {
		if existing, _ := s.GetPeer(p.ID); existing != nil {
			st.skipped++
			continue
		}
		if err := s.SavePeer(p); err != nil {
			return st, fmt.Errorf("import peer %s: %w", p.ID, err)
		}
		st.peers++
	}
	for _, srv := range doc.Servers {
		if existing, _ := s.GetServer(srv.URL); existing != nil {
			st.skipped++
			continue
		}
		if err := s.SaveServer(srv); err != nil {
			return st, fmt.Errorf("import server %s: %w", srv.URL, err)
		}
		st.servers++
	}

	// Cached rows — write-once semantics handle conflicts.
	for _, sig := range doc.CachedSignatures {
		if err := s.SaveCachedSignature(sig); err != nil {
			return st, fmt.Errorf("import signature %s: %w", sig.ID, err)
		}
		st.signatures++
	}
	for _, conn := range doc.CachedConnections {
		if err := s.SaveCachedConnection(conn); err != nil {
			return st, fmt.Errorf("import connection %s: %w", conn.ID, err)
		}
		st.connections++
	}
	for _, u := range doc.CachedUsers {
		if err := s.SaveCachedUser(u); err != nil {
			return st, fmt.Errorf("import cached user %s: %w", u.ID, err)
		}
		st.cachedUsers++
	}
	for _, p := range doc.PushState {
		if err := s.RecordPush(p.ItemID, p.ItemType, p.ServerURL, p.PushedAt); err != nil {
			return st, fmt.Errorf("import push_state: %w", err)
		}
		st.pushState++
	}
	return st, nil
}
