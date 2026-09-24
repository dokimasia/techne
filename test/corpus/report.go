// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package corpus

import (
	"fmt"
	"maps"
	"math"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

// Report collects what a run measured.
type Report struct {
	// Budget is the budget of a warm call of a read tool.
	Budget time.Duration

	mu       sync.Mutex
	sections []Section
}

// Section is what a run measured over one repository.
type Section struct {
	// Repository is the entry of the manifest.
	Repository Repository
	// Settled is how long the language server took to answer with total
	// coverage, and zero when it did not within the warmup.
	Settled time.Duration
	// Calls are the tool calls of the session.
	Calls []Call
	// Outcomes are the changes that the run applied or tried.
	Outcomes []Outcome
}

// Outcome is what happened to one change that a run tried.
type Outcome struct {
	// Operation is the name of the write tool.
	Operation string
	// Target is the declaration or the file of the change.
	Target string
	// Result is applied, declined or failed.
	Result string
	// Detail is the reason or the output that the result rests on.
	Detail string
}

// Add appends the section of one repository.
func (r *Report) Add(s Section) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sections = append(r.sections, s)
}

// Markdown returns the report as Markdown. For each repository it states
// when the server settled, and it lists the latency of the warm calls of
// each tool and the outcome of each change. The column of calls over the
// budget counts every tool, and only the read tools fail a run for it.
func (r *Report) Markdown() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	var b strings.Builder
	fmt.Fprintf(&b, "# Corpus report\n\nA warm call of outline, search, resolve or relations fails the run when it"+
		" takes longer than %s.\n", r.Budget)
	for _, s := range r.sections {
		fmt.Fprintf(&b, "\n## %s\n\n", s.Repository.Name)
		fmt.Fprintf(&b, "%s at %s.", s.Repository.Language, pinned(s.Repository))
		if s.Settled > 0 {
			fmt.Fprintf(&b, " The server settled after %s.", s.Settled.Round(time.Millisecond))
		} else {
			b.WriteString(" The server did not settle within the warmup.")
		}
		fmt.Fprintf(&b, "\n\n| Tool | Calls | p50 | p95 | Max | Over %s | Failed | Tiers |\n", r.Budget)
		b.WriteString("|---|---|---|---|---|---|---|---|\n")
		for _, tool := range tools(s.Calls) {
			var took []time.Duration
			over, failed, tiers := 0, 0, map[string]int{}
			for _, c := range s.Calls {
				if c.Tool != tool || !c.Warm {
					continue
				}
				took = append(took, c.Took)
				if c.Took > r.Budget {
					over++
				}
				if c.Failed {
					failed++
				}
				if c.Fidelity != "" {
					tiers[c.Fidelity+" "+c.Completeness]++
				}
			}
			if len(took) == 0 {
				continue
			}
			fmt.Fprintf(&b, "| %s | %d | %s | %s | %s | %d | %d | %s |\n", tool, len(took),
				round(Percentile(took, 50)), round(Percentile(took, 95)), round(Percentile(took, 100)), over, failed,
				counted(tiers))
		}
		if len(s.Outcomes) > 0 {
			b.WriteString("\n| Change | Target | Result | Detail |\n|---|---|---|---|\n")
			for _, o := range s.Outcomes {
				fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", o.Operation, o.Target, o.Result, cell(o.Detail))
			}
		}
	}
	return b.String()
}

// pinned returns the tag and commit of a clone, or the path of a read-only
// repository.
func pinned(r Repository) string {
	if !r.Writable() {
		return r.Path
	}
	return fmt.Sprintf("%s (%s)", r.Tag, r.Commit[:12])
}

// tools returns the names of the tools of calls, sorted.
func tools(calls []Call) []string {
	var out []string
	for _, c := range calls {
		if !slices.Contains(out, c.Tool) {
			out = append(out, c.Tool)
		}
	}
	slices.Sort(out)
	return out
}

// counted returns the counts of tiers, such as "resolved total 6, indexed
// total 2", in the order of the tiers.
func counted(tiers map[string]int) string {
	var out []string
	for _, tier := range slices.Sorted(maps.Keys(tiers)) {
		out = append(out, fmt.Sprintf("%s %d", tier, tiers[tier]))
	}
	return strings.Join(out, ", ")
}

// round rounds a duration to a millisecond.
func round(d time.Duration) time.Duration { return d.Round(time.Millisecond) }

// cell returns text on one line, without the pipes of a Markdown table and
// without terminal escape sequences, cut to 300 bytes.
func cell(text string) string {
	text = strings.NewReplacer("\n", " ", "|", "/").Replace(escapes.ReplaceAllString(text, ""))
	if len(text) > 300 {
		text = strings.ToValidUTF8(text[:300], "") + "…"
	}
	return text
}

// escapes matches the terminal escape sequences that set the colour of a
// build's output.
var escapes = regexp.MustCompile("\x1b\\[[0-9;]*m")

// Percentile returns the value at percentile p of durations by the
// nearest-rank method, and zero for no durations.
func Percentile(durations []time.Duration, p float64) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	sorted := slices.Sorted(slices.Values(durations))
	rank := int(math.Ceil(p / 100 * float64(len(sorted))))
	return sorted[min(max(rank-1, 0), len(sorted)-1)]
}
