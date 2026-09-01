// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package trust

import "go.dokimi.dev/techne/core/source"

// Provenance is what stands behind one answer.
//
// A service builds it. An engine declares its fidelity and returns
// caveats; it never assembles a Provenance, so it cannot overstate its
// own evidence.
//
// It carries no cost. What an answer cost is a selection concern and
// tells a caller nothing it can act on.
type Provenance struct {
	// Engine names the adapter that answered, so two answers at the same
	// tier can still be told apart.
	Engine       string
	Fidelity     Fidelity
	Completeness Completeness
	Caveats      []Caveat
}

// SupportsNegativeClaim reports whether an empty answer with this
// provenance means there are none.
func (p Provenance) SupportsNegativeClaim() bool {
	return SupportsNegativeClaim(p.Fidelity, p.Completeness)
}

// CaveatCode is a closed set, so a caller branches on one rather than
// matching prose.
type CaveatCode string

const (
	// CaveatStale means the files named drifted since they were indexed.
	CaveatStale CaveatCode = "stale"
	// CaveatTruncated means items were dropped to fit a budget.
	CaveatTruncated CaveatCode = "truncated"
	// CaveatBuildBroken means the workspace does not compile, so a
	// stronger engine could not run.
	CaveatBuildBroken CaveatCode = "build-broken"
	// CaveatIndexWarming means an engine is still building its index and
	// has not covered the scope yet.
	CaveatIndexWarming CaveatCode = "index-warming"
	// CaveatDynamic means reflection, string-keyed dispatch, struct tags
	// and runtime patching are invisible here, as they are to every
	// engine. Every resolved answer carries it.
	CaveatDynamic CaveatCode = "dynamic"
	// CaveatInactiveBuild means code behind inactive build flags was not
	// examined.
	CaveatInactiveBuild CaveatCode = "inactive-build-tags"
	// CaveatCrossLanguage means an edge leaving this language was not
	// looked for. No single-language engine sees one.
	CaveatCrossLanguage CaveatCode = "cross-language"
)

// Caveat is a limit on an answer that its fidelity does not express.
//
// A caveat that names files says which ones, because a caller told only
// that something drifted discards the whole answer, while one told which
// three files moved keeps the rest.
type Caveat struct {
	Code CaveatCode
	// Note is for a human reading the answer. A caller branches on Code.
	Note string
	// Paths are the files this caveat is about, empty when it is about
	// the answer as a whole.
	Paths []source.Path
}
