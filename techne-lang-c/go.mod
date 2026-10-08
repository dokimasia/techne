module go.dokimi.dev/techne/lang/c

go 1.27.0

require (
	github.com/tree-sitter/go-tree-sitter v0.25.0
	github.com/tree-sitter/tree-sitter-c v0.24.2
	go.dokimi.dev/assert v0.0.0-20261007133442-6f235714117b
	go.dokimi.dev/techne/core v0.0.0
	go.dokimi.dev/techne/lang v0.0.0
)

require (
	github.com/bmatcuk/doublestar/v4 v4.10.2 // indirect
	github.com/go-json-experiment/json v0.0.0-20260623181947-01eb4420fa68 // indirect
	github.com/mattn/go-pointer v0.0.1 // indirect
	go.lsp.dev/jsonrpc2 v1.0.1 // indirect
	go.lsp.dev/protocol v1.0.1 // indirect
	go.lsp.dev/uri v1.0.1 // indirect
)

replace go.dokimi.dev/techne/core => ../techne-core

replace go.dokimi.dev/techne/lang => ../techne-lang
