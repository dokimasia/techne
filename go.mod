// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

module go.dokimi.dev/techne

go 1.27.2

require (
	github.com/modelcontextprotocol/go-sdk v1.8.0
	go.dokimi.dev/assert v0.0.0-20261007133442-6f235714117b
	go.dokimi.dev/techne/core v0.0.0
	go.dokimi.dev/techne/lang v0.0.0
	go.dokimi.dev/techne/lang/c v0.0.0
	go.dokimi.dev/techne/lang/csharp v0.0.0
	go.dokimi.dev/techne/lang/go v0.0.0
	go.dokimi.dev/techne/lang/java v0.0.0
	go.dokimi.dev/techne/lang/javascript v0.0.0
	go.dokimi.dev/techne/lang/mock v0.0.0
	go.dokimi.dev/techne/lang/python v0.0.0
	go.dokimi.dev/techne/lang/ruby v0.0.0
	go.dokimi.dev/techne/lang/rust v0.0.0
	go.dokimi.dev/techne/lang/scala v0.0.0
	go.dokimi.dev/techne/lang/typescript v0.0.0
	go.dokimi.dev/techne/presenter v0.0.0
	go.dokimi.dev/techne/service v0.0.0
	go.dokimi.dev/techne/tool v0.0.0
)

require (
	github.com/bmatcuk/doublestar/v4 v4.10.2 // indirect
	github.com/go-json-experiment/json v0.0.0-20260623181947-01eb4420fa68 // indirect
	github.com/google/jsonschema-go v0.4.3 // indirect
	github.com/mattn/go-pointer v0.0.1 // indirect
	github.com/segmentio/asm v1.1.3 // indirect
	github.com/segmentio/encoding v0.5.4 // indirect
	github.com/tree-sitter/go-tree-sitter v0.25.0 // indirect
	github.com/tree-sitter/tree-sitter-c v0.24.2 // indirect
	github.com/tree-sitter/tree-sitter-c-sharp v0.23.5 // indirect
	github.com/tree-sitter/tree-sitter-go v0.25.0 // indirect
	github.com/tree-sitter/tree-sitter-java v0.23.5 // indirect
	github.com/tree-sitter/tree-sitter-javascript v0.25.0 // indirect
	github.com/tree-sitter/tree-sitter-python v0.25.0 // indirect
	github.com/tree-sitter/tree-sitter-ruby v0.23.1 // indirect
	github.com/tree-sitter/tree-sitter-rust v0.24.2 // indirect
	github.com/tree-sitter/tree-sitter-scala v0.26.2 // indirect
	github.com/tree-sitter/tree-sitter-typescript v0.23.2 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	go.lsp.dev/jsonrpc2 v1.0.1 // indirect
	go.lsp.dev/protocol v1.0.1 // indirect
	go.lsp.dev/uri v1.0.1 // indirect
	golang.org/x/mod v0.41.0 // indirect
	golang.org/x/oauth2 v0.35.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	golang.org/x/tools v0.51.0 // indirect
)

// The fork of the Go grammar parses a method with type parameters.
replace github.com/tree-sitter/tree-sitter-go => github.com/dokimasia/tree-sitter-go v0.0.0-20261008091630-10115b8fc25d

replace (
	go.dokimi.dev/techne/core => ./techne-core
	go.dokimi.dev/techne/lang => ./techne-lang
	go.dokimi.dev/techne/lang/c => ./techne-lang-c
	go.dokimi.dev/techne/lang/csharp => ./techne-lang-csharp
	go.dokimi.dev/techne/lang/go => ./techne-lang-go
	go.dokimi.dev/techne/lang/java => ./techne-lang-java
	go.dokimi.dev/techne/lang/javascript => ./techne-lang-javascript
	go.dokimi.dev/techne/lang/mock => ./techne-lang-mock
	go.dokimi.dev/techne/lang/python => ./techne-lang-python
	go.dokimi.dev/techne/lang/ruby => ./techne-lang-ruby
	go.dokimi.dev/techne/lang/rust => ./techne-lang-rust
	go.dokimi.dev/techne/lang/scala => ./techne-lang-scala
	go.dokimi.dev/techne/lang/typescript => ./techne-lang-typescript
	go.dokimi.dev/techne/presenter => ./techne-presenter
	go.dokimi.dev/techne/service => ./techne-service
	go.dokimi.dev/techne/tool => ./techne-tool
)
