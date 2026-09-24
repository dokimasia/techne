// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/lang"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// reporting is how long a question waits for a server without pull diagnostics to publish
// the diagnostics of the files it was given. One deadline covers every file of a question.
const reporting = 2 * time.Second

// diagnostics returns the diagnostics of the server for the file at p, and reports whether
// the server reported on p. It asks a server with pull diagnostics. For a server without them
// it waits until by for the server to publish.
func (e *Engine) diagnostics(
	ctx context.Context,
	held *session,
	p source.Path,
	by time.Time,
) ([]protocol.Diagnostic, bool, error) {
	if held.capable.DiagnosticProvider != nil {
		reported, _, err := e.pull(ctx, held, p)
		return reported, true, err
	}
	reported, said := held.reports.wait(ctx, uri.File(e.fullPath(p)), by)
	return reported, said, nil
}

// pull asks the server for the diagnostics of the file at p with textDocument/diagnostic, and
// reports whether the reply is a full report. A full report replaces the diagnostics that the
// session keeps for p. An unchanged report contains no diagnostics: a server sends one only in
// reply to a previous result id, and pull sends none.
func (e *Engine) pull(
	ctx context.Context,
	held *session,
	p source.Path,
) ([]protocol.Diagnostic, bool, error) {
	of := uri.File(e.fullPath(p))
	answered, err := held.asks.Diagnostic(ctx, &protocol.DocumentDiagnosticParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: of},
	})
	if err != nil {
		return nil, false, fmt.Errorf("lsp: %s: diagnostics of %s: %w", e.server.Name, p, err)
	}
	full, isFull := answered.(*protocol.RelatedFullDocumentDiagnosticReport)
	if !isFull {
		return nil, false, nil
	}
	held.reports.keep(of, full.Items)
	return full.Items, true, nil
}

// dependents returns the findings of workspace/diagnostic for every file of the workspace
// except the paths in changed, sorted by path. It skips a file outside the workspace and a
// file that [lang.Readable] refuses, such as a generated file.
func (e *Engine) dependents(
	ctx context.Context,
	held *session,
	changed []source.Path,
) ([]edit.Finding, error) {
	answered, err := held.asks.DiagnosticWorkspace(ctx, &protocol.WorkspaceDiagnosticParams{
		PreviousResultIds: []protocol.PreviousResultId{},
	})
	if err != nil {
		return nil, fmt.Errorf("lsp: %s: workspace diagnostics: %w", e.server.Name, err)
	}
	if answered == nil {
		return nil, nil
	}

	var out []edit.Finding
	for _, one := range answered.Items {
		full, isFull := one.(*protocol.WorkspaceFullDocumentDiagnosticReport)
		if !isFull || len(full.Items) == 0 {
			continue
		}
		p := e.pathOf(full.URI)
		if outside(p) || slices.Contains(changed, p) {
			continue
		}
		doc, err := e.read(p)
		if err != nil {
			continue
		}
		for _, d := range full.Items {
			out = append(out, edit.Finding{Diagnostic: found(d, doc)})
		}
	}
	slices.SortStableFunc(out, func(a, b edit.Finding) int {
		return cmp.Compare(a.Diagnostic.Span.Path, b.Diagnostic.Span.Path)
	})
	return out, nil
}

// found converts a protocol diagnostic of doc into a [diag.Diagnostic], with the source line
// it starts on as its snippet.
func found(one protocol.Diagnostic, doc document) diag.Diagnostic {
	span := doc.span(one.Range)
	source, _ := one.Source.Get()
	return diag.Diagnostic{
		Severity: graded(one.Severity),
		Message:  messaged(one.Message),
		Span:     span,
		Source:   source,
		Code:     coded(one.Code),
		Snippet:  doc.sourceLine(span),
	}
}

// messaged returns the text of a diagnostic message, which is a string or, since LSP 3.18,
// markup content.
func messaged(message protocol.InlayHintTooltip) string {
	switch held := message.(type) {
	case protocol.String:
		return string(held)
	case *protocol.MarkupContent:
		return held.Value
	}
	return ""
}

