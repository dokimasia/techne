// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package trust

import "go.dokimi.dev/techne/core/source"

// Provenance is the evidence behind one answer. Services build it from the
// engine that answered. Engines never build one, so an engine cannot
// overstate its own evidence.
type Provenance struct {
	// Engine is the name of the engine that answered. A merged answer lists
	// every contributing engine.
	Engine       string
	Fidelity     Fidelity
	Completeness Completeness
	Caveats      []Caveat
}

// SupportsNegativeClaim reports whether an empty answer with this provenance
// proves absence.
func (p Provenance) SupportsNegativeClaim() bool {
	return SupportsNegativeClaim(p.Fidelity, p.Completeness)
}

// CaveatCode identifies a caveat. Callers branch on the code, never on the
// note.
type CaveatCode string

const (
	// CaveatStale means the files in Caveat.Paths changed after they were
	// indexed.
	CaveatStale CaveatCode = "stale"
	// CaveatTruncated means items were dropped to fit a budget.
	CaveatTruncated CaveatCode = "truncated"
	// CaveatBuildBroken means the project does not compile, so the answer is
	// below the engine's declared tier.
	CaveatBuildBroken CaveatCode = "build-broken"
	// CaveatIndexWarming means the engine had not finished loading the
	// workspace.
	CaveatIndexWarming CaveatCode = "index-warming"
	// CaveatDynamic means reflection, string-keyed dispatch, struct tags,
	// and runtime patching are invisible to the engine. Every resolved answer
	// includes it.
	CaveatDynamic CaveatCode = "dynamic"
	// CaveatInactiveBuild means code behind inactive build flags was not
	// examined.
	CaveatInactiveBuild CaveatCode = "inactive-build-tags"
	// CaveatCrossLanguage means edges into other languages were not
	// examined.
	CaveatCrossLanguage CaveatCode = "cross-language"
	// CaveatUnrouted means no language claims the scope.
	CaveatUnrouted CaveatCode = "unrouted"
	// CaveatUnsupported means no engine serves this language and role for at
	// least part of the scope.
	CaveatUnsupported CaveatCode = "unsupported"
	// CaveatUnread means the files in Caveat.Paths exceed the size an engine
	// reads.
	CaveatUnread CaveatCode = "unread"
	// CaveatDependents means a gate checked the changed files but not the
	// files that depend on them.
	CaveatDependents CaveatCode = "dependents"
	// CaveatUnrewritten means a plan leaves a use of its subject as it was,
	// although the engine reports the use. Asking again returns the same plan.
	CaveatUnrewritten CaveatCode = "unrewritten"
	// CaveatPartialCheck means a gate checks less than the compiler of the
	// language checks, so a change that passes it can still fail to build.
	CaveatPartialCheck CaveatCode = "partial-check"
)

// Caveat is a limit on an answer that its fidelity does not express.
type Caveat struct {
	Code CaveatCode
	// Note is a human-readable explanation.
	Note string
	// Paths are the files the caveat applies to, or empty if it applies to
	// the whole answer.
	Paths []source.Path
}
