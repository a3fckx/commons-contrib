package skillrouter

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
)

type Catalog struct {
	Version string  `json:"version"`
	Skills  []Skill `json:"skills"`
}

type Skill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	Triggers    []string `json:"triggers"`
	Command     string   `json:"command"`
	Description string   `json:"description"`
}

type AgentEntry struct {
	ID      string `json:"id"`
	Class   string `json:"class"`
	Role    string `json:"role"`
	Binary  string `json:"binary"`
	Repo    string `json:"repo"`
	Cadence string `json:"cadence"`
}

type AgentRegistry struct {
	Agents []AgentEntry `json:"agents"`
}

type Match struct {
	Kind        string  `json:"kind"`
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Score       float64 `json:"score"`
	Command     string  `json:"command"`
	Description string  `json:"description"`
	Path        string  `json:"path,omitempty"`
	Class       string  `json:"class,omitempty"`
}

func LoadCatalog(path string) (*Catalog, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Catalog
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func LoadAgents(path string) (*AgentRegistry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r AgentRegistry
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func tokenize(q string) []string {
	q = strings.ToLower(q)
	for _, ch := range []string{",", ".", "?", "!", ":", ";", "(", ")", "[", "]"} {
		q = strings.ReplaceAll(q, ch, " ")
	}
	parts := strings.Fields(q)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) > 2 {
			out = append(out, p)
		}
	}
	return out
}

func scoreTriggers(tokens []string, triggers []string, id string) float64 {
	if len(tokens) == 0 {
		return 0
	}
	joined := strings.Join(tokens, " ")
	score := 0.0
	for _, t := range triggers {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		if strings.Contains(joined, t) {
			score += float64(len(strings.Fields(t))) * 2
		}
		for _, tok := range tokens {
			if tok == t || strings.Contains(t, tok) || strings.Contains(tok, t) {
				score += 1
			}
		}
	}
	for _, tok := range tokens {
		if tok == id || strings.Contains(id, tok) {
			score += 2
		}
	}
	return score
}

var agentCommands = map[string]string{
	"bounty-scout":   "cd commons-contrib && make run-bounty",
	"sourcekind":     "cd commons-contrib && make run-sourcekind",
	"sim-verifier":   "cd commons-contrib && make run-pipeline",
	"thread-curator": "cd commons-contrib && make run-conversation",
	"skill-router":   "cd commons-contrib && make run-skill-router",
}

func agentCommand(id, binary string) string {
	if c, ok := agentCommands[id]; ok {
		return c
	}
	if binary != "" {
		return "go run ./" + binary
	}
	return ""
}

func MatchQuery(query string, catalog *Catalog, agents *AgentRegistry, limit int) []Match {
	tokens := tokenize(query)
	var matches []Match

	for _, s := range catalog.Skills {
		sc := scoreTriggers(tokens, s.Triggers, s.ID)
		if sc <= 0 {
			continue
		}
		matches = append(matches, Match{
			Kind: "skill", ID: s.ID, Name: s.Name, Score: sc,
			Command: s.Command, Description: s.Description, Path: s.Path,
		})
	}

	for _, a := range agents.Agents {
		triggers := []string{a.ID, a.Class, a.Role, a.Binary}
		sc := scoreTriggers(tokens, triggers, a.ID)
		if sc <= 0 {
			continue
		}
		cmd := agentCommand(a.ID, a.Binary)
		matches = append(matches, Match{
			Kind: "agent", ID: a.ID, Name: "@" + a.ID, Score: sc,
			Command: cmd, Description: a.Role, Class: a.Class,
		})
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	if limit > 0 && len(matches) > limit {
		matches = matches[:limit]
	}
	return matches
}