package irdrift

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// D041 — internal/irdrift tests. Check() shells out to SBCL so the
// integration test is skipped when sbcl isn't installed; the pure-Go
// helpers (isGeneratedIR, copyDir, copyFile) are always tested.

func TestIsGeneratedIR(t *testing.T) {
	cases := []struct {
		rel  string
		want bool
	}{
		// Positive: anything under ir/domain/ ending in .ir.json
		{"domain/work-item.ir.json", true},
		{"domain/list-work-items.ir.json", true},
		{"domain/x.ir.json", true},
		// Negative: wrong extension
		{"domain/work-item.json", false},
		{"domain/README.md", false},
		// Negative: wrong directory
		{"schema/ir.schema.json", false},
		{"work-item.ir.json", false},               // top-level, no domain/ prefix
		{"domain/sub/work-item.ir.json", false},    // nested under domain/, not directly
		// Edge: exact suffix .ir.json without basename
		{"domain/.ir.json", false}, // empty basename
	}
	for _, tc := range cases {
		t.Run(tc.rel, func(t *testing.T) {
			if got := isGeneratedIR(tc.rel); got != tc.want {
				t.Errorf("isGeneratedIR(%q) = %v, want %v", tc.rel, got, tc.want)
			}
		})
	}
}

func TestCopyDirRoundTrip(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Build a small tree: a/b/leaf.txt, a/sibling.txt, top.txt.
	files := map[string]string{
		"top.txt":          "alpha",
		"a/sibling.txt":    "beta",
		"a/b/leaf.txt":     "gamma\nline2\n",
		"a/b/c/d/deep.txt": "ε",
	}
	for rel, content := range files {
		p := filepath.Join(src, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir: %v", err)
	}

	for rel, want := range files {
		got, err := os.ReadFile(filepath.Join(dst, rel))
		if err != nil {
			t.Errorf("%s: read after copy: %v", rel, err)
			continue
		}
		if string(got) != want {
			t.Errorf("%s: content mismatch — got %q, want %q", rel, got, want)
		}
	}
}

func TestCopyDirCreatesEmptyDirectories(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	// Empty directory should be replicated as an empty dir.
	if err := os.MkdirAll(filepath.Join(src, "empty/deeper"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := copyDir(src, dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "empty/deeper")); err != nil {
		t.Errorf("expected empty/deeper to exist in dst: %v", err)
	}
}

func TestCopyFileWritesBytesExactly(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src.bin")
	dst := filepath.Join(tmp, "out/dst.bin")
	want := []byte{0x00, 0x01, 0xFF, 'a', '\n', 0x7F}
	if err := os.WriteFile(src, want, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("copyFile produced %v, want %v", got, want)
	}
}

func TestCopyFileCreatesIntermediateDirectories(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "in.txt")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Deeply nested dst; intermediate dirs must be created automatically.
	dst := filepath.Join(tmp, "a/b/c/d/out.txt")
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile: %v", err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Errorf("dst not created: %v", err)
	}
}

// TestCheckIntegrationWithSBCL exercises Check() end-to-end. Skipped if
// sbcl isn't installed (e.g. on machines without Common Lisp toolchain).
// CI runners install sbcl as a contracts.yml step, so this runs there.
func TestCheckIntegrationWithSBCL(t *testing.T) {
	if _, err := exec.LookPath("sbcl"); err != nil {
		t.Skip("sbcl not available; skipping irdrift integration test")
	}
	// Use the real contracts root: walk up from this test file until we find
	// the dep map at the repo root.
	root, err := findRepoRoot()
	if err != nil {
		t.Skipf("repo root not found: %v", err)
	}
	res, err := Check(root)
	if err != nil {
		t.Fatalf("Check(real root): %v", err)
	}
	if len(res.Differs) != 0 || len(res.MissingInTemp) != 0 || len(res.ExtraInTemp) != 0 {
		t.Errorf("Check on clean working tree should report no drift; got differs=%d missing=%d extra=%d (sbcl output: %s)",
			len(res.Differs), len(res.MissingInTemp), len(res.ExtraInTemp), res.SBCLOutput)
	}
}

// findRepoRoot walks up from the current test directory looking for the
// dep-map file. Mirrors internal/findroot semantics without taking the
// import (which would require sibling-package setup).
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "ssot-dependency-map.riido.json")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

// Sanity: confirm copyDir errors propagate when src doesn't exist.
func TestCopyDirSrcMissingReturnsError(t *testing.T) {
	err := copyDir("/nonexistent/path/that/does/not/exist", t.TempDir())
	if err == nil {
		t.Errorf("expected error for missing src, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent") && !strings.Contains(err.Error(), "no such") {
		t.Logf("error message: %v (acceptable as long as non-nil)", err)
	}
}
