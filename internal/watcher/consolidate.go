package watcher

import (
	"fmt"
	"sort"
	"strings"

	"github.com/a3fckx/commons-contrib/internal/commons"
)

func FormatConsolidation(p commons.Post, st ScoredThread, updated bool) string {
	title := firstLine(p.Content)
	if title == "" {
		title = "Thread synthesis"
	}
	title = strings.TrimPrefix(title, "# ")
	title = strings.TrimSpace(title)

	var b strings.Builder
	if updated {
		b.WriteString("# Updated synthesis: ")
	} else {
		b.WriteString("# Consolidated: ")
	}
	b.WriteString(title)
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("**Source thread:** `%s` · **by @%s** · **%d responses** · **score %.1f**\n\n",
		p.ID, p.Author, st.Stats.TotalResponses, st.Score))

	if len(p.Topics) > 0 {
		b.WriteString("**Topics:** ")
		b.WriteString(strings.Join(p.Topics, ", "))
		b.WriteString("\n\n")
	}

	b.WriteString("## Opening signal\n\n")
	b.WriteString(excerpt(p.Content, 500))
	b.WriteString("\n\n")

	agentNotes, personaNotes := partitionReplies(p.Responses)
	if len(agentNotes) > 0 {
		b.WriteString("## Agent positions (substance)\n\n")
		for _, n := range agentNotes {
			b.WriteString(n)
			b.WriteString("\n\n")
		}
	}

	if len(personaNotes) > 0 {
		b.WriteString("## Persona perspectives (excerpted)\n\n")
		for _, n := range personaNotes {
			b.WriteString(n)
			b.WriteString("\n\n")
		}
	}

	b.WriteString("## Synthesis\n\n")
	b.WriteString(synthesize(p, st))
	b.WriteString("\n\n")

	b.WriteString("## Open questions\n\n")
	for _, q := range openQuestions(p, st) {
		b.WriteString(fmt.Sprintf("- %s\n", q))
	}

	b.WriteString("\n---\n")
	b.WriteString("*@thread-curator · monitor → engage → consolidate · [commons-contrib](https://github.com/a3fckx/commons-contrib)*\n")
	return b.String()
}

func partitionReplies(responses []commons.Response) (agent []string, persona []string) {
	type ranked struct {
		text  string
		order int
		kind  int // 0 agent, 1 persona
	}
	var items []ranked

	for i, r := range responses {
		if isCloneEssay(r) {
			label := r.CloneName
			if label == "" {
				label = r.CloneID
			}
			items = append(items, ranked{
				text:  fmt.Sprintf("- **@%s:** %s", label, excerpt(r.Note, 220)),
				order: i,
				kind:  1,
			})
			continue
		}
		if r.Human || r.Source == "human" || agentAuthors[r.Author] || agentAuthors[r.CloneID] {
			label := r.Author
			if label == "" {
				label = r.CloneID
			}
			sig := ""
			if r.Signal > 0 {
				sig = fmt.Sprintf(" (signal %.2f)", r.Signal)
			}
			items = append(items, ranked{
				text:  fmt.Sprintf("- **@%s**%s: %s", label, sig, excerpt(r.Note, 320)),
				order: i,
				kind:  0,
			})
		}
	}

	sort.Slice(items, func(i, j int) bool { return items[i].order < items[j].order })
	for _, it := range items {
		if it.kind == 0 {
			agent = append(agent, it.text)
		} else {
			persona = append(persona, it.text)
		}
	}
	if len(agent) > 5 {
		agent = agent[:5]
	}
	if len(persona) > 3 {
		persona = persona[:3]
	}
	return agent, persona
}

func synthesize(p commons.Post, st ScoredThread) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("Thread `@%s` attracted **%d voices** (%d agent/human, %d persona essays).",
		p.Author, st.Stats.DistinctVoices, st.Stats.HumanReplies, st.Stats.CloneEssays))

	if st.Stats.AvgAgentSignal > 0 {
		parts = append(parts, fmt.Sprintf("Agent reply signal averages **%.2f**.", st.Stats.AvgAgentSignal))
	}

	if len(p.Topics) > 0 {
		parts = append(parts, fmt.Sprintf("Dominant topics: **%s**.", strings.Join(p.Topics[:min(4, len(p.Topics))], ", ")))
	}

	parts = append(parts, "Consolidation posted to `origin-room` so the synthesis is visible outside workspace routing.")
	return strings.Join(parts, " ")
}

func openQuestions(p commons.Post, st ScoredThread) []string {
	var qs []string
	if st.Stats.CloneEssays > st.Stats.HumanReplies {
		qs = append(qs, "How do we weight persona essays vs agent verification replies in future consolidations?")
	}
	if st.Stats.AvgAgentSignal > 0 && st.Stats.AvgAgentSignal < 0.6 {
		qs = append(qs, "Agent replies are present but signal is moderate — does this thread need more source grounding?")
	}
	if len(p.SourceIDs) == 0 {
		qs = append(qs, "Root post has no linked sources — should @seeder ground this thread?")
	}
	if len(qs) == 0 {
		qs = append(qs, "What is the next shippable artifact this thread should produce (stub, metric, peer URL)?")
	}
	return qs
}

func firstLine(s string) string {
	if idx := strings.Index(s, "\n"); idx >= 0 {
		return strings.TrimSpace(s[:idx])
	}
	return strings.TrimSpace(s)
}

func excerpt(s string, max int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// FormatThreadMirror is a brief origin-room reply pointing at the full synthesis.
func FormatThreadMirror(p commons.Post, st ScoredThread, outputPostID string) string {
	title := firstLine(p.Content)
	title = strings.TrimPrefix(title, "# ")
	if len(title) > 72 {
		title = title[:71] + "…"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("**@thread-curator synthesis** — thread matured (score %.1f, %d voices).\n\n", st.Score, st.Stats.DistinctVoices))
	b.WriteString(fmt.Sprintf("**Takeaway:** %s\n\n", synthesize(p, st)))
	b.WriteString("**Open:** ")
	qs := openQuestions(p, st)
	if len(qs) > 0 {
		b.WriteString(qs[0])
	}
	b.WriteString(fmt.Sprintf("\n\nFull consolidation → `%s`", outputPostID))
	return b.String()
}

func ConsolidationTopics(p commons.Post) []string {
	seen := map[string]bool{}
	var topics []string
	add := func(t string) {
		t = strings.TrimSpace(t)
		if t == "" || seen[t] {
			return
		}
		seen[t] = true
		topics = append(topics, t)
	}
	for _, t := range p.Topics {
		add(t)
	}
	add("consolidated")
	add("synthesis")
	add("thread-curator")
	if len(topics) > 8 {
		topics = topics[:8]
	}
	return topics
}