package config

import "testing"

func TestServerValidate_EmptyHostname(t *testing.T) {
	s := server{HostName: "", Port: 8080}
	if err := s.Validate(); err == nil {
		t.Error("expected error for empty hostname")
	}
}

func TestServerValidate_InvalidPort(t *testing.T) {
	s := server{HostName: "localhost", Port: 0}
	if err := s.Validate(); err == nil {
		t.Error("expected error for port 0")
	}
}

func TestServerValidate_Valid(t *testing.T) {
	s := server{HostName: "localhost", Port: 8080}
	if err := s.Validate(); err != nil {
		t.Errorf("unexpected error: %s", err)
	}
}

func TestDatabaseValidate_UnknownType(t *testing.T) {
	d := database{Type: "postgres", DSN: "some-dsn"}
	if err := d.Validate(); err == nil {
		t.Error("expected error for unknown db type")
	}
}
