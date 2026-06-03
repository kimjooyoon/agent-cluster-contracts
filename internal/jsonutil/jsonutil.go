// Package jsonutil holds small JSON helpers used across tools. Kept tiny on
// purpose — when this grows beyond trivial helpers, write a decision record
// first.
package jsonutil

import (
	"encoding/json"
	"fmt"
	"os"
)

// ReadFile reads path and unmarshals into v.
func ReadFile(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

// WriteFile marshals v with 2-space indent and writes to path (truncating).
func WriteFile(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
