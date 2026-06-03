package secretscan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeAWS is an AKIA-pattern AWS key value, constructed at runtime so the
// literal does not appear in the source (which would trip GitHub Push
// Protection). Used as a stand-in across tests.
var fakeAWS = "AK" + "IA1234567890ABCDEF"


func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// D039: secretscan unit tests. The patterns guard real secrets; their
// regexes must catch positive cases AND not over-trigger on innocent
// strings (false positives would block legitimate PRs). These tests
// exercise each pattern with one positive + one or more negatives.

func TestPatternsMatchKnownSecrets(t *testing.T) {
	// All test inputs are CONSTRUCTED at runtime via string concatenation
	// so the literal bytes never appear in the source file. This avoids
	// triggering GitHub Push Protection and other repository-side secret
	// scanners that scan source text for canonical secret formats.
	akiaPrefix := "AK" + "IA" // split so the literal "AKIA" never appears as a substring of the source
	ghpPrefix := "gh" + "p_"
	ghPatPrefix := "github_" + "pat_"
	xoxPrefix := "xo" + "xb-"
	ej := "ey" + "J"
	aizaPrefix := "AI" + "za"
	pemHdr := "-----BEGIN RSA PRIVATE " + "KEY-----"
	pemCertHdr := "-----BEGIN CERTIFI" + "CATE-----"
	skAnt := "sk-" + "ant-"
	skO := "sk" + "-"

	cases := []struct {
		patternName string
		input       string
		wantMatch   bool
	}{
		// aws-access-key: AKIA + 16 uppercase alphanumeric
		{"aws-access-key", "key=" + akiaPrefix + "ABCDEFGHIJKLMNOP here", true},
		{"aws-access-key", "key=" + akiaPrefix + "abcdefghijklmnop here", false}, // lowercase fails
		{"aws-access-key", "key=" + akiaPrefix + "12345 here", false},            // too short
		// github-pat: ghp_/gho_/ghu_/ghs_/ghr_ + 30+ alphanumeric
		{"github-pat", "token=" + ghpPrefix + strings.Repeat("a", 35) + " end", true},
		{"github-pat", "token=" + ghpPrefix + "short", false},
		// github-fine: github_pat_ + 60+
		{"github-fine", "token=" + ghPatPrefix + strings.Repeat("a", 70) + " end", true},
		{"github-fine", "token=" + ghPatPrefix + "short", false},
		// slack-token: xoxa-/xoxb-/xoxp-/xoxr-/xoxs- + 10+
		{"slack-token", "tok=" + xoxPrefix + strings.Repeat("a", 15) + " end", true},
		{"slack-token", "tok=xo" + "xz-something", false},
		// jwt: 3 dot-separated base64-ish parts each starting with eyJ
		{"jwt", "auth=" + ej + "abcdefgh." + ej + "abcdefgh.signature99 end", true},
		{"jwt", "auth=" + ej + "tooshort end", false},
		// google-api-key: AIza + 35 url-safe chars
		{"google-api-key", "key=" + aizaPrefix + strings.Repeat("a", 35) + " end", true},
		{"google-api-key", "key=" + aizaPrefix + "_too_short", false},
		// pem-private: header literal
		{"pem-private", pemHdr + "\nMIIEow", true},
		{"pem-private", pemCertHdr + "\nfoo", false},
		// anthropic-key: sk-ant- + 20+
		{"anthropic-key", "key=" + skAnt + strings.Repeat("a", 25) + " end", true},
		{"anthropic-key", "key=" + skAnt + "shrt", false},
		// openai-key: sk- + 20+ alphanumeric
		{"openai-key", "key=" + skO + strings.Repeat("a", 25) + " end", true},
		{"openai-key", "key=" + skO + "short", false},
	}
	patterns := map[string]Pattern{}
	for _, p := range DefaultPatterns() {
		patterns[p.Name] = p
	}
	for _, tc := range cases {
		t.Run(tc.patternName+"_"+tc.input[:min(len(tc.input), 30)], func(t *testing.T) {
			p, ok := patterns[tc.patternName]
			if !ok {
				t.Fatalf("pattern %q not in DefaultPatterns", tc.patternName)
			}
			match := p.Regex.MatchString(tc.input)
			if match != tc.wantMatch {
				t.Errorf("pattern=%s input=%q: match=%v, want %v", tc.patternName, tc.input, match, tc.wantMatch)
			}
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestScanFindsSecretInFile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "config.txt"),
		"db_user=admin\naws_key=" + fakeAWS + "\nport=5432\n")
	findings, err := Scan(root, DefaultPatterns(), DefaultSkip())
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	f := findings[0]
	if f.Pattern != "aws-access-key" {
		t.Errorf("expected aws-access-key match, got %s", f.Pattern)
	}
	if f.Line != 2 {
		t.Errorf("expected line 2, got %d", f.Line)
	}
	// Excerpt redacts via * chars (verified separately in TestRedactedExcerptHidesTheSecret).
	if strings.Contains(f.Excerpt, fakeAWS) {
		t.Errorf("excerpt must not contain raw secret, got %q", f.Excerpt)
	}
}

func TestScanRespectsIgnoreMarker(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "fixture.txt"),
		"example=" + fakeAWS + " // secretscan:ignore (test data)\n")
	findings, _ := Scan(root, DefaultPatterns(), DefaultSkip())
	if len(findings) != 0 {
		t.Errorf("expected 0 findings (line has secretscan:ignore), got %v", findings)
	}
}

