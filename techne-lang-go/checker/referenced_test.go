// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package checker_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/source"
)

// declared is a file of declarations whose comments also write their names as words: a type, a
// field, two methods, a variable, a group of constants with a doc comment, a group of types and
// a function with a block doc comment.
const declared = `package p

// Pipeline is a list of steps. See [Pipeline.runs], [runs] and [*Pipeline].
type Pipeline struct {
	// steps are the steps that runs calls.
	steps []func()
}

// runs calls each step, runs no step twice, and returns how many steps it runs.
func (p *Pipeline) runs() int {
	for _, step := range p.steps {
		step()
	}
	return len(p.steps)
}

// Run runs the steps. It calls [Pipeline.runs] and not [a runs], [1x.runs], [Piperuns] or
// [Pipeline.runs twice].
func (p *Pipeline) Run() int { return /* the count */p.runs() }

// limit is the limit of runs.
var limit = 3

// most and least are the bounds of a run.
const (
	// most is the most runs.
	most = 4
)

type (
	// Stage is one step of a run.
	Stage func()
)

/* Count is the number of runs. */
func Count() int {
	//] is no doc link.
	return /* the limit */limit + most
}
`

// wordy is Go source with a comment that writes runs as a word. The file of another language
// and the Go file that does not parse start with it.
const wordy = "package p\n\n// runs is a word.\n"

// The files of the module that [declared] is in: the Go file, a file of another language and a
// Go file that does not parse.
const (
	declaredFile = "a.go"
	textFile     = "a.txt"
	brokenFile   = "broken.go"
)

func TestReferenced(t *testing.T) {
	t.Parallel()

	files := map[string]string{
		declaredFile: declared,
		textFile:     wordy,
		brokenFile:   wordy + "func (\n",
	}
	word := strings.Index(wordy, "runs")

	t.Run("Referenced", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			old  string
			give []string
			want []string
		}{
			{
				name: "keeps the edit of the name of the declaration",
				old:  "runs",
				give: []string{"Pipeline) runs"},
				want: []string{"Pipeline) runs"},
			},
			{
				name: "keeps the edit of a use right after a comment",
				old:  "runs",
				give: []string{"*/p.runs"},
				want: []string{"*/p.runs"},
			},
			{
				name: "keeps the edit of a name that starts where a comment ends",
				old:  "limit",
				give: []string{"*/limit"},
				want: []string{"*/limit"},
			},
			{
				name: "keeps the edit of the word that starts the doc comment of the declaration",
				old:  "runs",
				give: []string{"// runs", "Pipeline) runs"},
				want: []string{"// runs", "Pipeline) runs"},
			},
			{
				name: "leaves out the edits of the other words of the doc comment",
				old:  "runs",
				give: []string{"step, runs", "steps it runs", "Pipeline) runs"},
				want: []string{"Pipeline) runs"},
			},
			{
				name: "keeps the edits of the last names of doc links",
				old:  "runs",
				give: []string{"[Pipeline.runs", "[runs"},
				want: []string{"[Pipeline.runs", "[runs"},
			},
			{
				name: "leaves out the edits of words in brackets that are no doc links",
				old:  "runs",
				give: []string{"[a runs", "[1x.runs", "[Piperuns", "[Pipeline.runs twice"},
			},
			{
				name: "leaves out the edit of the word that starts a doc comment without the edit of its declaration",
				old:  "runs",
				give: []string{"// runs"},
			},
			{
				name: "leaves out an edit that starts at the start of a comment",
				old:  "//",
				give: []string{"// limit"},
			},
			{
				name: "leaves out an edit that starts at the start of a comment before a closing bracket",
				old:  "//",
				give: []string{"//]"},
			},
			{
				name: "keeps the edit of the word that starts the doc comment of a type",
				old:  "Pipeline",
				give: []string{"// Pipeline", "type Pipeline"},
				want: []string{"// Pipeline", "type Pipeline"},
			},
			{
				name: "keeps the edit of the word that starts the doc comment of a field",
				old:  "steps",
				give: []string{"// steps", "\tsteps"},
				want: []string{"// steps", "\tsteps"},
			},
			{
				name: "keeps the edit of the word that starts the doc comment of a variable",
				old:  "limit",
				give: []string{"// limit", "var limit"},
				want: []string{"// limit", "var limit"},
			},
			{
				name: "keeps the edit of the word that starts the doc comment of a constant of a group",
				old:  "most",
				give: []string{"// most is", "\tmost ="},
				want: []string{"// most is", "\tmost ="},
			},
			{
				name: "leaves out the edit of the word that starts the doc comment of a group",
				old:  "most",
				give: []string{"// most and", "// most is", "\tmost ="},
				want: []string{"// most is", "\tmost ="},
			},
			{
				name: "keeps the edit of the word that starts the doc comment of a type of a group",
				old:  "Stage",
				give: []string{"// Stage", "\tStage func"},
				want: []string{"// Stage", "\tStage func"},
			},
			{
				name: "keeps the edit of the word that starts a block doc comment",
				old:  "Count",
				give: []string{"/* Count", "func Count"},
				want: []string{"/* Count", "func Count"},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				got := serving(t, files).Referenced([]edit.Change{rewriting(t, tt.old, tt.give...)})
				if len(tt.want) == 0 {
					assert.Empty(t, got, "the changes that the rename keeps")
					return
				}
				want := []edit.Change{rewriting(t, tt.old, tt.want...)}
				assert.Equal(t, got, want, "the changes that the rename keeps")
			})
		}

		kept := []struct {
			name string
			give edit.Change
		}{
			{
				name: "keeps a change of a file of another language as it is",
				give: edited(textFile, word, word+len("runs")),
			},
			{
				name: "keeps a change of a file that cannot be read as it is",
				give: edited("absent.go", word, word+len("runs")),
			},
			{
				name: "keeps a change of a file that does not parse as it is",
				give: edited(brokenFile, word, word+len("runs")),
			},
			{
				name: "keeps a change of another kind as it is",
				give: edit.Change{Kind: edit.ChangeMove, Path: declaredFile, To: "b.go"},
			},
		}
		for _, tt := range kept {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				got := serving(t, files).Referenced([]edit.Change{tt.give})
				assert.Equal(t, got, []edit.Change{tt.give}, "the changes that the rename keeps")
			})
		}
	})
}

// rewriting returns the change of [declaredFile] that rewrites old at each anchor of
// [declared]: the last old in the first occurrence of the anchor.
func rewriting(t *testing.T, old string, anchors ...string) edit.Change {
	t.Helper()
	out := edit.Change{Kind: edit.ChangeEdit, Path: declaredFile}
	for _, anchor := range anchors {
		held := strings.Index(declared, anchor)
		assert.InRange(t, held, 0, 1<<63, "the index of "+anchor)
		start := held + strings.LastIndex(anchor, old)
		out.Edits = append(out.Edits, edited(declaredFile, start, start+len(old)).Edits...)
	}
	return out
}

// edited returns the change of the file at p that replaces the bytes from start to end with
// the word renamed.
func edited(p source.Path, start, end int) edit.Change {
	return edit.Change{Kind: edit.ChangeEdit, Path: p, Edits: []edit.TextEdit{{
		Span: source.Span{Path: p, Start: source.Position{Offset: start}, End: source.Position{Offset: end}},
		New:  "renamed",
	}}}
}
