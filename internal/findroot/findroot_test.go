package findroot

import (
	"os"
	"path/filepath"
	"testing"
)

func makeFakeContracts(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, sentinel), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func samePath(t *testing.T, a, b string) bool {
	t.Helper()
	aR, errA := filepath.EvalSymlinks(a)
	bR, errB := filepath.EvalSymlinks(b)
	if errA != nil || errB != nil {
		return a == b
	}
	return aR == bR
}

func TestFromCWDOrEnvUsesEnvWhenCWDWalkFails(t *testing.T) {
	fake := makeFakeContracts(t)
	t.Setenv(EnvVar, fake)

	// Chdir to a tempdir that has no contracts ancestry.
	outside := t.TempDir()
	prev, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := os.Chdir(outside); err != nil {
		t.Fatal(err)
	}

	got, err := FromCWDOrEnv()
	if err != nil {
		t.Fatalf("expected success via env, got %v", err)
	}
	if !samePath(t, got, fake) {
		t.Errorf("got %q, want %q", got, fake)
	}
}

func TestFromCWDOrEnvFailsWhenEnvUnsetAndCWDOutside(t *testing.T) {
	t.Setenv(EnvVar, "")

	outside := t.TempDir()
	prev, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := os.Chdir(outside); err != nil {
		t.Fatal(err)
	}

	_, err := FromCWDOrEnv()
	if err == nil {
		t.Fatal("expected error when env unset and CWD outside contracts")
	}
}

func TestFromCWDOrEnvFailsWhenEnvPointsAtNonContractsDir(t *testing.T) {
	notContracts := t.TempDir() // no sentinel
	t.Setenv(EnvVar, notContracts)

	outside := t.TempDir()
	prev, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := os.Chdir(outside); err != nil {
		t.Fatal(err)
	}

	_, err := FromCWDOrEnv()
	if err == nil {
		t.Fatal("expected error when env points at non-contracts dir")
	}
}

func TestFromCWDOrEnvPrefersCWDWhenAvailable(t *testing.T) {
	// When CWD walk succeeds, env var is ignored even if set to a different
	// location. Idempotent with old FromCWD behavior.
	cwdFake := makeFakeContracts(t)
	envFake := makeFakeContracts(t)
	t.Setenv(EnvVar, envFake)

	prev, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := os.Chdir(cwdFake); err != nil {
		t.Fatal(err)
	}

	got, err := FromCWDOrEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !samePath(t, got, cwdFake) {
		t.Errorf("expected CWD %q (preferred), got %q", cwdFake, got)
	}
}
