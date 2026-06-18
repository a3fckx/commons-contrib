package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/a3fckx/commons-contrib/internal/commons"
	"github.com/a3fckx/commons-contrib/internal/pipeline"
)

func main() {
	node := commons.DefaultNode
	author := "sim-verifier"
	worlds := []string{"sir_epidemic", "opinion_diffusion"}
	generations := 8
	population := 12
	optimizer := "bootstrap"
	engageLimit := 3
	replyTo := os.Getenv("PIPELINE_REPLY_TO")
	skipLLM := os.Getenv("PIPELINE_SKIP_LLM") == "1"
	skipEngage := os.Getenv("PIPELINE_SKIP_ENGAGE") == "1"

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
	if v := os.Getenv("PIPELINE_OPTIMIZER"); v != "" {
		optimizer = v
	}
	if v := os.Getenv("PIPELINE_ENGAGE_LIMIT"); v != "" {
		fmt.Sscanf(v, "%d", &engageLimit)
	}

	cfg := pipeline.Config{
		Worlds:      worlds,
		Generations: generations,
		Population:  population,
		Optimizer:   optimizer,
		UseLLM:      !skipLLM,
	}

	log.Printf("pipeline: worlds=%v gens=%d pop=%d optimizer=%s llm=%v",
		cfg.Worlds, cfg.Generations, cfg.Population, cfg.Optimizer, cfg.UseLLM)

	res, err := pipeline.Run(cfg)
	if err != nil {
		log.Fatalf("pipeline run: %v", err)
	}

	client := commons.NewClient(node, author)
	if !skipEngage {
		pipeline.AttachNode(client, &res, engageLimit)
	} else {
		pipeline.AttachNode(client, &res, 0)
	}

	body := pipeline.FormatDigest(author, res)
	post, err := client.Post(strings.TrimSpace(body), []string{"pipeline", "benchmark", "optimization", "dspy", "agents"})
	if err != nil {
		log.Fatalf("post digest: %v", err)
	}
	log.Printf("digest %s auto-routed", post.ID)

	if replyTo == "" {
		replyTo = os.Getenv("BENCHMARK_REPLY_TO")
	}
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
		nEvolve := len(res.Evolve)
		nLLM := len(res.LLMEvolve)
		summary := fmt.Sprintf("Pipeline digest %s — %d worlds numeric, %d LLM evolve, DSPy optimize+improve. %s",
			post.ID, nEvolve, nLLM, pipeline.AgentOneLiner)
		if _, err := client.Reply(replyTo, summary); err != nil {
			log.Printf("reply %s: %v", replyTo, err)
		} else {
			log.Printf("replied on %s", replyTo)
		}
	}
}