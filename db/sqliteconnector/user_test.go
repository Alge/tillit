package sqliteconnector

import (
	"testing"

	"github.com/Alge/tillit/models"
)

func newTestConnector(t *testing.T) *SqliteConnector {
	t.Helper()
	c, err := Init(":memory:")
	if err != nil {
		t.Fatalf("failed creating test connector: %s", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestCreateUserTable(t *testing.T) {
	// Init already calls CreateUserTable; if the schema is broken this will fail
	newTestConnector(t)
}

func TestCreateAndGetUser(t *testing.T) {
	c := newTestConnector(t)

	u := &models.User{
		ID:       "test-id",
		Username: "alice",
		PubKey:   "test-pubkey",
		IsAdmin:  false,
	}

	if err := c.CreateUser(u); err != nil {
		t.Fatalf("CreateUser failed: %s", err)
	}

	got, err := c.GetUser("test-id")
	if err != nil {
		t.Fatalf("GetUser failed: %s", err)
	}

	if got.ID != u.ID || got.Username != u.Username || got.PubKey != u.PubKey {
		t.Errorf("got %+v, want %+v", got, u)
	}
}

func TestGetUserList(t *testing.T) {
	c := newTestConnector(t)

	for i, name := range []string{"alice", "bob", "carol"} {
		u := &models.User{ID: name + "-id", Username: name, PubKey: "key-" + name}
		_ = i
		if err := c.CreateUser(u); err != nil {
			t.Fatalf("CreateUser failed: %s", err)
		}
	}

	res, err := c.GetUserList(1, 10)
	if err != nil {
		t.Fatalf("GetUserList failed: %s", err)
	}

	if len(res.Data) != 3 {
		t.Errorf("expected 3 users, got %d", len(res.Data))
	}
}
