package commands

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// tillitConfigFilename is the project-config filename a check looks for
// in the target directory. Future versions may walk upward to find the
// nearest one; for now we look in the target dir only.
const tillitConfigFilename = ".tillit"

// tillitConfig is the on-disk shape of the .tillit YAML file. Fields
// are intentionally minimal — only add to the schema once a command
// has a concrete need for the value.
type tillitConfig struct {
	Ecosystem string `yaml:"ecosystem"`
}

// loadTillitConfig reads .tillit from dir. Missing file is not an
// error — returns (nil, nil) so callers can transparently fall back
// to CLI flags. Malformed YAML is an error so users notice typos
// rather than silently picking up no config.
func loadTillitConfig(dir string) (*tillitConfig, error) {
	path := filepath.Join(dir, tillitConfigFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var cfg tillitConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &cfg, nil
}
