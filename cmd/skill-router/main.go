package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/a3fckx/commons-contrib/internal/commons"
	"github.com/a3fckx/commons-contrib/internal/mdx"
	"github.com/a3fckx/commons-contrib/internal/skillrouter"
)

func main() {
	node := commons.DefaultNode
	author := "skill-router"
	query := strings.TrimSpace(os.Getenv("SKILL_ROUTER_QUERY"))

	if v := os.Getenv("SOURCEKIND_NODE"); v != "" {
		node = v
	}
	if v := os.Getenv("SKILL_ROUTER_AUTHOR"); v != "" {
		author = v
	}

	root, _ := os.Getwd()
	catalogPath := envOr("SKILLS_CATALOG_PATH", filepath.Join(root, "skills.catalog.json"))
	agentsPath := envOr("AGENTS_REGISTRY_PATH", filepath.Join(root, "agents.registry.json"))

	catalog, err := skillrouter.LoadCatalog(catalogPath)
	if err != nil {
		log.Fatalf("catalog: %v", err)
	}
	agents, err := skillrouter.LoadAgents(agentsPath)
	if err != nil {
		log.Fatalf("agents: %v", err)
	}

	if query == "" {
		query = "monitor conversations engage consolidate deploy skills"
	}

	matches := skillrouter.MatchQuery(query, catalog, agents, 5)
	body := skillrouter.FormatDigest(query, matches)
	topics := []string{"skill-router", "skills", "query", "routing"}
	for _, m := range matches {
		topics = append(topics, m.ID)
	}

	client := commons.NewClient(node, author)
	post, err := client.PostRich(commons.PostRequest{
		Author:     author,
		Content:    body,
		ChannelID:  "origin-room",
		Kind:       "mdx",
		Topics:     topics[:min(8, len(topics))],
		RenderMeta: mdx.RenderMeta("sourcekind.routing.v1", "Skill route", query),
	})
	if err != nil {
		log.Fatalf("post: %v", err)
	}
	log.Printf("skill-router posted %s (%d matches)", post.ID, len(matches))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}