package sqliteconnector

import (
	"testing"

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
		Delegate:     false,
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
		got.Delegate != conn.Delegate ||
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
