// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package source

// Language identifies a programming language on requests and answers. The
// zero value is the empty string, which is not a language.
//
// Each language module declares its own value, and core declares none. The
// language registry refuses a value that two modules declare. The value is
// part of every [go.dokimi.dev/techne/core/sema.ID], so changing it
// invalidates the IDs an index has stored.
type Language string
