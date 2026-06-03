// Package secretscan walks a directory tree and flags any file that contains a
// pattern matching a known credential shape. Patterns are conservative — chosen
// to have low false-positive rates rather than to catch every possible secret.
//
// To intentionally include a string that would otherwise match (for example,
// in tests or documentation), append the marker "secretscan:ignore" anywhere
// on the same line.
package secretscan

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Pattern is a named regex.
type Pattern struct {
	Name    string
	Regex   *regexp.Regexp
}

// Finding is a single match.
type Finding struct {
	Path    string
	Line    int
	Pattern string
	Excerpt string
}

// DefaultPatterns covers AWS, GitHub, Slack, JWT, PEM, Google API. Add
// patterns only with a decision record explaining the false-positive analysis.
//
// Pattern source strings are written so they cannot match themselves (the
// brackets in `[A-Z]` are literal in the source, not a match).
func DefaultPatterns() []Pattern {
	return []Pattern{
		{Name: "aws-access-key", Regex: regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
		{Name: "github-pat",     Regex: regexp.MustCompile(`\b(ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{30,255}\b`)},
		{Name: "github-fine",    Regex: regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{60,}\b`)},
		{Name: "slack-token",    Regex: regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}\b`)},
		{Name: "jwt",            Regex: regexp.MustCompile(`\beyJ[A-Za-z0-9_=\-]{8,}\.eyJ[A-Za-z0-9_=\-]{8,}\.[A-Za-z0-9_=\-]{8,}\b`)},
		{Name: "google-api-key", Regex: regexp.MustCompile(`\bAIza[0-9A-Za-z\-_]{35}\b`)},
		{Name: "pem-private",    Regex: regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
		{Name: "anthropic-key",  Regex: regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_\-]{20,}\b`)},
		{Name: "openai-key",     Regex: regexp.MustCompile(`\bsk-[A-Za-z0-9]{20,}\b`)},
	}
}

const ignoreMarker = "secretscan:ignore"

var defaultSkipDirs = map[string]bool{
	".git":         true,
	".github":      false, // workflows must be scanned
	"bin":          true,
	"vendor":       true,
	"node_modules": true,
	".knowledge":   true,
	".devpattern":  true,
	".dart_tool":   true,
	"build":        true,
	".gradle":      true,
	"Pods":         true,
}

// Skip controls which paths are excluded from scanning.
type Skip struct {
	// Paths that, when matched as a path suffix, exclude that file (used to
	// keep secretscan from scanning its own source files which contain the
	// patterns as regex literals).
	SuffixSkips []string
}

func DefaultSkip() Skip {
	return Skip{
		SuffixSkips: []string{
			filepath.Join("internal", "secretscan", "secretscan.go"),
			filepath.Join("tools", "secretscan", "main.go"),
		},
	}
}

// Scan walks root and returns findings. Symlinks are not followed.
func Scan(root string, patterns []Pattern, skip Skip) ([]Finding, error) {
	var out []Finding
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if defaultSkipDirs[name] {
				return filepath.SkipDir
			}
			return nil
		}
		for _, suf := range skip.SuffixSkips {
			if strings.HasSuffix(path, suf) {
				return nil
			}
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > 1<<20 { // 1 MiB cap; binaries/lockfiles skipped
			return nil
		}
		if isBinaryByExt(name) {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		lineNo := 0
		for sc.Scan() {
			lineNo++
			line := sc.Text()
			if strings.Contains(line, ignoreMarker) {
				continue
			}
			for _, p := range patterns {
				if loc := p.Regex.FindStringIndex(line); loc != nil {
					out = append(out, Finding{
						Path:    path,
						Line:    lineNo,
						Pattern: p.Name,
						Excerpt: redactExcerpt(line, loc[0], loc[1]),
					})
				}
			}
		}
		return sc.Err()
	})
	return out, err
}

func redactExcerpt(line string, start, end int) string {
	if end-start <= 8 {
		return strings.Repeat("*", end-start)
	}
	return line[start:start+4] + strings.Repeat("*", end-start-4)
}

var binaryExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".pdf": true,
	".woff": true, ".woff2": true, ".ttf": true, ".eot": true,
	".zip": true, ".gz": true, ".tar": true, ".tgz": true,
	".so": true, ".dylib": true, ".dll": true, ".exe": true,
	".class": true, ".jar": true, ".o": true, ".a": true,
	".ico": true, ".icns": true,
}

func isBinaryByExt(name string) bool {
	return binaryExt[strings.ToLower(filepath.Ext(name))]
}
