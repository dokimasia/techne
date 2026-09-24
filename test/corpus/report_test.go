// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package corpus_test

import (
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/test/corpus"
)

func TestReport(t *testing.T) {
	t.Parallel()

	t.Run("Percentile", func(t *testing.T) {
		t.Parallel()
		ten := make([]time.Duration, 10)
		for i := range ten {
			ten[i] = time.Duration(10-i) * time.Second
		}

		tests := []struct {
			name  string
			given []time.Duration
			p     float64
			want  time.Duration
		}{
			{name: "returns the median by the nearest rank", given: ten, p: 50, want: 5 * time.Second},
			{name: "returns the largest value at percentile 95 of ten", given: ten, p: 95, want: 10 * time.Second},
			{name: "returns the largest value at percentile 100", given: ten, p: 100, want: 10 * time.Second},
			{name: "returns the one value of one", given: ten[:1], p: 50, want: 10 * time.Second},
			{name: "returns zero for no values", given: nil, p: 50, want: 0},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, corpus.Percentile(tt.given, tt.p), tt.want, "the percentile")
			})
		}
	})

	t.Run("Markdown", func(t *testing.T) {
		t.Parallel()
		clone := corpus.Repository{
			Name: "alpha", Language: "go", URL: "https://example.com/alpha.git", Tag: "v1.0.0",
			Commit: "f78e722310e50bcaca9276be22276d9e91d91308",
		}
		report := &corpus.Report{Budget: 2 * time.Second}
		report.Add(corpus.Section{
			Repository: clone,
			Settled:    90 * time.Second,
			Calls: []corpus.Call{
				{Tool: "outline", Took: 10 * time.Second},
				{Tool: "outline", Took: time.Second, Warm: true, Fidelity: "resolved", Completeness: "total"},
				{
					Tool: "outline", Took: 3 * time.Second, Warm: true, Failed: true,
					Fidelity: "indexed", Completeness: "total",
				},
			},
			Outcomes: []corpus.Outcome{
				{Operation: "rename.symbol", Target: "Store", Result: "applied", Detail: "3 files, built"},
				{
					Operation: "move.file", Target: "a.js", Result: "failed",
					Detail: "\x1b[31mCould not\nresolve\x1b[0m a|b",
				},
			},
		})
		report.Add(corpus.Section{Repository: corpus.Repository{Name: "beta", Language: "go", Path: "../beta"}})
		written := report.Markdown()

		t.Run("lists the warm calls of each tool", func(t *testing.T) {
			t.Parallel()
			assert.Contains(t, written, "| outline | 2 | 1s | 3s | 3s | 1 | 1 | indexed total 1, resolved total 1 |",
				"the row of outline")
		})

		t.Run("lists the outcome of each change", func(t *testing.T) {
			t.Parallel()
			assert.Contains(t, written, "| rename.symbol | Store | applied | 3 files, built |", "the row of the rename")
		})

		t.Run("writes the detail of a change as plain text on one line", func(t *testing.T) {
			t.Parallel()
			assert.Contains(t, written, "| move.file | a.js | failed | Could not resolve a/b |", "the row of the move")
		})

		t.Run("states when the server settled", func(t *testing.T) {
			t.Parallel()
			assert.Contains(t, written, "The server settled after 1m30s.", "the settle time of alpha")
		})

		t.Run("states a server that did not settle", func(t *testing.T) {
			t.Parallel()
			assert.Contains(t, written, "The server did not settle within the warmup.", "the settle time of beta")
		})

		t.Run("names the commit of a clone", func(t *testing.T) {
			t.Parallel()
			assert.Contains(t, written, "v1.0.0 (f78e722310e5)", "the pin of alpha")
		})
	})
}
