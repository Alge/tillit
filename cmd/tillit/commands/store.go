package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Alge/tillit/localstore"
)

const storeName = ".tillit.db"

func openStore() (*localstore.Store, error) {
	dir, err := storeDir()
	if err != nil {
		return nil, err
	}
	return localstore.Init(filepath.Join(dir, storeName))
}

func storeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	dir := filepath.Join(home, ".tillit")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("cannot create %s: %w", dir, err)
	}
	return dir, nil
}
