// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// settling is how long a file waits for a server that reports unasked.
//
// A server with no pull request analyses a file after it is opened and
// publishes when it is done, so a file opened by this very call has
// nothing said about it yet. Waiting is the difference between reporting
// a file as clean and reporting what is wrong with it.
const settling = 2 * time.Second

// Verify reports what the server says is wrong with a scope.
//
// # Two ways a server reports, and both are read
//
// Newer servers answer textDocument/diagnostic when asked. Older ones
// publish unasked, when they finish analysing, and never answer a
// request. Reading only the first reports every server of the second
// kind as having found nothing — a clean bill of health from a server
// that was never asked, which is worse than no answer at all.
//
// Which one is used is read from what the server said at initialise
// rather than found out by trying: a server answering "no such method"
// is indistinguishable from one answering that there is nothing wrong.
//
// # Suites are ignored
//
// A server has one analysis and no notion of which linter or test runner
// to run. A caller naming suites is answered from the same analysis, and
// the caveat says so.
func (e *Engine) Verify(
	ctx context.Context,
	req engine.Request,
	suites []string,
) (engine.Result[edit.Finding], error) {
	held, err := e.running(ctx)
	if err != nil {
		return engine.Result[edit.Finding]{}, fmt.Errorf("%w: %w", engine.ErrDecline, err)
	}

	paths, err := e.files(req)
	if err != nil {
		return engine.Result[edit.Finding]{}, err
	}
	if len(paths) == 0 {
		return engine.Result[edit.Finding]{Skipped: true, Completeness: trust.ScopeTotal}, nil
	}

	pulls := held.capable.DiagnosticProvider != nil
	var out []edit.Finding
	var waited bool
	for _, p := range paths {
		if stopped := ctx.Err(); stopped != nil {
			return engine.Result[edit.Finding]{}, stopped
		}
		if !req.Tests && e.declared.IsTest(string(p)) {
			continue
		}
		if opened := e.open(ctx, held, p); opened != nil {
			return engine.Result[edit.Finding]{}, opened
		}

		var reported []protocol.Diagnostic
		if pulls {
			reported, err = e.pull(ctx, held, p)
			if err != nil {
				return engine.Result[edit.Finding]{}, err
			}
		} else {
			var settled bool
			reported, settled = e.pushed.wait(ctx, uri.File(e.fullPath(p)), settling)
			waited = waited || !settled
		}

		doc, err := e.read(p)
		if err != nil {
			return engine.Result[edit.Finding]{}, err
		}
		for _, one := range reported {
			out = append(out, edit.Finding{Diagnostic: found(one, doc)})
		}
	}

	return engine.Result[edit.Finding]{
		Items:        out,
		Completeness: trust.ScopeTotal,
		Caveats:      caveats(suites, waited),
	}, nil
}

// pull asks the server what is wrong with one file.
func (e *Engine) pull(
	ctx context.Context,
	held *session,
	p source.Path,
) ([]protocol.Diagnostic, error) {
	answered, err := held.asks.Diagnostic(ctx, &protocol.DocumentDiagnosticParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(e.fullPath(p))},
	})
	if err != nil {
		return nil, fmt.Errorf("lsp: %s: verify %s: %w", e.server.Name, p, err)
	}

	// The unchanged arm says the answer is the one already given, and is
	// only ever sent in reply to a request carrying the previous result
	// id. This sends none, so an unchanged report is a server being
	// generous rather than an answer, and is read as nothing to add.
	if full, complete := answered.(*protocol.RelatedFullDocumentDiagnosticReport); complete {
		return full.Items, nil
	}
	return nil, nil
}

