package main

import (
	"log"
	"os"
	"strconv"

	"github.com/a3fckx/commons-contrib/internal/commons"
	"github.com/a3fckx/commons-contrib/internal/watcher"
)

func main() {
	node := commons.DefaultNode
	author := "thread-curator"
	engageLimit := 2
	threshold := 5.0
	channel := "origin-room"
	skipEngage := os.Getenv("CONVERSATION_SKIP_ENGAGE") == "1"

	if v := os.Getenv("SOURCEKIND_NODE"); v != "" {
		node = v
	}
	if v := os.Getenv("THREAD_CURATOR_AUTHOR"); v != "" {
		author = v
	}
	if v := os.Getenv("CONVERSATION_ENGAGE_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			engageLimit = n
		}
	}
	if v := os.Getenv("CONSOLIDATION_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			threshold = f
		}
	}
	if v := os.Getenv("CONSOLIDATION_CHANNEL"); v != "" {
		channel = v
	}

	client := commons.NewClient(node, author)
	cfg := watcher.Config{
		Author:                 author,
		EngageLimit:            engageLimit,
		ConsolidationThreshold: threshold,
		ConsolidationChannel:   channel,
		StatePath:              os.Getenv("CONVERSATION_STATE_PATH"),
		SkipEngage:             skipEngage,
	}

	res, err := watcher.Run(client, cfg)
	if err != nil {
		log.Fatalf("conversation loop: %v", err)
	}

	log.Printf("scanned %d hot posts, %d consolidation candidates", res.Scanned, res.Candidates)

	if res.Consolidated != nil {
		kind := "consolidated"
		if res.Consolidated.Updated {
			kind = "updated"
		}
		log.Printf("%s %s -> %s (score %.1f, reasons: %v)",
			kind,
			res.Consolidated.SourcePostID,
			res.Consolidated.OutputPostID,
			res.Consolidated.Score,
			res.Consolidated.Reasons,
		)
	} else if res.ConsolidateError != "" {
		log.Printf("consolidation error: %s", res.ConsolidateError)
	} else {
		log.Printf("no thread met consolidation threshold (%.1f)", threshold)
	}

	if res.Engage != nil {
		log.Printf("engaged in room %s (%d actions)", res.Engage.WorkspaceRoomID, len(res.Engage.Actions))
		for _, a := range res.Engage.Actions {
			log.Printf("  %s %s %s", a.PostID, a.Status, a.Rationale)
		}
	} else if res.EngageError != "" {
		log.Printf("engage error: %s", res.EngageError)
	}
}