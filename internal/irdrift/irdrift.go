// Package irdrift re-runs the SBCL DSL → JSON IR emitter into a temp tree and
// compares the result to the committed ir/ tree. Any difference fails. This is
// the executable guard for concept-map constraint C-002.
package irdrift

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

// Result is the outcome of one drift check.
type Result struct {
	Differs       []Diff
	MissingInTemp []string // present in committed but not regenerated
	ExtraInTemp   []string // regenerated a file that isn't committed
	SBCLOutput    string
}

// Diff is a single byte-level disagreement.
type Diff struct {
	Path        string
	Committed   []byte
	Regenerated []byte
}

// Check regenerates IR into a temp dir and compares it to root/ir/.
// It rewrites the emitter's relative output so it lands in tmpdir/ir/...
// rather than touching the working tree.
func Check(root string) (*Result, error) {
	tmp, err := os.MkdirTemp("", "irdrift-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	// The Lisp emitter records each DSL file's path relative to "/contracts/"
	// in the source.dsl_file field. To make that path stable across the
	// committed run and this temp run, we nest the working directory under a
	// "contracts/" subdir of tmp.
	workdir := filepath.Join(tmp, "contracts")
	if err := copyDir(filepath.Join(root, "dsl"), filepath.Join(workdir, "dsl")); err != nil {
		return nil, fmt.Errorf("copy dsl: %w", err)
	}

	cmd := exec.Command("sbcl", "--script", "dsl/emit-ir.lisp")
	cmd.Dir = workdir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return &Result{SBCLOutput: out.String()},
			fmt.Errorf("sbcl emit failed: %w (output: %s)", err, out.String())
	}

	r := &Result{SBCLOutput: out.String()}

	committedIR := filepath.Join(root, "ir")
	regenIR := filepath.Join(workdir, "ir")

	// Walk committed IR; compare each file.
	seen := map[string]bool{}
	err = filepath.WalkDir(committedIR, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		// Only diff generated files. README.md and other docs in ir/ are not regenerated.
		rel, err := filepath.Rel(committedIR, path)
		if err != nil {
			return err
		}
		if !isGeneratedIR(rel) {
			return nil
		}
		regen := filepath.Join(regenIR, rel)
		seen[rel] = true
		regenBytes, err := os.ReadFile(regen)
		if err != nil {
			if os.IsNotExist(err) {
				r.MissingInTemp = append(r.MissingInTemp, rel)
				return nil
			}
			return err
		}
		commBytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Equal(commBytes, regenBytes) {
			r.Differs = append(r.Differs, Diff{Path: rel, Committed: commBytes, Regenerated: regenBytes})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Walk regenerated IR; flag anything we generated but didn't commit.
	err = filepath.WalkDir(regenIR, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(regenIR, path)
		if err != nil {
			return err
		}
		if !isGeneratedIR(rel) {
			return nil
		}
		if !seen[rel] {
			r.ExtraInTemp = append(r.ExtraInTemp, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r, nil
}

// isGeneratedIR returns true for files this emitter is expected to produce.
// Currently: ir/domain/*.ir.json.
func isGeneratedIR(rel string) bool {
	dir, name := filepath.Split(rel)
	if filepath.Clean(dir) != "domain" {
		return false
	}
	return len(name) > len(".ir.json") &&
		name[len(name)-len(".ir.json"):] == ".ir.json"
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