// caveats states what this answer is not.
func caveats(suites []string, waited bool) []trust.Caveat {
	out := []trust.Caveat{dynamic}
	if len(suites) > 0 {
		out = append(out, trust.Caveat{
			Code: trust.CaveatUnsupported,
			Note: "a language server has one analysis and no suites to choose between, " +
				"so every suite named was answered from the same one",
		})
	}
	if waited {
		out = append(out, trust.Caveat{
			Code: trust.CaveatIndexWarming,
			Note: "this server reports when it finishes rather than when asked, " +
				"and a file it had not finished is reported as it stood",
		})
	}
	return out
}

// found turns one diagnostic into this vocabulary's own.
//
// The snippet is filled in here rather than left to whoever renders it,
// which has no filesystem: a message with no line to read it against
// costs a read per diagnostic to make sense of.
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

// messaged reads a message in whichever arm it arrived in.
//
// A newer server may send markup rather than a string. Reading only the
// string arm leaves the message empty, which is a diagnostic reported
// with nothing said about it.
func messaged(held protocol.InlayHintTooltip) string {
	switch message := held.(type) {
	case protocol.String:
		return string(message)
	case *protocol.MarkupContent:
		return message.Value
	}
	return ""
}

// grades maps the protocol's severities onto this vocabulary's.
//
// A server is not required to send one. Where none arrives the
// diagnostic keeps [diag.SeverityUnset], which says nobody graded it —
// a different fact from grading it least, and the one a caller filtering
// for errors needs in order not to hide it.
var grades = map[protocol.DiagnosticSeverity]diag.Severity{
	protocol.DiagnosticSeverityError:       diag.SeverityError,
	protocol.DiagnosticSeverityWarning:     diag.SeverityWarning,
	protocol.DiagnosticSeverityInformation: diag.SeverityInfo,
	protocol.DiagnosticSeverityHint:        diag.SeverityHint,
}

func graded(held protocol.DiagnosticSeverity) diag.Severity { return grades[held] }

// coded is the rule a diagnostic names, which the protocol writes as
// either a string or a number.
func coded(held protocol.ProgressToken) string {
	switch code := held.(type) {
	case protocol.String:
		return string(code)
	case protocol.Integer:
		return fmt.Sprint(int32(code))
	}
	return ""
}

// published is where the diagnostics a server sends unasked are kept.
//
// A server publishes when it has finished analysing rather than when
// anyone asks, so what arrives is stored until something wants it. It is
// keyed by file and holds only the latest publish for each: a publish is
// the whole of what the server currently says about that file.
type published struct {
	mu     sync.Mutex
	held   map[uri.URI][]protocol.Diagnostic
	waking map[uri.URI]chan struct{}
}

func newPublished() *published {
	return &published{
		held:   map[uri.URI][]protocol.Diagnostic{},
		waking: map[uri.URI]chan struct{}{},
	}
}

// keep records one publish and wakes whoever was waiting for it.
func (p *published) keep(of uri.URI, held []protocol.Diagnostic) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	p.held[of] = slices.Clone(held)
	if waking, waiting := p.waking[of]; waiting {
		close(waking)
		delete(p.waking, of)
	}
}

// wait returns what a server said about a file, waiting a bounded time
// for it to say anything at all.
//
// The second result reports whether the wait ended with an answer. A
// file the server had not finished is reported as it stood, and the
// caveat on the result says the answer may be short — which is the
// difference between "nothing is wrong" and "nothing was said yet".
func (p *published) wait(
	ctx context.Context,
	of uri.URI,
	within time.Duration,
) ([]protocol.Diagnostic, bool) {
	if p == nil {
		return nil, false
	}

	p.mu.Lock()
	if held, said := p.held[of]; said {
		p.mu.Unlock()
		return held, true
	}
	waking, waiting := p.waking[of]
	if !waiting {
		waking = make(chan struct{})
		p.waking[of] = waking
	}
	p.mu.Unlock()

	timer := time.NewTimer(within)
	defer timer.Stop()

	select {
	case <-waking:
	case <-timer.C:
		return nil, false
	case <-ctx.Done():
		return nil, false
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	return p.held[of], true
}
