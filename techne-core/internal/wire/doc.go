// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package wire maps enum values to the strings that represent them on the
// wire.
//
// Each enum in core declares one [Names]. A value outside the set encodes as
// the fallback's string. An unknown string decodes to the fallback value.
//
// # Dependency position
//
// Imports encoding/json and fmt from the standard library. Only the packages
// of core can import it.
package wire
