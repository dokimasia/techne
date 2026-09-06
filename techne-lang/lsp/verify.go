// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"go.dokimi.dev/techne/core/diag"
	"go.dokimi.dev/techne/core/edit"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

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
// reporting is how long one file waits for a server that publishes
// diagnostics rather than answering for them.
//
// A different wait from the one a workspace gets: that one is asked once
// and covers reading the whole project, this one is asked per file and
// covers analysing that file after it was opened. Charging a scope of
// fifty files the workspace figure fifty times over is what conflating
// them costs.
const reporting = 2 * time.Second

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

	var out []edit.Finding
	var waited bool
	var large []source.Path
	for _, p := range paths {
		if stopped := ctx.Err(); stopped != nil {
			return engine.Result[edit.Finding]{}, stopped
		}
		if !req.Tests && e.declared.IsTest(string(p)) {
			continue
		}
		if opened := e.open(ctx, held, p); opened != nil {
			// A file past the size an engine reads is one nothing
			// looked at, which lowers what this answer covers rather
			// than ending it. Coverage is the whole point here: a gate
			// saying a scope is clean must not make that claim about a
			// file it never opened.
			if _, big := errors.AsType[lang.LargeError](opened); big {
				large = append(large, p)
				continue
			}
			return engine.Result[edit.Finding]{}, opened
		}

		reported, settled, err := e.diagnostics(ctx, held, p)
		if err != nil {
			return engine.Result[edit.Finding]{}, err
		}
		waited = waited || !settled

		doc, err := e.read(p)
		if err != nil {
			return engine.Result[edit.Finding]{}, err
		}
		for _, one := range reported {
			out = append(out, e.finding(ctx, held, one, doc, len(out)))
		}
	}

	// A gate that says a file is clean is the claim a caller acts on by
	// shipping it. It may only be made about a file the server actually
	// analysed: one that never reported is a file nothing looked at, and
	// nothing looked at is not nothing wrong.
	covered, caveats := e.settled(ctx)
	if waited || len(large) > 0 {
		covered = trust.ScopePartial
	}
	caveats = append(caveats, reasons(suites, waited)...)
	return engine.Result[edit.Finding]{
		Items:        out,
		Completeness: covered,
		Caveats:      append(caveats, unread(large)...),
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

// reasons states what this answer is not, beyond what every answer
// already says.
func reasons(suites []string, waited bool) []trust.Caveat {
	var out []trust.Caveat
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
			Note: "this server had not reported on every file in the scope, so a file " +
				"with nothing against it may be one nothing has looked at yet",
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

// analysed reports whether a server has produced a view of one file.
//
// It is the evidence behind every empty semantic answer. A server that
// has not analysed a file resolves a definition inside it from a
// syntactic index and answers every other question with nothing — metals
// before it has imported a build does exactly that, reporting no
// implementation of a trait a class two lines below extends. Nothing in
// the answer itself tells the two apart.
//
// Diagnostics are what tells them apart, because producing them is what
// having a compiler view means. A server that answers a diagnostic
// request has one; a server that publishes unasked has one once it has
// published; a server that has done neither has not looked.
// A server that refuses the request has no view to report, which is the
// answer rather than a fault: this is asked to find out what an empty
// answer is worth, and a refusal settles that as surely as a reply does.
func (e *Engine) analysed(ctx context.Context, held *session, p source.Path) bool {
	if held.capable.DiagnosticProvider == nil {
		// Waited for rather than looked at. A server publishes when it
		// finishes, and a file it found nothing wrong with is published
		// as an empty list a moment later — looked at instantly, every
		// clean file reads as one nothing analysed, and every correct
		// empty answer is reported short.
		_, said := e.pushed.wait(ctx, uri.File(e.fullPath(p)), reporting)
		return said
	}
	answered, err := held.asks.Diagnostic(ctx, &protocol.DocumentDiagnosticParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(e.fullPath(p))},
	})
	if err != nil {
		return false
	}
	_, full := answered.(*protocol.RelatedFullDocumentDiagnosticReport)
	return full
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

// broken reports whether the server has said the workspace does not
// compile.
//
// Read from what arrived unasked rather than by asking, because asking
// per file per read would double every round trip to answer a question
// nobody put. A server publishes when it finishes analysing, so what is
// held here is what it has told this session so far — one fault
// anywhere in it is enough.
//
// It says nothing about a server that has published nothing, which is
// both a clean workspace and one nobody has looked at. The difference is
// what completeness already carries.
func (p *published) broken() bool {
	if p == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, held := range p.held {
		for _, one := range held {
			if one.Severity == protocol.DiagnosticSeverityError {
				return true
			}
		}
	}
	return false
}

// forget drops what a server said about a file.
//
// Called when the buffer is replaced, because what the server said was
// about the text it no longer holds. Left in place, the next question
// reads the old report as the new one and a gate judges a change by
// what was wrong before it.
func (p *published) forget(of uri.URI) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.held, of)
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
