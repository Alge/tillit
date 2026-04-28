package sqliteconnector

import (
	"testing"
	"time"

	"github.com/Alge/tillit/models"
)

func TestCreateAndGetConnection(t *testing.T) {
	c := newTestConnector(t)

	conn := &models.Connection{
		ID:           "conn-1",
		Owner:        "user-a",
		OtherID:      "user-b",
		Public:       true,
		Trust:        true,
		TrustExtends: 2,
	}

	if err := c.CreateConnection(conn); err != nil {
		t.Fatalf("CreateConnection failed: %v", err)
	}

	got, err := c.GetConnection("conn-1")
	if err != nil {
		t.Fatalf("GetConnection failed: %v", err)
	}

	if got.ID != conn.ID ||
		got.Owner != conn.Owner ||
		got.OtherID != conn.OtherID ||
		got.Public != conn.Public ||
		got.Trust != conn.Trust ||
		got.TrustExtends != conn.TrustExtends {
		t.Errorf("got %+v, want %+v", got, conn)
	}
}

func TestGetUserConnections(t *testing.T) {
	c := newTestConnector(t)

	for i, other := range []string{"user-b", "user-c"} {
		conn := &models.Connection{
			ID:      "conn-" + other,
			Owner:   "user-a",
			OtherID: other,
			Trust:   i%2 == 0,
		}
		if err := c.CreateConnection(conn); err != nil {
			t.Fatalf("CreateConnection failed: %v", err)
		}
	}

	// Different owner — should not appear
	if err := c.CreateConnection(&models.Connection{ID: "conn-x", Owner: "user-b", OtherID: "user-c"}); err != nil {
		t.Fatalf("CreateConnection failed: %v", err)
	}

	conns, err := c.GetUserConnections("user-a")
	if err != nil {
		t.Fatalf("GetUserConnections failed: %v", err)
	}

	if len(conns) != 2 {
		t.Errorf("expected 2 connections for user-a, got %d", len(conns))
	}
}

func TestDeleteConnection(t *testing.T) {
	c := newTestConnector(t)

	conn := &models.Connection{ID: "conn-1", Owner: "user-a", OtherID: "user-b"}
	if err := c.CreateConnection(conn); err != nil {
		t.Fatalf("CreateConnection failed: %v", err)
	}

	if err := c.DeleteConnection(conn); err != nil {
		t.Fatalf("DeleteConnection failed: %v", err)
	}

	_, err := c.GetConnection("conn-1")
	if err == nil {
		t.Error("expected error after deleting connection, got nil")
	}
}

func TestGetConnectionNotFound(t *testing.T) {
	c := newTestConnector(t)
	_, err := c.GetConnection("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent connection")
	}
}

func TestGetUserPublicConnections_FiltersPrivateAndRevoked(t *testing.T) {
	c := newTestConnector(t)
	now := time.Now().UTC().Truncate(time.Second)

	conns := []*models.Connection{
		{ID: "c-pub", Owner: "alice", OtherID: "bob", Public: true, Trust: true, CreatedAt: now},
		{ID: "c-priv", Owner: "alice", OtherID: "carol", Public: false, Trust: true, CreatedAt: now},
		{ID: "c-rev", Owner: "alice", OtherID: "dave", Public: true, Trust: true, CreatedAt: now},
	}
	for _, conn := range conns {
		if err := c.CreateConnection(conn); err != nil {
			t.Fatalf("CreateConnection failed: %v", err)
		}
	}
	if err := c.RevokeConnection("c-rev", now); err != nil {
		t.Fatalf("RevokeConnection failed: %v", err)
	}

	got, err := c.GetUserPublicConnections("alice", nil)
	if err != nil {
		t.Fatalf("GetUserPublicConnections failed: %v", err)
	}
	if len(got) != 1 || got[0].ID != "c-pub" {
		t.Errorf("expected only [c-pub], got %v", got)
	}
}

func TestGetUserPublicConnections_SinceFilter(t *testing.T) {
	c := newTestConnector(t)
	t1 := time.Now().UTC().Truncate(time.Second)
	t2 := t1.Add(time.Minute)

	if err := c.CreateConnection(&models.Connection{
		ID: "c-old", Owner: "alice", OtherID: "bob", Public: true, Trust: true, CreatedAt: t1,
	}); err != nil {
		t.Fatalf("CreateConnection failed: %v", err)
	}
	if err := c.CreateConnection(&models.Connection{
		ID: "c-new", Owner: "alice", OtherID: "carol", Public: true, Trust: true, CreatedAt: t2,
	}); err != nil {
		t.Fatalf("CreateConnection failed: %v", err)
	}

	got, err := c.GetUserPublicConnections("alice", &t1)
	if err != nil {
		t.Fatalf("GetUserPublicConnections failed: %v", err)
	}
	if len(got) != 1 || got[0].ID != "c-new" {
		t.Errorf("expected only [c-new], got %+v", got)
	}
}

func TestRevokeConnection_NotFound(t *testing.T) {
	c := newTestConnector(t)
	err := c.RevokeConnection("nope", time.Now())
	if err == nil {
		t.Error("expected error for unknown connection")
	}
}
