// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

module go.dokimi.dev/techne/lang

go 1.27.3

require (
	github.com/go-json-experiment/json v0.0.0-20260623181947-01eb4420fa68 // indirect
	github.com/mattn/go-pointer v0.0.1 // indirect
	github.com/tree-sitter/tree-sitter-c v0.24.2 // indirect
	github.com/tree-sitter/tree-sitter-go v0.25.0 // indirect
	github.com/tree-sitter/tree-sitter-javascript v0.25.0 // indirect
	github.com/tree-sitter/tree-sitter-python v0.25.0 // indirect
	github.com/tree-sitter/tree-sitter-rust v0.24.2 // indirect
)

require (
	github.com/bmatcuk/doublestar/v4 v4.10.3
	github.com/tree-sitter/go-tree-sitter v0.25.0
	go.dokimi.dev/assert v0.2.0
	go.dokimi.dev/techne/core v0.0.0
	go.lsp.dev/jsonrpc2 v1.0.1
	go.lsp.dev/protocol v1.0.1
	go.lsp.dev/uri v1.0.1
)

replace go.dokimi.dev/techne/core => ../techne-core
