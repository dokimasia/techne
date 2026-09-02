module go.dokimi.dev/techne/lang/java

go 1.27.0

require (
	github.com/tree-sitter/go-tree-sitter v0.25.0
	github.com/tree-sitter/tree-sitter-java v0.23.5
	go.dokimi.dev/assert v0.0.0-20260901105745-9c4b8bd0fc5f
	go.dokimi.dev/techne/core v0.0.0
	go.dokimi.dev/techne/lang v0.0.0
)

require (
	github.com/go-json-experiment/json v0.0.0-20260623181947-01eb4420fa68 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/mattn/go-pointer v0.0.1 // indirect
	go.lsp.dev/jsonrpc2 v1.0.1 // indirect
	go.lsp.dev/protocol v1.0.1 // indirect
	go.lsp.dev/uri v1.0.1 // indirect
)

replace go.dokimi.dev/techne/core => ../techne-core

replace go.dokimi.dev/techne/lang => ../techne-lang
