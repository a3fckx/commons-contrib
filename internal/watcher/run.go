package watcher

import (
	"fmt"
	"sort"

	"github.com/a3fckx/commons-contrib/internal/commons"
)

type Result struct {
	Scanned          int
	Candidates       int
	Consolidated     *ConsolidationOutcome
	Engage           *commons.EngageResponse
	EngageError      string
	ConsolidateError string
}

type ConsolidationOutcome struct {
	SourcePostID string
	OutputPostID string
	Score        float64
	Updated      bool
	Reasons      []string
}

func Run(client *commons.Client, cfg Config) (Result, error) {
	if cfg.EngageLimit <= 0 {
		cfg.EngageLimit = 2
	}
	if cfg.ConsolidationThreshold <= 0 {
		cfg.ConsolidationThreshold = 5.0
	}
	if cfg.ConsolidationChannel == "" {
		cfg.ConsolidationChannel = "origin-room"
	}

	statePath := cfg.StatePath
	if statePath == "" {
		var err error
		statePath, err = DefaultStatePath()
		if err != nil {
			return Result{}, err
		}
	}

	state, err := LoadState(statePath)
	if err != nil {
		return Result{}, fmt.Errorf("load state: %w", err)
	}

	posts, err := client.Feed("global", "hot", 50)
	if err != nil {
		return Result{}, fmt.Errorf("feed: %w", err)
	}

	result := Result{Scanned: len(posts)}
	var candidates []ScoredThread

	for _, p := range posts {
		if IsOwnContent(cfg.Author, p.Author) {
			continue
		}
		if stringsHasPrefixConsolidated(p.Content) {
			continue
		}

		st := ScoreThread(p)
		if st.Score < cfg.ConsolidationThreshold || !st.Ready {
			continue
		}

		updated := state.NeedsUpdate(p.ID, len(p.Responses))
		if state.WasConsolidated(p.ID) && !updated {
			continue
		}

		st.Reasons = append(st.Reasons, fmt.Sprintf("threshold %.1f met", cfg.ConsolidationThreshold))
		candidates = append(candidates, st)
	}

	result.Candidates = len(candidates)

	if len(candidates) > 0 {
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].Score > candidates[j].Score
		})
		best := candidates[0]
		updated := state.NeedsUpdate(best.Post.ID, len(best.Post.Responses))

		content := FormatConsolidation(best.Post, best, updated)
		topics := ConsolidationTopics(best.Post)

		post, err := client.PostRich(commons.PostRequest{
			Author:     cfg.Author,
			Content:    content,
			ChannelID:  cfg.ConsolidationChannel,
			AgentID:    cfg.Author,
			Kind:       "mdx",
			Topics:     topics,
			RenderMeta: map[string]any{
				"schema": "sourcekind.synthesis.v1",
				"title":  firstLine(best.Post.Content),
			},
		})
		if err != nil {
			result.ConsolidateError = err.Error()
		} else {
			state.Mark(best.Post.ID, post.ID, len(best.Post.Responses))
			if err := state.Save(); err != nil {
				result.ConsolidateError = fmt.Sprintf("posted %s but state save failed: %v", post.ID, err)
			}
			result.Consolidated = &ConsolidationOutcome{
				SourcePostID: best.Post.ID,
				OutputPostID: post.ID,
				Score:        best.Score,
				Updated:      updated,
				Reasons:      best.Reasons,
			}
			mirror := FormatThreadMirror(best.Post, best, post.ID)
			if _, err := client.Reply(best.Post.ID, mirror); err != nil {
				result.ConsolidateError = fmt.Sprintf("consolidated %s but thread mirror failed: %v", post.ID, err)
			}
		}
	}

	if !cfg.SkipEngage {
		eng, err := client.Engage(cfg.EngageLimit)
		if err != nil {
			result.EngageError = err.Error()
		} else {
			result.Engage = eng
		}
	}

	return result, nil
}

func stringsHasPrefixConsolidated(content string) bool {
	return len(content) > 14 && (content[:14] == "# Consolidated" || content[:18] == "# Updated synthesis")
}