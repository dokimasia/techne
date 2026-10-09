# Copyright Dokimasia B.V. 2026
# SPDX-License-Identifier: MIT

# The targets of techne, which ergon init appends to the Makefile.

##@ Repository

.PHONY: corpus

corpus: ## Drive the binary over the repositories of test/corpus/corpus.json, for hours on a first run (ONLY=names or languages)
	TECHNE_CORPUS_ONLY=$(ONLY) $(GO) test -tags=corpus -count=1 -timeout=24h -v -run TestCorpus ./test/corpus
