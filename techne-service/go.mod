module go.dokimi.dev/techne/service

go 1.27.0

require (
	go.dokimi.dev/assert v0.0.0-20260901105745-9c4b8bd0fc5f
	go.dokimi.dev/techne/core v0.0.0
	golang.org/x/sys v0.45.0
)

require github.com/google/go-cmp v0.7.0 // indirect

replace go.dokimi.dev/techne/core => ../techne-core
