package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/a3fckx/commons-contrib/internal/commons"
)

const githubTokenEnv = "GITHUB_TOKEN"

type githubSearchResponse struct {
	Items []githubIssue `json:"items"`
}

type githubIssue struct {
	Title    string `json:"title"`
	HTMLURL  string `json:"html_url"`
	State    string `json:"state"`
	Labels   []githubLabel `json:"labels"`
	Body     string `json:"body"`
	UpdatedAt string `json:"updated_at"`
	User     struct {
		Login string `json:"login"`
	} `json:"user"`
}

type githubLabel struct {
	Name string `json:"name"`
}

func main() {
	node := commons.DefaultNode
	author := "bounty-scout"
	if v := os.Getenv("SOURCEKIND_NODE"); v != "" {
		node = v
	}
	if v := os.Getenv("BOUNTY_SCOUT_AUTHOR"); v != "" {
		author = v
	}

	client := commons.NewClient(node, author)

	if alreadyPostedToday(client) {
		log.Println("Digest already posted today, skipping")
		return
	}

	bounties := fetchGitHubBounties()

	sort.Slice(bounties, func(i, j int) bool {
		return bounties[i].UpdatedAt.After(bounties[j].UpdatedAt)
	})

	if len(bounties) > 10 {
		bounties = bounties[:10]
	}

	if len(bounties) == 0 {
		log.Println("No new bounties found")
		return
	}

	content := formatDigest(bounties)
	post, err := client.Post(content, []string{"bounties", "money", "open-source"})
	if err != nil {
		log.Fatalf("Failed to post: %v", err)
	}

	log.Printf("Posted bounty digest %s with %d bounties", post.ID, len(bounties))
}

func fetchGitHubBounties() []commons.Bounty {
	var bounties []commons.Bounty

	queries := []string{
		`label:bounty+is:open+is:issue+created:>2026-06-01`,
		`label:bounty+is:open+is:issue+language:go+created:>2026-06-01`,
		`label:bounty+is:open+is:issue+language:typescript+created:>2026-06-01`,
		`label:"good+first+issue"+label:bounty+is:open+is:issue`,
	}

	token := os.Getenv(githubTokenEnv)

	for _, q := range queries {
		url := fmt.Sprintf("https://api.github.com/search/issues?q=%s&sort=updated&order=desc&per_page=10", q)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "bounty-scout/0.1.0")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != 200 {
			continue
		}

		var result githubSearchResponse
		if err := json.Unmarshal(body, &result); err != nil {
			continue
		}

		for _, item := range result.Items {
			amount := extractAmount(item.Body)
			if amount == "" {
				amount = extractAmount(item.Title)
			}
			lang := extractLanguage(item.Labels)

			updatedAt, _ := time.Parse(time.RFC3339, item.UpdatedAt)

			bounties = append(bounties, commons.Bounty{
				Title:       cleanTitle(item.Title),
				URL:         item.HTMLURL,
				Platform:    "GitHub",
				Amount:      amount,
				Language:    lang,
				Description: truncate(item.Body, 200),
				UpdatedAt:   updatedAt,
			})
		}
	}

	seen := map[string]bool{}
	var deduped []commons.Bounty
	for _, b := range bounties {
		if !seen[b.URL] {
			seen[b.URL] = true
			deduped = append(deduped, b)
		}
	}
	return deduped
}

func fetchAlgoraBounties() []commons.Bounty {
	return nil
}

func extractAmount(text string) string {
	text = strings.ToLower(text)
	for _, pattern := range []string{"$", "usd", "bounty:"} {
		idx := strings.Index(text, pattern)
		if idx >= 0 {
			end := idx + 30
			if end > len(text) {
				end = len(text)
			}
			snippet := text[idx:end]
			snippet = strings.TrimSpace(snippet)
			if len(snippet) > 20 {
				snippet = snippet[:20]
			}
			return strings.TrimSpace(snippet)
		}
	}
	return ""
}

func extractLanguage(labels []githubLabel) string {
	for _, l := range labels {
		if strings.HasPrefix(l.Name, "language:") {
			return strings.TrimPrefix(l.Name, "language:")
		}
	}
	return ""
}

func cleanTitle(title string) string {
	title = strings.TrimSpace(title)
	prefixes := []string{"[$", "[0", "[BOTTUBE:", "🎯", "💎"}
	for _, p := range prefixes {
		if strings.HasPrefix(title, p) {
			return title
		}
	}
	return title
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func alreadyPostedToday(client *commons.Client) bool {
	posts, err := client.Feed("global", "new", 50)
	if err != nil {
		return false
	}
	today := time.Now().Format("2006-01-02")
	for _, p := range posts {
		if p.Author == client.Author && strings.Contains(p.Content, today) {
			return true
		}
	}
	return false
}

func formatDigest(bounties []commons.Bounty) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# 🤖 Bounty Scout Digest — %s\n\n", time.Now().Format("2006-01-02")))
	sb.WriteString(fmt.Sprintf("**%d open bounties** found across GitHub and partner platforms.\n\n", len(bounties)))
	sb.WriteString("---\n\n")

	for i, b := range bounties {
		sb.WriteString(fmt.Sprintf("## %d. %s\n", i+1, b.Title))
		if b.Amount != "" {
			sb.WriteString(fmt.Sprintf("**Bounty:** %s  \n", b.Amount))
		}
		sb.WriteString(fmt.Sprintf("**Platform:** %s", b.Platform))
		if b.Language != "" {
			sb.WriteString(fmt.Sprintf(" · **Lang:** %s", b.Language))
		}
		sb.WriteString(fmt.Sprintf("  \n**URL:** %s\n", b.URL))
		if b.Description != "" {
			sb.WriteString(fmt.Sprintf("\n> %s\n", b.Description))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("---\n")
	sb.WriteString("*Scout runs daily. To filter by language or platform, reply with criteria.*\n")
	sb.WriteString("*Maintained by @commandcode · MIT License · [repo](https://github.com/a3fckx/commons-contrib)*\n")

	return sb.String()
}
