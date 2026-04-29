package commands

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	tillit_crypto "github.com/Alge/tillit/crypto"
	"github.com/Alge/tillit/localstore"
	"github.com/Alge/tillit/resolver"
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

// exportScope is how much cached-row data to include.
type exportScope int

const (
	// scopeSelf is the default — only the chosen identity's own
	// signatures and connections.
	scopeSelf exportScope = iota
	// scopeIncludePeers extends the export to every signer reachable
	// from the chosen identity through the trust graph (direct peers
	// plus transitive ones, bounded by their TrustDepth/TrustExtends).
	scopeIncludePeers
	// scopeAll dumps every row in the local store regardless of
	// signer.
	scopeAll
)

// Export writes a snapshot of the local store to a file.
//
// Two orthogonal axes:
//
//   - Which keys: default (active key only), --key <name> (a specific
//     stored key), or --all (every key, plus every cached row for a
//     full backup). --key and --all are mutually exclusive.
//   - Which cached rows for that identity: default (own only) or
//     --include-peers (also rows by every signer reachable in the
//     identity's trust graph). --include-peers combines with --key
//     or the default; with --all it's redundant (--all already
//     includes everything).
//
// The output contains private key material — handle accordingly.
func Export(args []string) error {
	scope := scopeSelf
	dumpAll := false
	keyName := ""
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--all" || a == "-a":
			dumpAll = true
		case a == "--include-peers":
			scope = scopeIncludePeers
		case a == "--help" || a == "-h":
			fmt.Fprintln(os.Stderr, "usage: tillit export [--all | --key <name>] [--include-peers] <file>")
			return nil
		case a == "--key" || a == "-k":
			if i+1 >= len(args) {
				return fmt.Errorf("%s requires a value", a)
			}
			keyName = args[i+1]
			i++
		case strings.HasPrefix(a, "--key="):
			keyName = strings.TrimPrefix(a, "--key=")
		case strings.HasPrefix(a, "-"):
			return fmt.Errorf("unknown flag %q", a)
		default:
			positional = append(positional, a)
		}
	}
	if dumpAll && keyName != "" {
		return fmt.Errorf("--all and --key are mutually exclusive — --all dumps every key, --key picks one")
	}
	if dumpAll {
		// Full backup overrides any cached-row scope flag.
		scope = scopeAll
	}
	if len(positional) != 1 {
		return fmt.Errorf("usage: tillit export [--all | --key <name>] [--include-peers] <file>")
	}
	path := positional[0]

	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	doc, err := buildExportDoc(s, scope, keyName)
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

// buildExportDoc gathers state into a snapshot.
//
// Scope:
//   - scopeSelf: cached signatures/connections only for the chosen
//     identity; cached_users excluded.
//   - scopeIncludePeers: also includes rows by every signer reachable
//     in the chosen identity's trust graph, plus the cached_users
//     pubkey cache for those signers (so the recipient can verify
//     signatures offline).
//   - scopeAll: every row in every cache table, regardless of signer.
//
// Identity selection: keyName="" picks the active key (and includes
// every stored key in the export). A specific keyName picks that one
// key as the identity AND restricts the keys array to just that one
// — the use case is "give a colleague this single identity."
func buildExportDoc(s *localstore.Store, scope exportScope, keyName string) (*exportDoc, error) {
	allKeys, err := s.ListKeys()
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
	allSigs, err := s.ListAllCachedSignatures()
	if err != nil {
		return nil, fmt.Errorf("list signatures: %w", err)
	}
	allConns, err := s.ListAllCachedConnections()
	if err != nil {
		return nil, fmt.Errorf("list connections: %w", err)
	}
	push, err := s.ListAllPushState()
	if err != nil {
		return nil, fmt.Errorf("list push state: %w", err)
	}

	exportKeys := allKeys
	identityID := ""
	activeName := ""
	if keyName != "" {
		k, err := s.GetKey(keyName)
		if err != nil {
			return nil, fmt.Errorf("--key %q: %w", keyName, err)
		}
		uid, err := userIDFromKey(k)
		if err != nil {
			return nil, err
		}
		identityID = uid
		exportKeys = []*localstore.Key{k}
		activeName = keyName
	} else {
		activeName, _ = s.GetActiveKey()
		if activeName != "" {
			if k, err := s.GetKey(activeName); err == nil {
				identityID, _ = userIDFromKey(k)
			}
		}
	}

	doc := &exportDoc{
		Version:    exportFormatVersion,
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		ActiveKey:  activeName,
		Keys:       exportKeys,
		Peers:      peers,
		Servers:    servers,
		PushState:  push,
	}

	switch scope {
	case scopeAll:
		users, err := s.ListCachedUsers()
		if err != nil {
			return nil, fmt.Errorf("list cached users: %w", err)
		}
		doc.CachedSignatures = allSigs
		doc.CachedConnections = allConns
		doc.CachedUsers = users
		return doc, nil

	case scopeIncludePeers:
		if identityID == "" {
			return doc, nil
		}
		// Walk the trust graph rooted at the chosen identity.
		r := resolver.New(s, identityID)
		entries, err := r.TrustSet(identityID)
		if err != nil {
			return nil, fmt.Errorf("walk trust set: %w", err)
		}
		signers := make(map[string]bool, len(entries))
		for _, e := range entries {
			signers[e.SignerID] = true
		}
		for _, sig := range allSigs {
			if signers[sig.Signer] {
				doc.CachedSignatures = append(doc.CachedSignatures, sig)
			}
		}
		for _, c := range allConns {
			if signers[c.Signer] {
				doc.CachedConnections = append(doc.CachedConnections, c)
			}
		}
		// Pubkey cache for the trust set so the recipient can verify
		// signatures offline.
		users, err := s.ListCachedUsers()
		if err != nil {
			return nil, fmt.Errorf("list cached users: %w", err)
		}
		for _, u := range users {
			if signers[u.ID] {
				doc.CachedUsers = append(doc.CachedUsers, u)
			}
		}
		return doc, nil

	default: // scopeSelf
		if identityID == "" {
			return doc, nil
		}
		for _, sig := range allSigs {
			if sig.Signer == identityID {
				doc.CachedSignatures = append(doc.CachedSignatures, sig)
			}
		}
		for _, c := range allConns {
			if c.Signer == identityID {
				doc.CachedConnections = append(doc.CachedConnections, c)
			}
		}
		return doc, nil
	}
}

// userIDFromKey derives a user id from a stored key the same way
// activeSignerAndID does — sha256 of the pubkey bytes, base64url
// encoded — but without requiring the key to be the active one.
func userIDFromKey(k *localstore.Key) (string, error) {
	privBytes, err := base64.RawURLEncoding.DecodeString(k.PrivKey)
	if err != nil {
		return "", fmt.Errorf("invalid stored private key %q: %w", k.Name, err)
	}
	signer, err := tillit_crypto.LoadSigner(k.Algorithm, privBytes)
	if err != nil {
		return "", fmt.Errorf("load signer for %q: %w", k.Name, err)
	}
	hash := sha256.Sum256(signer.PublicKey())
	return base64.RawURLEncoding.EncodeToString(hash[:]), nil
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
