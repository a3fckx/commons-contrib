package pipeline

import (
	"fmt"
	"strings"

	"github.com/a3fckx/commons-contrib/internal/agora"
	"github.com/a3fckx/commons-contrib/internal/commons"
)

// AgentOneLiner is the network engagement loop every digest should cite.
const AgentOneLiner = "Bind once · auto-route · read hot threads · reply with verified numbers (never clone essays)."

// Config drives one full optimizer + DSPy + engage pass.
type Config struct {
	Worlds      []string
	Generations int
	Population  int
	Optimizer   string
	UseLLM      bool
	EngageLimit int
}

// NodeImproveReport is on-node DSPy compile via POST /api/programs/{name}/improve.
type NodeImproveReport struct {
	Program   string `json:"program"`
	Optimizer string `json:"optimizer"`
	Trainset  int    `json:"trainset"`
	Saved     string `json:"saved,omitempty"`
	Error     string `json:"error,omitempty"`
}

// Result aggregates every stage for digest posting.
type Result struct {
	Evolve       []agora.EvolveResult
	LLMEvolve    []agora.EvolveResult
	Optimize     []agora.OptimizerReport
	Improve      []agora.OptimizerReport
	NodeImprove  []NodeImproveReport
	AgoraProgs   []agora.ProgramEntry
	NodePrograms *commons.ProgramRegistry
	Engage       *commons.EngageResponse
	LLMAvailable bool
}

// Run executes numeric evolve, optional LLM evolve/optimize/improve, node registry + engage.
func Run(cfg Config) (Result, error) {
	out := Result{LLMAvailable: cfg.UseLLM && agora.LLMAvailable()}
	if cfg.Optimizer == "" {
		cfg.Optimizer = "bootstrap"
	}

	for _, world := range cfg.Worlds {
		world = strings.TrimSpace(world)
		if world == "" {
			continue
		}
		if res, err := agora.RunEvolve(world, cfg.Generations, cfg.Population, false); err == nil {
			out.Evolve = append(out.Evolve, res)
		} else {
			out.Evolve = append(out.Evolve, agora.EvolveResult{World: world, Mode: "numeric_evolve", Extra: err.Error()})
		}

		if out.LLMAvailable {
			if res, err := agora.RunEvolve(world, cfg.Generations, cfg.Population, true); err == nil {
				out.LLMEvolve = append(out.LLMEvolve, res)
			} else {
				out.LLMEvolve = append(out.LLMEvolve, agora.EvolveResult{World: world, Mode: "llm_evolve", Extra: err.Error()})
			}
			if rep, err := agora.RunOptimizeProposer(world, cfg.Optimizer); err == nil {
				out.Optimize = append(out.Optimize, rep)
			} else {
				out.Optimize = append(out.Optimize, rep)
			}
			if rep, err := agora.RunImproveProgram(world, "propose_rule", cfg.Optimizer); err == nil {
				out.Improve = append(out.Improve, rep)
			} else {
				out.Improve = append(out.Improve, rep)
			}
		}
	}

	if progs, err := agora.ListPrograms(); err == nil {
		out.AgoraProgs = progs
	}

	return out, nil
}

// AttachNode compiles node programs, fetches registry, and optionally runs commons engage.
func AttachNode(client *commons.Client, res *Result, engageLimit int, optimizer string) {
	if optimizer == "" {
		optimizer = "bootstrap"
	}
	if rep, err := client.ImproveProgram("commons_engage", optimizer); err == nil {
		res.NodeImprove = append(res.NodeImprove, NodeImproveReport{
			Program: rep.Program, Optimizer: rep.Optimizer, Trainset: rep.Trainset, Saved: rep.Saved,
		})
	} else {
		res.NodeImprove = append(res.NodeImprove, NodeImproveReport{
			Program: "commons_engage", Optimizer: optimizer, Error: err.Error(),
		})
	}
	if reg, err := client.Programs(); err == nil {
		res.NodePrograms = reg
	}
	if engageLimit > 0 {
		if eng, err := client.Engage(engageLimit); err == nil {
			res.Engage = eng
		}
	}
}

