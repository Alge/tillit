package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTillitConfig_Missing(t *testing.T) {
	dir := t.TempDir()
	cfg, err := loadTillitConfig(dir)
	if err != nil {
		t.Fatalf("loadTillitConfig: %v", err)
	}
	if cfg != nil {
		t.Errorf("expected nil config when file missing, got %+v", cfg)
	}
}

func TestLoadTillitConfig_ReadsEcosystem(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".tillit"), []byte("ecosystem: go\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := loadTillitConfig(dir)
	if err != nil {
		t.Fatalf("loadTillitConfig: %v", err)
	}
	if cfg == nil || cfg.Ecosystem != "go" {
		t.Errorf("expected ecosystem 'go', got %+v", cfg)
	}
}

func TestLoadTillitConfig_RejectsMalformed(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".tillit"), []byte("ecosystem: [unclosed"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := loadTillitConfig(dir)
	if err == nil {
		t.Error("expected error on malformed YAML")
	}
}

func TestLoadTillitConfig_EmptyFileIsValid(t *testing.T) {
	// An empty .tillit is fine — fields all zero, callers fall back.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".tillit"), []byte(""), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := loadTillitConfig(dir)
	if err != nil {
		t.Fatalf("loadTillitConfig: %v", err)
	}
	if cfg == nil {
		t.Error("expected non-nil config for empty file")
	}
	if cfg.Ecosystem != "" {
		t.Errorf("expected empty ecosystem, got %q", cfg.Ecosystem)
	}
}
