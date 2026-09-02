// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package tool keeps an answer small enough to be worth reading.
//
// The consumer is a language model with a fixed context window, so a
// correct answer that costs more than reading the file has removed the
// reason to call the tool.
//
// # Thin before dropping
//
// [Fit] takes documentation off every item before it removes any item.
// Fifty names with no documentation answers what is there; eight
// complete entries that do not say forty-two more exist answer a
// different question, wrongly.
//
// # Say what was cut
//
// Every truncated answer carries [trust.CaveatTruncated] naming how many
// matched against how many came back. An answer that found something
// returns at least one item: zero items beside a count reads like an
// answer nothing served.
//
// # Detail chooses before the budget does
//
// [Detail] selects what each item carries, and [Fit] then measures. The
// zero Detail is [Standard], which answers most questions without paying
// for prose.
//
// # The count is an estimate
//
// Tokens are estimated from the serialised answer rather than tokenised:
// the server does not know the client's tokeniser, and the estimate only
// has to be good enough to decide what to drop. It is deliberately
// conservative, so an answer can come back smaller than it needed to be.
//
// # Dependency position
//
// Imports core/engine, core/sema and core/trust. Thinning an answer
// never changes the evidence behind it.
package tool