// grades maps each protocol severity to a [diag.Severity]. A diagnostic without a severity
// keeps [diag.SeverityUnset], which a filter for errors does not drop.
var grades = map[protocol.DiagnosticSeverity]diag.Severity{
	protocol.DiagnosticSeverityError:       diag.SeverityError,
	protocol.DiagnosticSeverityWarning:     diag.SeverityWarning,
	protocol.DiagnosticSeverityInformation: diag.SeverityInfo,
	protocol.DiagnosticSeverityHint:        diag.SeverityHint,
}

// graded returns the [diag.Severity] of a protocol severity.
func graded(severity protocol.DiagnosticSeverity) diag.Severity { return grades[severity] }

// coded returns the code of a diagnostic, which the protocol sends as a string or an integer,
// as text.
func coded(code protocol.ProgressToken) string {
	switch held := code.(type) {
	case protocol.String:
		return string(held)
	case protocol.Integer:
		return fmt.Sprint(int32(held))
	}
	return ""
}

// reports keeps the latest diagnostics that a server reported for each file, published or
// pulled. A report replaces the previous one, because a publish and a full pull report both
// contain every diagnostic of the file. reports is safe for concurrent use.
type reports struct {
	mu   sync.Mutex
	kept map[uri.URI][]protocol.Diagnostic
	// waking has one channel per file that a caller waits on, which the next report closes.
	waking map[uri.URI]chan struct{}
}

// newReports returns an empty store.
func newReports() *reports {
	return &reports{kept: map[uri.URI][]protocol.Diagnostic{}, waking: map[uri.URI]chan struct{}{}}
}

// keep stores the diagnostics of the file of, and wakes the callers that wait for them.
func (r *reports) keep(of uri.URI, diagnostics []protocol.Diagnostic) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.kept[of] = slices.Clone(diagnostics)
	if waking, waiting := r.waking[of]; waiting {
		close(waking)
		delete(r.waking, of)
	}
}

// forget drops the diagnostics of the file of. The engine calls it when it replaces the buffer
// of the file, because the diagnostics describe the replaced content.
func (r *reports) forget(of uri.URI) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.kept, of)
}

// errors returns the zero-based start lines of the kept diagnostics of error severity, by the
// workspace path of their file, for the files in the directory within. at maps a URI to its
// workspace path. A server that reported nothing returns an empty map.
func (r *reports) errors(within source.Path, at func(uri.URI) source.Path) map[source.Path][]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[source.Path][]int{}
	for of, diagnostics := range r.kept {
		p := at(of)
		if !lang.Within(p, within) {
			continue
		}
		for _, one := range diagnostics {
			if one.Severity == protocol.DiagnosticSeverityError {
				out[p] = append(out[p], int(one.Range.Start.Line))
			}
		}
	}
	return out
}

// wait returns the diagnostics of the file of, and reports whether the server reported on it.
// It waits for a report until by or until ctx ends, and returns at once when a report is kept
// or by has passed.
func (r *reports) wait(ctx context.Context, of uri.URI, by time.Time) ([]protocol.Diagnostic, bool) {
	r.mu.Lock()
	if kept, said := r.kept[of]; said {
		r.mu.Unlock()
		return kept, true
	}
	left := time.Until(by)
	if left <= 0 {
		r.mu.Unlock()
		return nil, false
	}
	waking, waiting := r.waking[of]
	if !waiting {
		waking = make(chan struct{})
		r.waking[of] = waking
	}
	r.mu.Unlock()

	timer := time.NewTimer(left)
	defer timer.Stop()
	select {
	case <-waking:
	case <-timer.C:
		return nil, false
	case <-ctx.Done():
		return nil, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	kept, said := r.kept[of]
	return kept, said
}

// PublishDiagnostics keeps the diagnostics that the server published for a file.
func (a answers) PublishDiagnostics(_ context.Context, params *protocol.PublishDiagnosticsParams) error {
	a.reports.keep(params.URI, params.Diagnostics)
	return nil
}
