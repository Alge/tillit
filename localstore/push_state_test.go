package localstore_test

import (
	"testing"
	"time"

	"github.com/Alge/tillit/localstore"
)

func TestRecordAndCheckPush(t *testing.T) {
	s := newTestStore(t)

	pushed, err := s.IsPushed("c-1", localstore.ItemConnection, "https://a.example.com")
	if err != nil {
		t.Fatalf("IsPushed failed: %v", err)
	}
	if pushed {
		t.Error("expected IsPushed=false before recording")
	}

	now := time.Now().UTC().Truncate(time.Second)
	if err := s.RecordPush("c-1", localstore.ItemConnection, "https://a.example.com", now); err != nil {
		t.Fatalf("RecordPush failed: %v", err)
	}

	pushed, err = s.IsPushed("c-1", localstore.ItemConnection, "https://a.example.com")
	if err != nil {
		t.Fatalf("IsPushed failed: %v", err)
	}
	if !pushed {
		t.Error("expected IsPushed=true after recording")
	}

	// Different server — still unpushed.
	pushed, _ = s.IsPushed("c-1", localstore.ItemConnection, "https://b.example.com")
	if pushed {
		t.Error("push to A must not imply push to B")
	}

	// Different item type — still unpushed.
	pushed, _ = s.IsPushed("c-1", localstore.ItemSignature, "https://a.example.com")
	if pushed {
		t.Error("connection push must not imply signature push for the same id")
	}
}

func TestRecordPush_Idempotent(t *testing.T) {
	s := newTestStore(t)

	now := time.Now().UTC().Truncate(time.Second)
	for i := 0; i < 3; i++ {
		if err := s.RecordPush("c-1", localstore.ItemConnection, "https://a.example.com", now); err != nil {
			t.Fatalf("RecordPush(%d) failed: %v", i, err)
		}
	}
	pushed, _ := s.IsPushed("c-1", localstore.ItemConnection, "https://a.example.com")
	if !pushed {
		t.Error("expected IsPushed=true")
	}
}
