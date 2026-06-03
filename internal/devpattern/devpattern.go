// Package devpattern records the source pattern (top_down vs bottom_up) for
// each work item, as required by the initial agreement Section 5. Events are
// appended to .devpattern/events.jsonl so they're machine-readable and
// inspectable by CI.
package devpattern

import (
	"bufio"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

const (
	dirName  = ".devpattern"
	fileName = "events.jsonl"

	SourceTopDown  = "top_down"
	SourceBottomUp = "bottom_up"
)

type Event struct {
	WorkItem string    `json:"work_item"`
	Source   string    `json:"source"`
	At       time.Time `json:"at"`
	Note     string    `json:"note,omitempty"`
	Override bool      `json:"override,omitempty"`
}

// Select uses crypto/rand to choose top_down or bottom_up. Random source is
// crypto/rand (not math/rand) so the event log is harder to bias.
func Select() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(2))
	if err != nil {
		return "", err
	}
	if n.Int64() == 0 {
		return SourceTopDown, nil
	}
	return SourceBottomUp, nil
}

// Append writes an event to .devpattern/events.jsonl under root.
func Append(root string, e Event) error {
	dir := filepath.Join(root, dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, fileName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

// Load reads every event in .devpattern/events.jsonl. Missing file → empty slice.
func Load(root string) ([]Event, error) {
	p := filepath.Join(root, dirName, fileName)
	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var out []Event
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Event
		if err := json.Unmarshal(line, &e); err != nil {
			return nil, fmt.Errorf("parse %s: %w", p, err)
		}
		out = append(out, e)
	}
	return out, sc.Err()
}
