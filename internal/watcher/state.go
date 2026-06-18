package watcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type ConsolidationRecord struct {
	ConsolidatedAt             time.Time `json:"consolidatedAt"`
	OutputPostID               string    `json:"outputPostId"`
	ResponseCountAtConsolidation int     `json:"responseCountAtConsolidation"`
}

type State struct {
	Consolidated map[string]ConsolidationRecord `json:"consolidated"`
	path         string
}

func LoadState(path string) (*State, error) {
	s := &State{
		Consolidated: make(map[string]ConsolidationRecord),
		path:         path,
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(raw, s); err != nil {
		return nil, err
	}
	if s.Consolidated == nil {
		s.Consolidated = make(map[string]ConsolidationRecord)
	}
	s.path = path
	return s, nil
}

func (s *State) Save() error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o644)
}

func (s *State) WasConsolidated(postID string) bool {
	_, ok := s.Consolidated[postID]
	return ok
}

// NeedsUpdate returns true when a thread gained enough new replies to warrant refresh.
func (s *State) NeedsUpdate(postID string, currentResponses int) bool {
	rec, ok := s.Consolidated[postID]
	if !ok {
		return false
	}
	return currentResponses-rec.ResponseCountAtConsolidation >= 3
}

func (s *State) Mark(postID, outputPostID string, responseCount int) {
	s.Consolidated[postID] = ConsolidationRecord{
		ConsolidatedAt:               time.Now().UTC(),
		OutputPostID:                 outputPostID,
		ResponseCountAtConsolidation: responseCount,
	}
}

func DefaultStatePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ".state/thread-curator.json", nil
	}
	return filepath.Join(dir, "commons-contrib", "thread-curator.json"), nil
}