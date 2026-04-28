package localstore_test

import (
	"testing"

	"github.com/Alge/tillit/localstore"
)

func TestSaveAndGetServer(t *testing.T) {
	s := newTestStore(t)

	srv := &localstore.Server{
		URL:    "https://tillit.example.com",
		Alias:  "example",
		UserID: "abc123",
	}
	if err := s.SaveServer(srv); err != nil {
		t.Fatalf("SaveServer failed: %v", err)
	}

	got, err := s.GetServer("https://tillit.example.com")
	if err != nil {
		t.Fatalf("GetServer failed: %v", err)
	}
	if got.URL != srv.URL || got.Alias != srv.Alias || got.UserID != srv.UserID {
		t.Errorf("got %+v, want %+v", got, srv)
	}
}

func TestGetServer_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetServer("https://nope.example.com")
	if err == nil {
		t.Error("expected error for nonexistent server")
	}
}

func TestListServers(t *testing.T) {
	s := newTestStore(t)

	for _, url := range []string{"https://a.example.com", "https://b.example.com"} {
		s.SaveServer(&localstore.Server{URL: url, UserID: "u1"})
	}

	servers, err := s.ListServers()
	if err != nil {
		t.Fatalf("ListServers failed: %v", err)
	}
	if len(servers) != 2 {
		t.Errorf("expected 2 servers, got %d", len(servers))
	}
}

func TestSaveAndGetPeer(t *testing.T) {
	s := newTestStore(t)

	peer := &localstore.Peer{
		ID:           "abc123",
		ServerURL:    "https://tillit.example.com",
		TrustDepth:   2,
		Public:     true,
		Distrusted:   false,
	}
	if err := s.SavePeer(peer); err != nil {
		t.Fatalf("SavePeer failed: %v", err)
	}

	got, err := s.GetPeer("abc123")
	if err != nil {
		t.Fatalf("GetPeer failed: %v", err)
	}
	if got.ID != peer.ID || got.ServerURL != peer.ServerURL ||
		got.TrustDepth != peer.TrustDepth || got.Public != peer.Public ||
		got.Distrusted != peer.Distrusted {
		t.Errorf("got %+v, want %+v", got, peer)
	}
}

func TestGetPeer_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetPeer("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent peer")
	}
}

func TestListPeers(t *testing.T) {
	s := newTestStore(t)

	s.SavePeer(&localstore.Peer{ID: "u1", ServerURL: "https://a.example.com", TrustDepth: 1})
	s.SavePeer(&localstore.Peer{ID: "u2", ServerURL: "https://b.example.com", TrustDepth: 1})
	// distrusted peer
	s.SavePeer(&localstore.Peer{ID: "u3", ServerURL: "https://c.example.com", Distrusted: true})

	all, err := s.ListPeers()
	if err != nil {
		t.Fatalf("ListPeers failed: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 peers total, got %d", len(all))
	}
}

func TestRemovePeer(t *testing.T) {
	s := newTestStore(t)

	s.SavePeer(&localstore.Peer{ID: "u1", ServerURL: "https://a.example.com", TrustDepth: 1})
	if err := s.RemovePeer("u1"); err != nil {
		t.Fatalf("RemovePeer failed: %v", err)
	}
	_, err := s.GetPeer("u1")
	if err == nil {
		t.Error("expected error after removing peer")
	}
}
