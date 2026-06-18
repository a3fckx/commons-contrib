package agora

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// EvolveResult is the objective score from one agora evolve run.
type EvolveResult struct {
	World       string  `json:"world"`
	Mode        string  `json:"mode"`
	BaselineErr float64 `json:"baselineError"`
	EvolvedErr  float64 `json:"evolvedError"`
	BestFitness float64 `json:"bestFitness"`
	Generations int     `json:"generations"`
	Population  int     `json:"population"`
	Improvement float64 `json:"improvementPct"`
	ReplayPath  string  `json:"replayPath"`
	Extra       string  `json:"extra,omitempty"`
}

// OptimizerReport is DSPy compile output (optimize / programs improve).
type OptimizerReport struct {
	World      string  `json:"world"`
	Program    string  `json:"program"`
	Optimizer  string  `json:"optimizer"`
	MeanReward float64 `json:"meanReward,omitempty"`
	Trainset   int     `json:"trainset,omitempty"`
	Saved      string  `json:"saved,omitempty"`
	Error      string  `json:"error,omitempty"`
}

// ProgramEntry from `agora programs list`.
type ProgramEntry struct {
	Name      string `json:"name"`
	Improved  bool   `json:"improved"`
	Signature string `json:"signature"`
}

var (
	evolveLine = regexp.MustCompile(`best fitness ([0-9.]+) \(error ([0-9.]+)\)`)
	jsonBlock  = regexp.MustCompile(`(?s)\{.*\}`)
)

// Root resolves the agora experiments directory (parent of the `agora` python package).
func Root() string {
	if v := os.Getenv("AGORA_ROOT"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, "Desktop", "Attri", "a3fckx", "experiments", "agora"),
		"/Users/a3fckx/Desktop/Attri/a3fckx/experiments/agora",
	}
	for _, c := range candidates {
		if st, err := os.Stat(filepath.Join(c, "agora", "cli.py")); err == nil && !st.IsDir() {
			return c
		}
	}
	return candidates[0]
}

// LLMAvailable checks agora dspy + OpenRouter readiness.
func LLMAvailable() bool {
	root := Root()
	cmd := exec.Command("python3", "-c", "from agora import llm; import json; print(json.dumps({'ok': llm.available()}))")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	var payload struct {
		OK bool `json:"ok"`
	}
	if json.Unmarshal(bytes.TrimSpace(out), &payload) != nil {
		return false
	}
	return payload.OK
}

func baselineError(root, world string) (float64, error) {
	script := fmt.Sprintf(`
import os, json
from agora.sim import load_world_dir
from agora import evolve
root = %q
world = %q
w, r, t = load_world_dir(os.path.join(root, "worlds", world))
if t is None:
    raise SystemExit("no target")
_, err = evolve.fitness(w.to_body(), r.to_body(), t)
print(json.dumps({"error": err}))
`, root, world)
	return runPythonJSON(root, script, "error")
}

