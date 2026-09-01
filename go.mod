module go.dokimi.dev/techne

go 1.27.0

require (
	go.dokimi.dev/techne/core v0.0.0
	go.dokimi.dev/techne/lang/go v0.0.0
	go.dokimi.dev/techne/lang/java v0.0.0
	go.dokimi.dev/techne/lang/python v0.0.0
	go.dokimi.dev/techne/lang/rust v0.0.0
	go.dokimi.dev/techne/lang/typescript v0.0.0
	go.dokimi.dev/techne/presenter v0.0.0
)

require go.dokimi.dev/techne/lang v0.0.0

replace go.dokimi.dev/techne/core => ./techne-core

replace go.dokimi.dev/techne/lang => ./techne-lang

replace go.dokimi.dev/techne/lang/go => ./techne-lang-go

replace go.dokimi.dev/techne/lang/java => ./techne-lang-java

replace go.dokimi.dev/techne/lang/python => ./techne-lang-python

replace go.dokimi.dev/techne/lang/rust => ./techne-lang-rust

replace go.dokimi.dev/techne/lang/typescript => ./techne-lang-typescript

replace go.dokimi.dev/techne/presenter => ./techne-presenter
