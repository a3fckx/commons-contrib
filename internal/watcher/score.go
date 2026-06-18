package watcher

import (
	"strings"

	"github.com/a3fckx/commons-contrib/internal/commons"
)

var clonePersonas = map[string]bool{
	"einstein": true,
	"tesla":    true,
	"jobs":     true,
}

var agentAuthors = map[string]bool{
	"sim-verifier":  true,
	"grok-agent":    true,
	"commandcode":   true,
	"bounty-scout":  true,
	"sourcekind":    true,
	"thread-curator": true,
	"erdos-program": true,
}

// Config tunes monitor → engage → consolidate behavior.
type Config struct {
	Author                string
	EngageLimit           int
	ConsolidationThreshold float64
	ConsolidationChannel  string
	StatePath             string
	SkipEngage            bool
}

// ScoredThread is a feed post ranked for consolidation.
type ScoredThread struct {
	Post    commons.Post
	Score   float64
	Reasons []string
	Ready   bool
	Stats   ThreadStats
}

type ThreadStats struct {
	TotalResponses int
	HumanReplies   int
	AgentReplies   int
	CloneEssays    int
	DistinctVoices int
	AvgAgentSignal float64
}

func ScoreThread(p commons.Post) ScoredThread {
	st := analyzeResponses(p)
	score := 0.0
	var reasons []string

	if st.TotalResponses >= 3 {
		score += 2.0
		reasons = append(reasons, "≥3 responses")
	}
	if st.HumanReplies >= 1 {
		score += 2.0
		reasons = append(reasons, "human/agent replies present")
	}
	if st.DistinctVoices >= 2 {
		score += 1.5
		reasons = append(reasons, "multi-voice thread")
	}
	if st.AgentReplies >= 1 {
		score += 1.0
	}
	if st.AvgAgentSignal >= 0.55 {
		score += 1.0
		reasons = append(reasons, "high-signal agent replies")
	}
	if len(p.SourceIDs) > 0 {
		score += 0.5
	}
	if len(strings.TrimSpace(p.Content)) > 200 {
		score += 0.5
	}
	if st.TotalResponses > 0 {
		essayRatio := float64(st.CloneEssays) / float64(st.TotalResponses)
		if essayRatio < 0.75 {
			score += 1.0
			reasons = append(reasons, "not essay-dominated")
		}
	}
	if len(p.Topics) >= 2 {
		score += 0.5
	}

	return ScoredThread{
		Post:    p,
		Score:   score,
		Reasons: reasons,
		Ready:   score >= 5.0 && st.TotalResponses >= 3,
		Stats:   st,
	}
}

func analyzeResponses(p commons.Post) ThreadStats {
	voices := map[string]bool{}
	var agentSignals []float64

	st := ThreadStats{TotalResponses: len(p.Responses)}
	for _, r := range p.Responses {
		voice := responseVoice(r)
		if voice != "" {
			voices[voice] = true
		}

		if isCloneEssay(r) {
			st.CloneEssays++
			continue
		}
		if r.Human || r.Source == "human" || agentAuthors[r.Author] || agentAuthors[r.CloneID] {
			st.HumanReplies++
			if r.Signal > 0 {
				agentSignals = append(agentSignals, r.Signal)
			}
			continue
		}
		if agentAuthors[r.CloneID] {
			st.AgentReplies++
			if r.Signal > 0 {
				agentSignals = append(agentSignals, r.Signal)
			}
		}
	}

	st.DistinctVoices = len(voices)
	if len(voices) > 0 {
		voices[p.Author] = true
		st.DistinctVoices = len(voices)
	}
	if len(agentSignals) > 0 {
		sum := 0.0
		for _, s := range agentSignals {
			sum += s
		}
		st.AvgAgentSignal = sum / float64(len(agentSignals))
	}
	return st
}

func responseVoice(r commons.Response) string {
	if r.Author != "" {
		return r.Author
	}
	if r.CloneID != "" && !clonePersonas[r.CloneID] {
		return r.CloneID
	}
	if clonePersonas[r.CloneID] {
		return r.CloneID
	}
	return ""
}

func isCloneEssay(r commons.Response) bool {
	if clonePersonas[r.CloneID] {
		style := strings.ToLower(r.PublicationStyle)
		if style == "essay" || style == "discussion" {
			return len(r.Note) > 400
		}
	}
	return false
}

func IsOwnContent(author, postAuthor string) bool {
	return author == postAuthor
}