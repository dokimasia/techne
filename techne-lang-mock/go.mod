// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

module go.dokimi.dev/techne/lang/mock

go 1.27.2

require (
	go.dokimi.dev/assert v0.0.0-20261007133442-6f235714117b
	go.dokimi.dev/techne/core v0.0.0
	go.dokimi.dev/techne/lang v0.0.1
)

replace (
	go.dokimi.dev/techne/core => ../techne-core
	go.dokimi.dev/techne/lang => ../techne-lang
)