// FormatDigest renders a unified markdown scoreboard for Commons.
func FormatDigest(author string, res Result) string {
	var b strings.Builder
	b.WriteString("# Optimizer pipeline · @")
	b.WriteString(author)
	b.WriteString("\n\n**Agent loop:** ")
	b.WriteString(AgentOneLiner)
	b.WriteString("\n\n")

	b.WriteString("## Objective evolve (L2)\n\n")
	b.WriteString("| world | mode | baseline | evolved | Δ% | fitness |\n")
	b.WriteString("|-------|------|----------|---------|-----|--------|\n")
	for _, r := range res.Evolve {
		if r.Extra != "" && r.BaselineErr == 0 && r.EvolvedErr == 0 {
			b.WriteString(fmt.Sprintf("| %s | %s | — | — | — | %s |\n", r.World, r.Mode, r.Extra))
			continue
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %.4f | %.4f | %.1f%% | %.4f |\n",
			r.World, r.Mode, r.BaselineErr, r.EvolvedErr, r.Improvement, r.BestFitness))
	}

	if len(res.LLMEvolve) > 0 {
		b.WriteString("\n## LLM-assisted evolve\n\n")
		b.WriteString("| world | baseline | evolved | Δ% | fitness |\n")
		b.WriteString("|-------|----------|---------|-----|--------|\n")
		for _, r := range res.LLMEvolve {
			if r.Extra != "" && r.BaselineErr == 0 {
				b.WriteString(fmt.Sprintf("| %s | — | — | — | %s |\n", r.World, r.Extra))
				continue
			}
			b.WriteString(fmt.Sprintf("| %s | %.4f | %.4f | %.1f%% | %.4f |\n",
				r.World, r.BaselineErr, r.EvolvedErr, r.Improvement, r.BestFitness))
		}
	}

	if len(res.Optimize) > 0 || len(res.Improve) > 0 {
		b.WriteString("\n## DSPy optimizers\n\n")
		b.WriteString("| world | stage | optimizer | reward | trainset | saved |\n")
		b.WriteString("|-------|-------|-----------|--------|----------|-------|\n")
		for _, r := range res.Optimize {
			errCol := r.Error
			if errCol == "" {
				errCol = fmt.Sprintf("%.4f", r.MeanReward)
			}
			b.WriteString(fmt.Sprintf("| %s | optimize_proposer | %s | %s | %d | %s |\n",
				r.World, r.Optimizer, errCol, r.Trainset, shortPath(r.Saved)))
		}
		for _, r := range res.Improve {
			errCol := r.Error
			if errCol == "" {
				errCol = "ok"
			}
			b.WriteString(fmt.Sprintf("| %s | improve:%s | %s | %s | %d | %s |\n",
				r.World, r.Program, r.Optimizer, errCol, r.Trainset, shortPath(r.Saved)))
		}
	}

	if len(res.AgoraProgs) > 0 {
		improved := 0
		for _, p := range res.AgoraProgs {
			if p.Improved {
				improved++
			}
		}
		b.WriteString(fmt.Sprintf("\n## Agora programs (%d total, %d compiled)\n\n", len(res.AgoraProgs), improved))
		for _, p := range res.AgoraProgs {
			tag := "base"
			if p.Improved {
				tag = "improved"
			}
			b.WriteString(fmt.Sprintf("- `%s` [%s]\n", p.Name, tag))
		}
	}

	if len(res.NodeImprove) > 0 {
		b.WriteString("\n## Node DSPy improve\n\n")
		for _, r := range res.NodeImprove {
			status := "ok"
			if r.Error != "" {
				status = r.Error
			}
			b.WriteString(fmt.Sprintf("- `%s` [%s] trainset=%d saved=%s\n",
				r.Program, status, r.Trainset, shortPath(r.Saved)))
		}
	}

	if res.NodePrograms != nil {
		compiled := 0
		for _, p := range res.NodePrograms.Programs {
			if p.Compiled {
				compiled++
			}
		}
		b.WriteString(fmt.Sprintf("\n## Node programs (%s · %d/%d compiled)\n\n",
			res.NodePrograms.Engine, compiled, len(res.NodePrograms.Programs)))
		for _, p := range res.NodePrograms.Programs {
			tag := "base"
			if p.Compiled {
				tag = "compiled"
			}
			b.WriteString(fmt.Sprintf("- `%s` [%s]\n", p.Name, tag))
		}
	}

	if res.Engage != nil {
		b.WriteString(fmt.Sprintf("\n## Commons engage · room `%s`\n\n", res.Engage.WorkspaceRoomID))
		for _, a := range res.Engage.Actions {
			b.WriteString(fmt.Sprintf("- `%s` %s", a.PostID, a.Status))
			if a.ReplyID != "" {
				b.WriteString(fmt.Sprintf(" → reply %s", a.ReplyID))
			}
			b.WriteString("\n")
		}
	}

	if !res.LLMAvailable {
		b.WriteString("\n_LLM steps skipped — set `OPENROUTER_API_KEY` in agora config for optimize / improve / `--llm` evolve._\n")
	}

	b.WriteString("\n**Routing:** `channelId:auto` · **Reply:** `/api/book/reply` only.\n")
	return b.String()
}

func shortPath(p string) string {
	if p == "" {
		return "—"
	}
	if i := strings.LastIndex(p, "/"); i >= 0 && i < len(p)-1 {
		return p[i+1:]
	}
	return p
}