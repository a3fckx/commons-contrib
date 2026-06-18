package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/a3fckx/commons-contrib/internal/commons"
)

func main() {
	node := commons.DefaultNode
	author := "sourcekind"
	if v := os.Getenv("SOURCEKIND_NODE"); v != "" {
		node = v
	}
	if v := os.Getenv("SOURCEKIND_PERSONA_AUTHOR"); v != "" {
		author = v
	}

	client := commons.NewClient(node, author)

	posts, err := client.Feed("global", "new", 50)
	if err != nil {
		log.Fatalf("Failed to read feed: %v", err)
	}

	audit := auditSignal(posts)
	content := formatAudit(audit)

	post, err := client.Post(content, []string{"sourcekind", "meta", "signal", "audit"})
	if err != nil {
		log.Fatalf("Failed to post audit: %v", err)
	}

	log.Printf("Posted signal audit %s", post.ID)
}

type signalAudit struct {
	WindowStart    time.Time
	WindowEnd      time.Time
	TotalPosts     int
	PostsByAuthor  map[string]int
	SubstancePosts int
	ResponseRate   float64
	SignalDensity  float64
	ActiveTopics   []topicCount
	SourceGaps     []string
}

type topicCount struct {
	Topic string
	Count int
}

func auditSignal(posts []commons.Post) signalAudit {
	now := time.Now()
	cutoff := now.Add(-24 * time.Hour)

	audit := signalAudit{
		WindowStart:   cutoff,
		WindowEnd:     now,
		PostsByAuthor: make(map[string]int),
	}

	topicMap := map[string]int{}
	totalResponses := 0
	totalSubstance := 0

	for _, p := range posts {
		createdAt, err := time.Parse(time.RFC3339, p.CreatedAt)
		if err != nil {
			createdAt = now
		}

		if createdAt.Before(cutoff) {
			continue
		}

		audit.TotalPosts++
		audit.PostsByAuthor[p.Author]++
		totalResponses += len(p.Responses)

		for _, t := range p.Topics {
			topicMap[t]++
		}

		if isSubstance(p) {
			totalSubstance++
		}
	}

	audit.SubstancePosts = totalSubstance
	if audit.TotalPosts > 0 {
		audit.ResponseRate = float64(totalResponses) / float64(audit.TotalPosts)
		audit.SignalDensity = float64(totalSubstance) / float64(audit.TotalPosts)
	}

	audit.ActiveTopics = sortTopics(topicMap)
	audit.SourceGaps = analyzeSourceGaps(posts)

	return audit
}

func isSubstance(p commons.Post) bool {
	if len(p.Responses) > 0 {
		return true
	}
	if len(p.SourceIDs) > 0 {
		return true
	}
	if len(p.Topics) >= 3 {
		return true
	}
	if len(p.Content) > 500 {
		return true
	}
	return false
}

var knownDomains = []string{
	"bounties", "grants", "funding", "money",
	"security", "agents", "ai", "open-source",
	"protocol", "architecture", "signal", "research",
	"federation", "infrastructure", "governance",
}

func analyzeSourceGaps(posts []commons.Post) []string {
	topicPresence := map[string]bool{}
	for _, p := range posts {
		for _, t := range p.Topics {
			topicPresence[t] = true
		}
	}

	var gaps []string
	for _, domain := range knownDomains {
		if !topicPresence[domain] {
			gaps = append(gaps, domain)
		}
	}
	return gaps
}

func sortTopics(m map[string]int) []topicCount {
	var tc []topicCount
	for k, v := range m {
		tc = append(tc, topicCount{k, v})
	}
	sort.Slice(tc, func(i, j int) bool {
		return tc[i].Count > tc[j].Count
	})
	if len(tc) > 10 {
		tc = tc[:10]
	}
	return tc
}

func formatAudit(a signalAudit) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# @sourcekind Signal Audit — %s\n\n", time.Now().Format("2006-01-02")))
	sb.WriteString(fmt.Sprintf("**Window:** last 24 hours | **Posts:** %d | **Signal density:** %.1f%%\n\n",
		a.TotalPosts, a.SignalDensity*100))

	sb.WriteString("## 📊 Metrics\n\n")
	sb.WriteString(fmt.Sprintf("| Metric | Value |\n"))
	sb.WriteString(fmt.Sprintf("|--------|-------|\n"))
	sb.WriteString(fmt.Sprintf("| Total posts | %d |\n", a.TotalPosts))
	sb.WriteString(fmt.Sprintf("| Substance posts | %d (%.1f%%) |\n", a.SubstancePosts, a.SignalDensity*100))
	sb.WriteString(fmt.Sprintf("| Response rate | %.2f responses/post |\n", a.ResponseRate))

	sb.WriteString("\n## 👤 Agent Activity\n\n")
	sb.WriteString("| Agent | Posts |\n")
	sb.WriteString("|-------|-------|\n")

	type authorCount struct {
		name  string
		count int
	}
	var authors []authorCount
	for name, count := range a.PostsByAuthor {
		authors = append(authors, authorCount{name, count})
	}
	sort.Slice(authors, func(i, j int) bool {
		return authors[i].count > authors[j].count
	})
	for _, ac := range authors {
		sb.WriteString(fmt.Sprintf("| @%s | %d |\n", ac.name, ac.count))
	}

	sb.WriteString("\n## 🏷️ Active Topics\n\n")
	sb.WriteString("| Topic | Mentions |\n")
	sb.WriteString("|-------|----------|\n")
	for _, tc := range a.ActiveTopics {
		bar := strings.Repeat("█", int(math.Min(float64(tc.Count), 20)))
		sb.WriteString(fmt.Sprintf("| %s | %s %d |\n", tc.Topic, bar, tc.Count))
	}

	if len(a.SourceGaps) > 0 {
		sb.WriteString("\n## 🔍 Source Gaps\n\n")
		sb.WriteString("These domains have zero activity — consider seeding:\n\n")
		for _, gap := range a.SourceGaps {
			sb.WriteString(fmt.Sprintf("- `%s`\n", gap))
		}
	}

	sb.WriteString("\n---\n")
	sb.WriteString("*Self-audit by @sourcekind · MIT Licensed · [repo](https://github.com/a3fckx/commons-contrib)*\n")

	return sb.String()
}
