// Package knowledge is a simple file-based knowledge surface. It walks repo
// files, builds an inverted index of word → []occurrence, and supports
// natural-language-ish queries by tokenizing the query and scoring documents.
//
// This is NOT a vector index. It is intentionally a transparent grep-with-
// scoring. The brief says the vector store is a future upgrade and that
// source files remain SSOT; an inverted index honors that. Decision required
// before adding embeddings.
package knowledge

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	indexDir    = ".knowledge"
	indexFile   = "index.json"
	recordsFile = "records.jsonl"
)

// Indexable file extensions. Add new ones with a decision record so the
// knowledge surface stays consistent across agents.
var indexableExt = map[string]bool{
	".md":   true,
	".json": true,
	".lisp": true,
	".go":   true,
	".yml":  true,
	".yaml": true,
	".dart": true,
}

var skipDirs = map[string]bool{
	".git":         true,
	"bin":          true,
	"vendor":       true,
	"node_modules": true,
	".knowledge":   true,
	".devpattern":  true,
	".dart_tool":   true,
	"build":        true,
	"Pods":         true,
}

// Index is the on-disk inverted index.
type Index struct {
	BuiltAt time.Time           `json:"built_at"`
	Roots   []string            `json:"roots"`
	Tokens  map[string][]Occurr `json:"tokens"`
	Files   []FileMeta          `json:"files"`
}

type Occurr struct {
	FileID int `json:"f"`
	Line   int `json:"l"`
}

type FileMeta struct {
	ID   int    `json:"id"`
	Path string `json:"path"`
}

// Build walks each root and produces an Index. paths are stored relative to
// each respective root and prefixed with the root's index in Roots.
func Build(roots []string) (*Index, error) {
	idx := &Index{
		BuiltAt: time.Now().UTC(),
		Roots:   roots,
		Tokens:  map[string][]Occurr{},
	}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if skipDirs[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			ext := strings.ToLower(filepath.Ext(d.Name()))
			if !indexableExt[ext] {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			if info.Size() > 5<<20 {
				return nil
			}
			fid := len(idx.Files)
			idx.Files = append(idx.Files, FileMeta{ID: fid, Path: path})
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			sc := bufio.NewScanner(f)
			sc.Buffer(make([]byte, 1<<20), 1<<20)
			line := 0
			for sc.Scan() {
				line++
				for _, tok := range tokenize(sc.Text()) {
					idx.Tokens[tok] = append(idx.Tokens[tok], Occurr{FileID: fid, Line: line})
				}
			}
			return sc.Err()
		})
		if err != nil {
			return nil, err
		}
	}
	return idx, nil
}

// Save writes the index to root/.knowledge/index.json.
func Save(root string, idx *Index) error {
	dir := filepath.Join(root, indexDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(idx)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, indexFile), data, 0o644)
}

// LoadIndex reads root/.knowledge/index.json.
func LoadIndex(root string) (*Index, error) {
	p := filepath.Join(root, indexDir, indexFile)
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	idx := &Index{}
	if err := json.Unmarshal(data, idx); err != nil {
		return nil, fmt.Errorf("parse %s: %w", p, err)
	}
	return idx, nil
}

// QueryResult is one hit returned by Query.
type QueryResult struct {
	Path  string  `json:"path"`
	Score float64 `json:"score"`
	Lines []int   `json:"lines"`
}

// Query tokenizes q and scores each file by sum of per-token hits.
func (idx *Index) Query(q string, top int) []QueryResult {
	terms := tokenize(q)
	if len(terms) == 0 {
		return nil
	}
	fileScore := map[int]float64{}
	fileLines := map[int]map[int]bool{}
	for _, t := range terms {
		hits := idx.Tokens[t]
		for _, h := range hits {
			fileScore[h.FileID] += 1.0
			if fileLines[h.FileID] == nil {
				fileLines[h.FileID] = map[int]bool{}
			}
			fileLines[h.FileID][h.Line] = true
		}
	}
	var out []QueryResult
	for fid, s := range fileScore {
		linesSet := fileLines[fid]
		lines := make([]int, 0, len(linesSet))
		for l := range linesSet {
			lines = append(lines, l)
		}
		sort.Ints(lines)
		if len(lines) > 5 {
			lines = lines[:5]
		}
		out = append(out, QueryResult{
			Path:  idx.Files[fid].Path,
			Score: s,
			Lines: lines,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if top > 0 && len(out) > top {
		out = out[:top]
	}
	return out
}

// KnowledgeRecord is a manual entry (PR summary, CI failure summary, note).
type KnowledgeRecord struct {
	ID         string    `json:"id"`
	At         time.Time `json:"at"`
	Kind       string    `json:"kind"`
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	Decision   string    `json:"decision,omitempty"`
	Tags       []string  `json:"tags,omitempty"`
	Status     string    `json:"status"` // "active" | "superseded" | "rejected"
	Supersedes string    `json:"supersedes,omitempty"`
}

// AppendRecord adds a manual record to .knowledge/records.jsonl.
func AppendRecord(root string, r KnowledgeRecord) error {
	dir := filepath.Join(root, indexDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, recordsFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	return err
}

// LoadRecords reads .knowledge/records.jsonl (missing file → empty).
func LoadRecords(root string) ([]KnowledgeRecord, error) {
	p := filepath.Join(root, indexDir, recordsFile)
	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var out []KnowledgeRecord
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var r KnowledgeRecord
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, sc.Err()
}

// Supersede marks one record as superseded by another. Implementation: append
// a new event with status="superseded" and Supersedes=oldID; LoadRecords
// callers fold these on read.
func Supersede(root, oldID, newID, note string) error {
	return AppendRecord(root, KnowledgeRecord{
		ID:         newID,
		At:         time.Now().UTC(),
		Kind:       "supersede",
		Title:      "supersede " + oldID,
		Body:       note,
		Status:     "active",
		Supersedes: oldID,
	})
}

// Fold collapses a record stream into the latest live state: superseded entries
// are removed, replaced by their supersedors.
func Fold(records []KnowledgeRecord) []KnowledgeRecord {
	superseded := map[string]bool{}
	for _, r := range records {
		if r.Supersedes != "" {
			superseded[r.Supersedes] = true
		}
	}
	var out []KnowledgeRecord
	for _, r := range records {
		if superseded[r.ID] {
			continue
		}
		out = append(out, r)
	}
	return out
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	var out []string
	var cur strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			cur.WriteRune(r)
			continue
		}
		if cur.Len() >= 3 {
			out = append(out, cur.String())
		}
		cur.Reset()
	}
	if cur.Len() >= 3 {
		out = append(out, cur.String())
	}
	return out
}
