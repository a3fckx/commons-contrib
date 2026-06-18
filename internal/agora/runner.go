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
	World        string  `json:"world"`
	BaselineErr  float64 `json:"baselineError"`
	EvolvedErr   float64 `json:"evolvedError"`
	BestFitness  float64 `json:"bestFitness"`
	Generations  int     `json:"generations"`
	Population   int     `json:"population"`
	Improvement  float64 `json:"improvementPct"`
	ReplayPath   string  `json:"replayPath"`
}

var evolveLine = regexp.MustCompile(`best fitness ([0-9.]+) \(error ([0-9.]+)\)`)

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

// RunEvolve executes `python3 -m agora evolve` and returns parsed scores.
func RunEvolve(world string, generations, population int) (EvolveResult, error) {
	root := Root()
	if st, err := os.Stat(filepath.Join(root, "agora")); err != nil || !st.IsDir() {
		return EvolveResult{}, fmt.Errorf("AGORA_ROOT invalid (%s): set env to experiments/agora", root)
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
	cmd := exec.Command("python3", args...)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return EvolveResult{}, fmt.Errorf("evolve %s: %w: %s", world, err, strings.TrimSpace(stderr.String()))
	}

	match := evolveLine.FindStringSubmatch(stdout.String())
	if len(match) != 3 {
		return EvolveResult{}, fmt.Errorf("parse evolve output for %s: %q", world, stdout.String())
	}
	fitness, _ := strconv.ParseFloat(match[1], 64)
	evolved, _ := strconv.ParseFloat(match[2], 64)

	// Prefer fitness of saved rules.evolved.json if present
	if saved, err := evolvedError(root, world); err == nil {
		evolved = saved
	}

	improve := 0.0
	if base > 0 {
		improve = (base - evolved) / base * 100
	}

	return EvolveResult{
		World:       world,
		BaselineErr: round(base, 4),
		EvolvedErr:  round(evolved, 4),
		BestFitness: round(fitness, 4),
		Generations: generations,
		Population:  population,
		Improvement: round(improve, 1),
		ReplayPath:  filepath.Join(root, "worlds", world, "rules.evolved.json"),
	}, nil
}

func round(v float64, places int) float64 {
	p := 1.0
	for i := 0; i < places; i++ {
		p *= 10
	}
	return float64(int(v*p+0.5)) / p
}