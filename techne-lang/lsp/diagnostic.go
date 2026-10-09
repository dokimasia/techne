// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strconv"
	"sync"
	"time"

	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
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
// it waits until by for the server to publish. For the file on disk, which ondisk reports, the
// diagnostics of a server that declares [Server.DiskCheck] add the findings of its check on
// disk, which the server publishes.
func (e *Engine) diagnostics(
	ctx context.Context,
	held *session,
	p source.Path,
	by time.Time,
	ondisk bool,
) ([]protocol.Diagnostic, bool, error) {
	of := uri.File(e.fullPath(p))
	if held.capable.DiagnosticProvider == nil {
		reported, said := held.reports.wait(ctx, of, by)
		return reported, said, nil
	}
	reported, _, err := e.pull(ctx, held, p)
	if err != nil {
		return nil, false, err
	}
	if ondisk && e.server.DiskCheck != "" {
		reported = append(reported, held.reports.published(of)...)
	}
	return reported, true, nil
}

// pull asks the server for the diagnostics of the file at p with textDocument/diagnostic, and
// reports whether the reply is a full report. A full report replaces the pulled diagnostics
// that the session keeps for p. An unchanged report contains no diagnostics: a server sends
// one only in reply to a previous result id, and pull sends none.
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
	held.reports.pulled(of, full.Items)
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
		if lang.Outside(p) || slices.Contains(changed, p) {
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
		return strconv.Itoa(int(held))
	}
	return ""
}

// reports keeps the latest diagnostics that a server reported for each file, in two kinds: the
// diagnostics that it published, and the diagnostics of its reply to textDocument/diagnostic.
// A report replaces the previous report of its kind, because a publish and a full pull report
// each contain every diagnostic of the file that they cover. reports is safe for concurrent
// use.
type reports struct {
	// kept are the published diagnostics, and pulls the pulled diagnostics, of each file.
	kept, pulls map[uri.URI][]protocol.Diagnostic
	// released are the files whose buffer the engine released and has not opened again.
	released map[uri.URI]bool
	// waking has one channel per file that a caller waits on, which the next publish closes.
	waking map[uri.URI]chan struct{}
	mu     sync.Mutex
	// ondisk reports whether a publish describes the file on disk, as the check on disk of a
	// server that declares [Server.DiskCheck] publishes it, so a change of a buffer keeps it.
	ondisk bool
}

// newReports returns an empty store. ondisk reports whether the publishes describe the files
// on disk.
func newReports(ondisk bool) *reports {
	return &reports{
		ondisk:   ondisk,
		kept:     map[uri.URI][]protocol.Diagnostic{},
		pulls:    map[uri.URI][]protocol.Diagnostic{},
		released: map[uri.URI]bool{},
		waking:   map[uri.URI]chan struct{}{},
	}
}

// keep stores the published diagnostics of the file of, and wakes the callers that wait for
// them.
func (r *reports) keep(of uri.URI, diagnostics []protocol.Diagnostic) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.kept[of] = slices.Clone(diagnostics)
	if waking, waiting := r.waking[of]; waiting {
		close(waking)
		delete(r.waking, of)
	}
}

// pulled stores the pulled diagnostics of the file of.
func (r *reports) pulled(of uri.URI, diagnostics []protocol.Diagnostic) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pulls[of] = slices.Clone(diagnostics)
}

// published returns the published diagnostics of the file of, or nil for a file without a
// publish.
func (r *reports) published(of uri.URI) []protocol.Diagnostic {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.kept[of])
}

// said reports whether a report of the file of is kept: the server published its diagnostics,
// none included, and no later change of the buffer dropped them.
func (r *reports) said(of uri.URI) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, kept := r.kept[of]
	return kept
}

// forget drops the diagnostics of the buffer of the file of. The engine calls it when it
// replaces the buffer, because the diagnostics describe the replaced content. A publish that
// describes the file on disk is kept.
func (r *reports) forget(of uri.URI) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.drop(of)
}

// release drops the diagnostics of the buffer of the file of, as [reports.forget] does. The
// engine calls it when it releases the buffer, and calls [reports.reopening] before it opens a
// buffer of the file again.
func (r *reports) release(of uri.URI) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.drop(of)
	r.released[of] = true
}

// reopening drops the diagnostics of the file of before the engine opens a buffer of it, when
// the engine released its buffer before. A server can publish a report of the file after the
// release, such as the empty report that jdtls publishes for a closed buffer. That report
// describes no content that the engine sends. The report of a file whose buffer the engine
// never released is kept, because a server publishes it for the file on disk.
func (r *reports) reopening(of uri.URI) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.released[of] {
		r.drop(of)
		delete(r.released, of)
	}
}

// drop deletes the pulled diagnostics of the file of, and its published diagnostics unless a
// publish describes the file on disk. The caller has locked mu.
func (r *reports) drop(of uri.URI) {
	delete(r.pulls, of)
	if !r.ondisk {
		delete(r.kept, of)
	}
}

// errors returns the zero-based start lines of the kept diagnostics of error severity of both
// kinds, by the workspace path of their file, for the files in the directory within. at maps a
// URI to its workspace path. A server that reported nothing returns an empty map.
func (r *reports) errors(within source.Path, at func(uri.URI) source.Path) map[source.Path][]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[source.Path][]int{}
	for _, kind := range []map[uri.URI][]protocol.Diagnostic{r.kept, r.pulls} {
		for of, diagnostics := range kind {
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
	}
	return out
}

// wait returns the published diagnostics of the file of, and reports whether the server
// published a report of it. It waits for a report until by or until ctx ends, and returns at
// once when a report is kept or by has passed.
func (r *reports) wait(ctx context.Context, of uri.URI, by time.Time) ([]protocol.Diagnostic, bool) {
	defer engine.Waiting(ctx)()
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

// PublishDiagnostics returns nil without a record, because [ordered] keeps the diagnostics when
// the stream of the session reads the notification.
func (answers) PublishDiagnostics(context.Context, *protocol.PublishDiagnosticsParams) error {
	return nil
}
