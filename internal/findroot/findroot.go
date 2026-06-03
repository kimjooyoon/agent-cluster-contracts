// Package findroot locates the contracts repo root by walking upward from the
// current working directory until it finds the sentinel file
// ssot-dependency-map.riido.json. This lets tools run from any subdirectory.
//
// Decision 013 adds FromCWDOrEnv which falls back to the AGENT_CLUSTER_CONTRACTS
// env var when the CWD walk fails, so tools invoked from sibling repos
// (backend/, frontend/) can still locate the contracts checkout.
package findroot

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const sentinel = "ssot-dependency-map.riido.json"

// EnvVar is the environment variable consulted by FromCWDOrEnv when the
// upward CWD walk fails (decision 013).
const EnvVar = "AGENT_CLUSTER_CONTRACTS"

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

// FromCWDOrEnv tries FromCWD first; on failure, falls back to the directory
// pointed at by the AGENT_CLUSTER_CONTRACTS env var. The env var must point
// at an actual contracts checkout (containing the sentinel file) — otherwise
// an informative error is returned, never a wrong path.
//
// Use this for tools that may run from sibling repos (gen-go-client,
// gen-dart-client). Tools intended to run only inside contracts should
// continue using FromCWD.
func FromCWDOrEnv() (string, error) {
	if root, err := FromCWD(); err == nil {
		return root, nil
	}
	env := os.Getenv(EnvVar)
	if env == "" {
		return "", fmt.Errorf("contracts root not found (CWD walk failed and %s is unset; set it to your contracts checkout)", EnvVar)
	}
	abs, err := filepath.Abs(env)
	if err != nil {
		return "", fmt.Errorf("%s=%q: %w", EnvVar, env, err)
	}
	if _, err := os.Stat(filepath.Join(abs, sentinel)); err != nil {
		return "", fmt.Errorf("%s=%q: not a contracts checkout (no %s in that directory)", EnvVar, env, sentinel)
	}
	return abs, nil
}