func evolvedError(root, world string) (float64, error) {
	path := filepath.Join(root, "worlds", world, "rules.evolved.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var doc struct {
		Body map[string]any `json:"body"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return 0, err
	}
	rules := doc.Body
	if rules == nil {
		var flat map[string]any
		if err := json.Unmarshal(raw, &flat); err != nil {
			return 0, err
		}
		rules = flat
	}
	script := fmt.Sprintf(`
import os, json
from agora.sim import load_world_dir
from agora import evolve
root = %q
world = %q
rules = json.loads(%q)
w, _, t = load_world_dir(os.path.join(root, "worlds", world))
_, err = evolve.fitness(w.to_body(), rules, t)
print(json.dumps({"error": err}))
`, root, world, mustJSON(rules))
	return runPythonJSON(root, script, "error")
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func runPythonJSON(root, script, key string) (float64, error) {
	cmd := exec.Command("python3", "-c", script)
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("agora python: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var payload map[string]float64
	if err := json.Unmarshal(bytes.TrimSpace(out), &payload); err != nil {
		return 0, fmt.Errorf("decode python output: %w (%s)", err, string(out))
	}
	v, ok := payload[key]
	if !ok {
		return 0, fmt.Errorf("missing %q in python output: %s", key, string(out))
	}
	return v, nil
}

func runAgora(args ...string) (stdout string, stderr string, err error) {
	root := Root()
	cmd := exec.Command("python3", args...)
	cmd.Dir = root
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

func parseJSONStdout(stdout string) (map[string]any, error) {
	trim := strings.TrimSpace(stdout)
	if strings.HasPrefix(trim, "{") {
		var m map[string]any
		if err := json.Unmarshal([]byte(trim), &m); err == nil {
			return m, nil
		}
	}
	match := jsonBlock.FindString(trim)
	if match == "" {
		return nil, fmt.Errorf("no json in output: %s", trim)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(match), &m); err != nil {
		return nil, err
	}
	return m, nil
}

// RunEvolve executes numeric or LLM-assisted evolve.
func RunEvolve(world string, generations, population int, useLLM bool) (EvolveResult, error) {
	root := Root()
	if st, err := os.Stat(filepath.Join(root, "agora")); err != nil || !st.IsDir() {
		return EvolveResult{}, fmt.Errorf("AGORA_ROOT invalid (%s)", root)
	}

	mode := "numeric_evolve"
	if useLLM {
		mode = "llm_evolve"
		if !LLMAvailable() {
			return EvolveResult{World: world, Mode: mode, Extra: "skipped: no LLM key"}, fmt.Errorf("LLM unavailable")
		}
	}

	base, err := baselineError(root, world)
	if err != nil {
		return EvolveResult{}, fmt.Errorf("%s baseline: %w", world, err)
	}

	args := []string{"-m", "agora", "evolve",
		"--world", world,
		"--generations", strconv.Itoa(generations),
		"--population", strconv.Itoa(population),
	}
	if useLLM {
		args = append(args, "--llm")
	}
	stdout, stderr, err := runAgora(args...)
	if err != nil {
		return EvolveResult{}, fmt.Errorf("evolve %s: %w: %s", world, err, strings.TrimSpace(stderr))
	}

	match := evolveLine.FindStringSubmatch(stdout)
	if len(match) != 3 {
		return EvolveResult{}, fmt.Errorf("parse evolve output for %s: %q", world, stdout)
	}
	fitness, _ := strconv.ParseFloat(match[1], 64)
	evolved, _ := strconv.ParseFloat(match[2], 64)
	if saved, err := evolvedError(root, world); err == nil {
		evolved = saved
	}
	improve := 0.0
	if base > 0 {
		improve = (base - evolved) / base * 100
	}
	return EvolveResult{
		World: world, Mode: mode,
		BaselineErr: round(base, 4), EvolvedErr: round(evolved, 4),
		BestFitness: round(fitness, 4), Generations: generations, Population: population,
		Improvement: round(improve, 1),
		ReplayPath:  filepath.Join(root, "worlds", world, "rules.evolved.json"),
	}, nil
}

// RunOptimizeProposer runs `agora optimize` (DSPy compile proposer vs sim fitness).
func RunOptimizeProposer(world, optimizer string) (OptimizerReport, error) {
	if !LLMAvailable() {
		return OptimizerReport{World: world, Program: "propose_rule", Optimizer: optimizer, Error: "no LLM"}, fmt.Errorf("LLM unavailable")
	}
	stdout, stderr, err := runAgora("-m", "agora", "optimize", "--world", world, "--optimizer", optimizer)
	if err != nil {
		return OptimizerReport{World: world, Program: "optimize_proposer", Optimizer: optimizer, Error: err.Error()}, err
	}
	payload, perr := parseJSONStdout(stdout)
	if perr != nil {
		return OptimizerReport{World: world, Program: "optimize_proposer", Optimizer: optimizer, Error: perr.Error() + " stderr:" + stderr}, perr
	}
	rep := OptimizerReport{World: world, Program: "optimize_proposer", Optimizer: optimizer}
	if v, ok := payload["mean_reward"].(float64); ok {
		rep.MeanReward = round(v, 4)
	}
	if v, ok := payload["trainset_size"].(float64); ok {
		rep.Trainset = int(v)
	}
	if v, ok := payload["saved"].(string); ok {
		rep.Saved = v
	}
	return rep, nil
}

// RunImproveProgram runs `agora programs improve` for simulation-grounded propose_rule.
func RunImproveProgram(world, program, optimizer string) (OptimizerReport, error) {
	if !LLMAvailable() {
		return OptimizerReport{World: world, Program: program, Optimizer: optimizer, Error: "no LLM"}, fmt.Errorf("LLM unavailable")
	}
	args := []string{"-m", "agora", "programs", "improve", "--name", program, "--optimizer", optimizer}
	if world != "" {
		args = append(args, "--world", world)
	}
	stdout, stderr, err := runAgora(args...)
	if err != nil {
		return OptimizerReport{World: world, Program: program, Optimizer: optimizer, Error: err.Error()}, err
	}
	payload, perr := parseJSONStdout(stdout)
	if perr != nil {
		return OptimizerReport{World: world, Program: program, Optimizer: optimizer, Error: perr.Error() + " stderr:" + stderr}, perr
	}
	rep := OptimizerReport{World: world, Program: program, Optimizer: optimizer}
	if v, ok := payload["error"].(string); ok {
		rep.Error = v
		return rep, fmt.Errorf("%s", v)
	}
	if v, ok := payload["trainset"].(float64); ok {
		rep.Trainset = int(v)
	}
	if v, ok := payload["saved"].(string); ok {
		rep.Saved = v
	}
	return rep, nil
}

// ListPrograms returns agora DSPy program registry lines.
func ListPrograms() ([]ProgramEntry, error) {
	stdout, _, err := runAgora("-m", "agora", "programs", "list")
	if err != nil {
		return nil, err
	}
	var entries []ProgramEntry
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		body := strings.TrimSpace(strings.TrimPrefix(line, "- "))
		bracket := strings.Index(body, "[")
		if bracket <= 0 {
			continue
		}
		name := strings.TrimSpace(body[:bracket])
		improved := strings.Contains(body, "[improved]")
		sig := ""
		if i := strings.Index(body, "->"); i >= 0 {
			sig = strings.TrimSpace(body[i+2:])
		}
		entries = append(entries, ProgramEntry{Name: name, Improved: improved, Signature: sig})
	}
	return entries, nil
}

func round(v float64, places int) float64 {
	p := 1.0
	for i := 0; i < places; i++ {
		p *= 10
	}
	return float64(int(v*p+0.5)) / p
}