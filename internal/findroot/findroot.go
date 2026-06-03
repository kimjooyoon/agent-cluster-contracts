// Package findroot locates the contracts repo root by walking upward from the
// current working directory until it finds the sentinel file
// ssot-dependency-map.riido.json. This lets tools run from any subdirectory.
package findroot

import (
	"errors"
	"os"
	"path/filepath"
)

const sentinel = "ssot-dependency-map.riido.json"

// FromCWD walks upward from the current working directory.
func FromCWD() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return From(cwd)
}

// From walks upward from start.
func From(start string) (string, error) {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, sentinel)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("contracts repo root not found (no ssot-dependency-map.riido.json upward from " + start + ")")
		}
		dir = parent
	}
}