func TestScanSkipsBinaryFilesByExtension(t *testing.T) {
	root := t.TempDir()
	// Even binary-extension files with a fake secret payload shouldn't be scanned.
	writeFile(t, filepath.Join(root, "image.png"),
		"binary header " + fakeAWS + " more bytes")
	findings, _ := Scan(root, DefaultPatterns(), DefaultSkip())
	if len(findings) != 0 {
		t.Errorf("binary by extension must be skipped, got %v", findings)
	}
}

func TestScanSkipsDefaultDirs(t *testing.T) {
	root := t.TempDir()
	// .git/ contents must never be scanned (would catch all kinds of stuff
	// from refs and remotes).
	writeFile(t, filepath.Join(root, ".git/config"),
		"url=" + fakeAWS + "\n")
	writeFile(t, filepath.Join(root, "node_modules/secret.txt"),
		"key=" + fakeAWS + "\n")
	findings, _ := Scan(root, DefaultPatterns(), DefaultSkip())
	if len(findings) != 0 {
		t.Errorf("expected 0 findings (.git and node_modules skipped), got %v", findings)
	}
}

func TestScanRespectsSuffixSkips(t *testing.T) {
	// Files matching SuffixSkips entries are excluded — this is how
	// secretscan avoids scanning its own source code (which contains
	// the patterns as regex literals).
	root := t.TempDir()
	target := filepath.Join(root, "internal/secretscan/secretscan.go")
	writeFile(t, target, "regexp.MustCompile(`" + "AK"+"IA[0-9A-Z]{16}" + "`)\n")
	findings, _ := Scan(root, DefaultPatterns(), DefaultSkip())
	if len(findings) != 0 {
		t.Errorf("internal/secretscan/secretscan.go must be SuffixSkipped, got %v", findings)
	}
}

func TestScanIgnoresOversizedFiles(t *testing.T) {
	// Files larger than 1 MiB are skipped (likely binaries / lockfiles).
	root := t.TempDir()
	big := strings.Repeat(fakeAWS + " ", 200000) // ~4 MiB
	writeFile(t, filepath.Join(root, "huge.txt"), big)
	findings, _ := Scan(root, DefaultPatterns(), DefaultSkip())
	if len(findings) != 0 {
		t.Errorf("oversized file (>1 MiB) must be skipped, got %v", findings)
	}
}

func TestScanWalksSubdirectoriesUnlessSkipped(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a/b/c/leaf.txt"),
		fakeAWS+"\n")
	findings, _ := Scan(root, DefaultPatterns(), DefaultSkip())
	if len(findings) != 1 {
		t.Errorf("walker must descend into non-skipped subdirs, got %v", findings)
	}
}

func TestRedactedExcerptHidesTheSecret(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "key.txt"),
		"aws="+fakeAWS+" more\n")
	findings, _ := Scan(root, DefaultPatterns(), DefaultSkip())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %v", findings)
	}
	if strings.Contains(findings[0].Excerpt, fakeAWS) {
		t.Errorf("excerpt must redact the secret value, got %q", findings[0].Excerpt)
	}
}
