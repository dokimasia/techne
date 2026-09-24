// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package tool

import (
	"fmt"
	"strings"

	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/source"
	"go.dokimi.dev/techne/core/trust"
)

// The types of the input fields that take a word of a closed vocabulary. The schema of each
// field lists the words as an enum, which [New] derives from the vocabulary of core, so every
// schema lists a word added there. A tool refuses any other word, and the reason lists the
// words that the field takes.
type (
	// KindWord is a word of [sema.Kinds]. The empty word selects every kind.
	KindWord string
	// RelationWord is a word of [sema.RelationKinds].
	RelationWord string
	// FidelityWord is a word of [trust.Fidelities]. The empty word selects [trust.None].
	FidelityWord string
)

// Include adds bindings to an answer beside the declarations that a file offers to the rest of
// a program.
type Include string

const (
	// IncludeImport adds what a file brings into scope.
	IncludeImport Include = "import"
	// IncludeParameter adds the parameters and the type parameters of each signature.
	IncludeParameter Include = "parameter"
	// IncludeLocal adds the declarations inside the body of a callable or the value of a
	// binding.
	IncludeLocal Include = "local"
	// IncludeAll adds every binding, the labels of statements included.
	IncludeAll Include = "all"
)

// Includes returns every Include.
func Includes() []Include {
	return []Include{IncludeImport, IncludeParameter, IncludeLocal, IncludeAll}
}

// String returns the word of i.
func (i Include) String() string { return string(i) }

// binds are the bindings that each Include adds.
var binds = map[Include]engine.Bindings{
	IncludeImport:    engine.BindImports,
	IncludeParameter: engine.BindParameters,
	IncludeLocal:     engine.BindLocals,
	IncludeAll:       engine.BindAll,
}

// kindOf returns the kind of w, [sema.KindUnknown] for the empty word, and a refusal for a
// word that no kind has.
func kindOf(w KindWord) (sema.Kind, *Failure) {
	if w == "" {
		return sema.KindUnknown, nil
	}
	return worded("kind", string(w), sema.Kinds())
}

// relationOf returns the relation kind of w, and a refusal for a word that no relation kind
// has.
func relationOf(w RelationWord) (sema.RelationKind, *Failure) {
	return worded("relation", string(w), sema.RelationKinds())
}

// fidelityOf returns the fidelity of w, [trust.None] for the empty word, and a refusal for a
// word that no fidelity has.
func fidelityOf(w FidelityWord) (trust.Fidelity, *Failure) {
	if w == "" {
		return trust.None, nil
	}
	return worded("preferred_fidelity", string(w), trust.Fidelities())
}

// levelOf returns the level of w, [DefaultDetail] of scope for [DetailUnset], and a refusal
// for a word that no level has.
func levelOf(w Detail, scope source.Path) (Detail, *Failure) {
	if w == DetailUnset {
		return DefaultDetail(scope), nil
	}
	return worded("detail", string(w), Levels())
}

// bindingsOf returns the bindings that words add, and a refusal for a word that no Include
// has.
func bindingsOf(words []Include) (engine.Bindings, *Failure) {
	var out engine.Bindings
	for _, w := range words {
		one, failure := worded("include", string(w), Includes())
		if failure != nil {
			return 0, failure
		}
		out |= binds[one]
	}
	return out, nil
}

// worded returns the value of values whose word is given, and a refusal that lists the words of
// values when no value has it. field is the name of the input field.
func worded[T fmt.Stringer](field, given string, values []T) (T, *Failure) {
	words := make([]string, 0, len(values))
	for _, v := range values {
		if v.String() == given {
			return v, nil
		}
		words = append(words, v.String())
	}
	var zero T
	return zero, &Failure{
		Code:   trust.Refused.String(),
		Reason: fmt.Sprintf("%s does not take %q. It takes %s", field, given, strings.Join(words, ", ")),
	}
}

// first returns the first failure that is not nil, or nil.
func first(failures ...*Failure) *Failure {
	for _, one := range failures {
		if one != nil {
			return one
		}
	}
	return nil
}
