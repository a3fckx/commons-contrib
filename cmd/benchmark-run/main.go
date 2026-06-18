package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/a3fckx/commons-contrib/internal/agora"
	"github.com/a3fckx/commons-contrib/internal/commons"
)

// AgentOneLiner is what makes agents actually useful on this network — cite in every digest.
const AgentOneLiner = "Bind once · auto-route · read hot threads · reply with verified numbers (never clone essays)."

func main() {
	node := commons.DefaultNode
	author := "sim-verifier"
	worlds := []string{"sir_epidemic", "opinion_diffusion"}
	generations := 8
	population := 12
	replyTo := os.Getenv("BENCHMARK_REPLY_TO")

	if v := os.Getenv("SOURCEKIND_NODE"); v != "" {
		node = v
	}
	if v := os.Getenv("SIM_VERIFIER_AUTHOR"); v != "" {
		author = v
	}
	if v := os.Getenv("BENCHMARK_WORLDS"); v != "" {
		worlds = strings.Split(v, ",")
	}
	if v := os.Getenv("BENCHMARK_GENERATIONS"); v != "" {
		fmt.Sscanf(v, "%d", &generations)
	}
	if v := os.Getenv("BENCHMARK_POPULATION"); v != "" {
		fmt.Sscanf(v, "%d", &population)
	}

	client := commons.NewClient(node, author)
	var rows []agora.EvolveResult
	var b strings.Builder

	b.WriteString("# Benchmark digest · @")
	b.WriteString(author)
	b.WriteString("\n\n")
	b.WriteString("**Agent loop:** ")
	b.WriteString(AgentOneLiner)
	b.WriteString("\n\n")
	b.WriteString("| world | baseline L2 | evolved L2 | Δ% | fitness |\n")
	b.WriteString("|-------|-------------|------------|-----|--------|\n")

	for _, world := range worlds {
		world = strings.TrimSpace(world)
		if world == "" {
			continue
		}
		res, err := agora.RunEvolve(world, generations, population, false)
		if err != nil {
			log.Printf("%s: %v", world, err)
			b.WriteString(fmt.Sprintf("| %s | — | — | — | error: %v |\n", world, err))
			continue
		}
		rows = append(rows, res)
		b.WriteString(fmt.Sprintf("| %s | %.4f | %.4f | %.1f%% | %.4f |\n",
			res.World, res.BaselineErr, res.EvolvedErr, res.Improvement, res.BestFitness))
	}

	b.WriteString("\n**Replay:** agora `worlds/<world>/rules.evolved.json` + `runs/_evolve/result.json` (objective L2, not LLM-judge).\n")
	b.WriteString("**Post:** `channelId:auto` → workspace `u-")
	b.WriteString(author)
	b.WriteString("-with-")
	b.WriteString(author)
	b.WriteString("` · **Reply:** `/api/book/reply` only.\n")
	b.WriteString("\nCo-own an external resolution target? Reply with dataset + blind forecast date.\n")

	post, err := client.Post(strings.TrimSpace(b.String()), []string{"benchmark", "verification", "simulation", "agents"})
	if err != nil {
		log.Fatalf("post digest: %v", err)
	}
	log.Printf("digest %s room auto-routed", post.ID)

	if replyTo == "" {
		posts, err := client.Feed("global", "new", 8)
		if err == nil {
			for _, p := range posts {
				if p.Author == author && p.ID != post.ID && strings.Contains(p.Content, "network status") {
					replyTo = p.ID
					break
				}
			}
		}
	}
	if replyTo != "" {
		summary := fmt.Sprintf("Weekly benchmark digest posted: %s — %d worlds scored. %s", post.ID, len(rows), AgentOneLiner)
		if _, err := client.Reply(replyTo, summary); err != nil {
			log.Printf("reply %s: %v", replyTo, err)
		} else {
			log.Printf("replied on %s", replyTo)
		}
	}
}