// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/techne/core/engine"
	"go.dokimi.dev/techne/core/sema"
	"go.dokimi.dev/techne/core/trust"
	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/lang/lsp"
)

// The tests speak to a real process over a real pipe, because that is
// where the interesting failures are: framing, a notification arriving
// between a request and its answer, a server asking the client something
// and waiting for a reply.
//
// The process is this test binary, re-run with an environment variable
// set. Nothing has to be built, installed or found on a path, so the
// suite says the same thing on a machine with no language server on it.
const (
	acting  = "TECHNE_LSP_FAKE"
	pretend = "TECHNE_LSP_FAKE_MODE"
	// noting names a file the fake appends a line to as it starts, so a
	// case can count how many servers a run of calls actually started.
	noting = "TECHNE_LSP_FAKE_STARTS"
	// elsewhere names a file outside the workspace, which a server whose
	// own view is wider than techne's root answers about routinely.
	elsewhere = "TECHNE_LSP_FAKE_OUTSIDE"
)

// TestMain turns the test binary into a language server when it is asked
// to be one, and runs the tests otherwise.
func TestMain(m *testing.M) {
	if os.Getenv(acting) == "" {
		os.Exit(m.Run())
	}
	os.Exit(serve(os.Getenv(pretend)))
}

// The shapes the fake takes. Each is a real server's behaviour, and a
// name rather than a literal because the fake and the cases have to
// agree on it and a typo in either would quietly test nothing.
const (
	// modeDefault the shape most servers answer in.
	modeDefault = ""
	// modeSilent starts and never answers.
	modeSilent = "silent"
	// modeDies exits during the handshake.
	modeDies = "dies"
	// modeEmpty reports a file that declares nothing.
	modeEmpty = "empty"
	// modeUnicode answers about a line holding a character outside ASCII.
	modeUnicode = "unicode"
	// modeFlat answers document symbols without building the tree.
	modeFlat = "flat"
	// modeOneLocation answers a definition as a single location rather than a list.
	modeOneLocation = "one"
	// modeLinks answers a definition as a list of links.
	modeLinks = "links"
	// modeUnresolved answers that a name denotes nothing.
	modeUnresolved = "nowhere"
	// modeUnranged answers a workspace query naming a file and no range in it.
	modeUnranged = "resolving"
	// modeUnnameable says the position cannot be renamed.
	modeUnnameable = "unnameable"
	// modeOrdered answers a rename as an ordered list that may move files.
	modeOrdered = "ordered"
	// modeOverlapping answers a rename with edits that write over each other.
	modeOverlapping = "overlapping"
	// modeStrict renames only what is at the column the protocol counts to.
	modeStrict = "strict"
	// modePushes reports diagnostics when it finishes rather than when asked.
	modePushes = "pushes"
	// modeAsks interrogates the client during the handshake.
	modeAsks = "asks"
	// modeElsewhere answers about a file outside the workspace.
	modeElsewhere = "elsewhere"
	// modeUncallable refuses the call hierarchy, as it does for anything not callable.
	modeUncallable = "uncallable"
	// modeLoading reads the workspace before it can answer, and says so
	// the way a server does: a job that begins and later ends.
	modeLoading = "loading"
	// modeStuck begins that job and never finishes it.
	modeStuck = "stuck"
	// modeUngated has finished loading and answers no diagnostic
	// request and publishes nothing, which is a server with no compiler
	// view of the file: tsserver over one outside its project, metals
	// before it has imported a build.
	modeUngated = "ungated"
	// modeThin answers document symbols and renames, and says at
	// initialise that it answers nothing else — which is most servers
	// for most of the protocol.
	modeThin = "thin"
	// modeEchoes reports the first line of the buffer it is holding as
	// the declaration a file makes, which is the only way to see from
	// outside what a server thinks a file says.
	modeEchoes = "echoes"
	// modeMoveless declares everything except an interest in files
	// moving, which is most servers.
	modeMoveless = "moveless"
	// modeSilentMove is asked about a file move and answers that it
	// implies no edits, while publishing nothing and answering no
	// diagnostic request. metals does exactly that.
	modeSilentMove = "silentmove"
	// modeExtracts offers extracting a variable beside extracting a
	// function, both under one kind, and hands back the edit when the
	// action is resolved. gopls, rust-analyzer and jdtls all work this
	// way.
	modeExtracts = "extracts"
	// modeCommands offers the same extraction as a command instead, and
	// answers it by asking the client to apply the result.
	// typescript-language-server exposes every refactoring this way.
	modeCommands = "commands"
	// modeWatches keeps its own model of the workspace and refuses to
	// refactor while it differs from the filesystem, until it is told
	// what changed. jdtls answers "out of sync with file system".
	modeWatches = "watches"
	// modeOpened rewrites only the buffers the client is holding, and
	// answers perfectly well who uses a declaration while doing so.
	// metals renames a class in the one file it was given and leaves
	// every other use of it where it was.
	modeOpened = "opened"
	// modeShort names a use and then does not rewrite it, whatever the
	// client is holding.
	modeShort = "short"
	// modeConflicts works the rename out and will not do it, which is
	// what a server answers when the new name is already taken.
	modeConflicts = "conflicts"
	// modeUnenclosed answers a reference on the file's first line,
	// which is outside every declaration an outline names — an import
	// sits exactly there.
	modeUnenclosed = "unenclosed"
	// modeCompiles reports what is wrong with the buffer it is holding
	// rather than a fixed list, and offers one quick fix for it, which
	// is what a gate over content nobody has written needs.
	modeCompiles = "compiles"
)

// buffered reports whether a mode only rewrites what it was given.
func buffered(mode string) bool { return mode == modeOpened || mode == modeShort }

// placeholder is what the fake calls the function it extracts, as every
// real server calls it something of its own: newFunction, fun_name,
// getWeighted.
const placeholder = "newFunction"

// extracts reports whether a mode offers a refactoring at all.
func extracts(mode string) bool { return mode == modeExtracts || mode == modeCommands }

// loading is how long the loading mode takes to read its workspace.
//
// Longer than starting a process and asking it two questions, so a case
// that fails to wait meets the empty answer rather than racing past it.
const loading = 2 * time.Second

// asked is one request off the wire, in the shape the fake reads it.
type asked struct {
	ID     *int64          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// serve answers the protocol the way the mode asks it to.
//
// One switch over the method, and each answer chosen by mode. A mode
// name is a shape a real server takes: the single location gopls sends
// for a definition, the link list rust-analyzer sends, the map-shaped
// workspace edit an older server sends for a rename.
func serve(mode string) int {
	if at := os.Getenv(noting); at != "" {
		held, err := os.OpenFile(at, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return 4
		}
		fmt.Fprintln(held, "started")
		_ = held.Close()
	}

	in, out := bufio.NewReader(os.Stdin), os.Stdout
	// Writing happens from the loop and, in the loading modes, from a
	// timer as well.
	var sending sync.Mutex
	say := func(body string) {
		sending.Lock()
		defer sending.Unlock()
		write(out, body)
	}
	// done reports whether the loading modes have finished loading.
	done := false
	// opened is what the client said it can do, kept from initialise.
	var opened json.RawMessage
	// answered is what the client said when this server asked it
	// something, so a case can see a callback that a passing outline
	// would otherwise hide.
	answered := map[string]string{}
	// The workspace is a temporary directory the fake never learns the
	// name of, so every answer naming a file echoes back the one the
	// request named.
	var seen string
	// holding is the buffer the client has given this server, per file,
	// which is what a real server answers from and what goes stale when
	// something rewrites the file without saying so.
	holding := map[string]string{}
	// synced is whether this server's own model of the workspace agrees
	// with the filesystem. A change to a buffer does not settle it: the
	// file on disk is a different thing from the buffer, and only being
	// told about the file settles it.
	synced := true

	for {
		raw, err := frame(in)
		if err != nil {
			return 0
		}
		var held asked
		if err := json.Unmarshal(raw, &held); err != nil {
			return 1
		}
		if named := document(held.Params); named != "" {
			seen = named
		}

		switch held.Method {
		case "initialize":
			// A real server knows the workspace root from here, and
			// answers workspace-wide questions before any document is
			// opened. Without this the fake would have no file to name in
			// a workspace symbol answer.
			seen = rooted(held.Params) + "/a.fake"
			opened = held.Params
			switch mode {
			case modeSilent:
				// A server that starts and never answers. The client
				// must give up on its context rather than wait for one.
				select {}
			case modeDies:
				return 3
			}
			// A notification and a request of its own before the answer,
			// which is what a real server does and what a client that
			// matched by arrival order would get wrong.
			write(out, `{"jsonrpc":"2.0","method":"window/logMessage",`+
				`"params":{"type":3,"message":"starting"}}`)
			write(out, `{"jsonrpc":"2.0","id":9001,"method":"client/registerCapability",`+
				`"params":{"registrations":[]}}`)
			if mode == modeAsks {
				// A real server asks these during startup and blocks on
				// the reply. What comes back is kept so the next request
				// can report it, because a fake cannot assert.
				answered["configuration"] = request(in, out, 9002,
					`"workspace/configuration",`+
						`"params":{"items":[{"section":"fake"},{"section":"absent"}]}`)
				answered["folders"] = request(in, out, 9003,
					`"workspace/workspaceFolders","params":null`)
				answered["edit"] = request(in, out, 9004,
					`"workspace/applyEdit","params":{"edit":{"changes":{}}}`)
			}
			if mode == modeLoading || mode == modeStuck {
				// A server that reads the workspace announces it right
				// after the handshake, which is the moment a client that
				// asked too early would be answered with nothing.
				say(`{"jsonrpc":"2.0","id":9100,` +
					`"method":"window/workDoneProgress/create",` +
					`"params":{"token":"loading"}}`)
				say(`{"jsonrpc":"2.0","method":"$/progress","params":` +
					`{"token":"loading","value":{"kind":"begin","title":"Loading"}}}`)
				if mode == modeLoading {
					go func() {
						time.Sleep(loading)
						say(`{"jsonrpc":"2.0","method":"$/progress","params":` +
							`{"token":"loading","value":{"kind":"end"}}}`)
						sending.Lock()
						done = true
						sending.Unlock()
					}()
				}
			}
			answer(out, held.ID, capabilities(mode, held.Params))

		case "textDocument/didOpen":
			holding[seen] = opening(held.Params)
			if mode == modePushes && !receives(opened) {
				// A server checks whether the client can receive
				// diagnostics before it sends any. techne once did not
				// say so, and every push-model server was silent.
				continue
			}
			if mode == modePushes {
				// A server with no pull request reports when it has
				// finished rather than when it is asked.
				write(out, fmt.Sprintf(
					`{"jsonrpc":"2.0","method":"textDocument/publishDiagnostics",`+
						`"params":{"uri":%q,"diagnostics":%s}}`, seen, problems()))
			}

		case "textDocument/didClose":
			delete(holding, seen)

		case "textDocument/didChange":
			holding[seen] = changed(held.Params)
			if mode == modeWatches {
				synced = false
			}

		case "textDocument/codeAction":
			if mode == modeCompiles {
				answer(out, held.ID, mends(held.Params, seen))
				continue
			}
			answer(out, held.ID, actions(mode))
		case "codeAction/resolve":
			answer(out, held.ID, resolved(held.Params, seen, holding[seen]))
		case "workspace/executeCommand":
			// The refactoring is performed rather than described: the
			// server works it out, offers the client the result and
			// answers the command.
			request(in, out, 9200, fmt.Sprintf(
				`"workspace/applyEdit","params":{"edit":%s}`,
				lifted(seen, holding[seen], "function")))
			answer(out, held.ID, `null`)

		case "textDocument/documentSymbol":
			if extracts(mode) {
				// What the buffer this server is holding declares, so a
				// client can see the declaration an extraction added.
				answer(out, held.ID, functions(holding[seen]))
				continue
			}
			if mode == modeEchoes {
				// The buffer this server is holding, as a declaration
				// name. A client that opened the file and never said it
				// changed is answered with what it first sent.
				answer(out, held.ID, oneName(first(holding[seen])))
				continue
			}
			if mode == modeAsks {
				// The answers are reported as declaration names, which is
				// the only channel a fake has to a case reading an
				// outline.
				answer(out, held.ID, told(answered))
				continue
			}
			answer(out, held.ID, symbols(mode))
		case "textDocument/definition":
			answer(out, held.ID, defined(mode, seen))
		case "textDocument/references":
			if mode == modeUnenclosed {
				answer(out, held.ID, fmt.Sprintf(
					`[{"uri":%q,"range":{"start":{"line":0,"character":0},`+
						`"end":{"line":0,"character":7}}}]`, seen))
				continue
			}
			if buffered(mode) {
				// From the buffer it holds, as a server answers: a use
				// in a file whose buffer no longer names the
				// declaration is a use the server cannot see.
				answer(out, held.ID, usedIn(sibling(seen), viewOf(holding, sibling(seen))))
				continue
			}
			if mode == modeLoading || mode == modeStuck {
				// What a server answers before it has read the
				// workspace: nothing, in the same shape as a real
				// answer. Reported as it stands it is a claim that the
				// declaration is unused.
				sending.Lock()
				ready := done
				sending.Unlock()
				if !ready {
					answer(out, held.ID, `[]`)
					continue
				}
			}
			answer(out, held.ID, references(where(mode, seen)))
		case "textDocument/implementation":
			if mode == modeUngated {
				// What a server with no compiler view answers: nothing,
				// in the same shape as having looked and found none.
				answer(out, held.ID, `[]`)
				continue
			}
			answer(out, held.ID, implementations(seen))
		case "textDocument/prepareCallHierarchy":
			if mode == modeUncallable {
				// What a server says when asked about a declaration
				// nothing can call: a type, an interface, a constant.
				oops(out, held.ID, "Store is not a function")
				continue
			}
			answer(out, held.ID, hierarchy(seen))
		case "callHierarchy/incomingCalls":
			answer(out, held.ID, incoming(seen))
		case "callHierarchy/outgoingCalls":
			answer(out, held.ID, outgoing(seen))
		case "textDocument/prepareTypeHierarchy":
			answer(out, held.ID, hierarchy(seen))
		case "typeHierarchy/supertypes":
			answer(out, held.ID, supertypes(seen))
		case "typeHierarchy/subtypes":
			answer(out, held.ID, subtypes(seen))
		case "textDocument/formatting":
			answer(out, held.ID, formatted(mode))
		case "workspace/symbol":
			if mode == modeEchoes {
				// One declaration per file this server is holding, named
				// for the file, which is the only way to see from
				// outside what it still has open.
				answer(out, held.ID, stillOpen(holding))
				continue
			}
			answer(out, held.ID, matches(mode, seen))
		case "workspace/didChangeWatchedFiles":
			synced = true

		case "workspace/willRenameFiles":
			if mode == modeSilentMove {
				answer(out, held.ID, `{"changes":{}}`)
				continue
			}
			answer(out, held.ID, moving(movingFile(held.Params)))

		case "textDocument/prepareRename":
			answer(out, held.ID, prepared(mode, held.Params))
		case "textDocument/rename":
			if mode == modeConflicts {
				oops(out, held.ID, "renaming this type conflicts with func in same block")
				continue
			}
			if buffered(mode) {
				buffer := viewOf(holding, sibling(seen))
				if mode == modeShort {
					buffer = ""
				}
				answer(out, held.ID, rewriting(seen, sibling(seen), buffer))
				continue
			}
			if mode == modeWatches && !synced {
				oops(out, held.ID, "Resource is out of sync with file system.")
				continue
			}
			if extracts(mode) {
				answer(out, held.ID, naming(seen, holding[seen], wanted(held.Params)))
				continue
			}
			answer(out, held.ID, renamed(mode, where(mode, seen)))
		case "textDocument/diagnostic":
			if mode == modeCompiles {
				// About the buffer this server is holding, so a case can
				// change the content and see the answer change.
				answer(out, held.ID, `{"kind":"full","items":`+wrong(holding[seen])+`}`)
				continue
			}
			answer(out, held.ID, reported())

		case "shutdown":
			answer(out, held.ID, `null`)
		case "exit":
			return 0
		default:
			if held.ID != nil {
				answer(out, held.ID, `null`)
			}
		}
	}
}

// request asks the client something and waits for its reply, reading
// past whatever else arrives first.
//
// It returns the reply's result verbatim, or the error the client sent
// back instead. Both are answers a case wants to be able to see.
func request(in *bufio.Reader, out io.Writer, id int64, rest string) string {
	write(out, fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":%s}`, id, rest))
	for {
		raw, err := frame(in)
		if err != nil {
			return "unanswered"
		}
		var reply struct {
			ID     *int64          `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(raw, &reply); err != nil {
			return "unreadable"
		}
		if reply.ID == nil || *reply.ID != id {
			continue
		}
		if reply.Error != nil {
			return "refused: " + reply.Error.Message
		}
		return string(reply.Result)
	}
}

// told reports what the client answered, one declaration per callback,
// named for what came back. A case reads the names.
func told(answered map[string]string) string {
	out := make([]string, 0, len(answered))
	for _, of := range []string{"configuration", "folders", "edit"} {
		out = append(out, fmt.Sprintf(
			`{"name":%q,"kind":12,
			  "range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}},
			  "selectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}`,
			of+"="+answered[of]))
	}
	return "[" + strings.Join(out, ",") + "]"
}

// opening is the text a didOpen carried.
func opening(params json.RawMessage) string {
	var held struct {
		TextDocument struct {
			Text string `json:"text"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(params, &held); err != nil {
		return ""
	}
	return held.TextDocument.Text
}

// changed is the text a didChange carried, whole-document.
func changed(params json.RawMessage) string {
	var held struct {
		ContentChanges []struct {
			Text string `json:"text"`
		} `json:"contentChanges"`
	}
	if err := json.Unmarshal(params, &held); err != nil || len(held.ContentChanges) == 0 {
		return ""
	}
	return held.ContentChanges[len(held.ContentChanges)-1].Text
}

// first is a buffer's first line, which is what the echoing mode reports
// as the name of what the file declares.
func first(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	return line
}

// oneName is one declaration by that name, so a case can read what the
// server was holding out of an outline.
func oneName(name string) string {
	return fmt.Sprintf(`[{"name":%q,"kind":23,
	  "range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}},
	  "selectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}]`, name)
}

// movingFile is the file a move names, as the client named it.
func movingFile(params json.RawMessage) string {
	var held struct {
		Files []struct {
			OldURI string `json:"oldUri"`
		} `json:"files"`
	}
	if err := json.Unmarshal(params, &held); err != nil || len(held.Files) == 0 {
		return ""
	}
	return held.Files[0].OldURI
}

// moving is what a move implies, which for a language tying a file's
// name to what it declares is a rename of the declaration inside the
// file that is about to move. jdtls answers exactly this.
func moving(of string) string {
	return fmt.Sprintf(`{"changes":{%q:[
	  {"range":%s,"newText":"Vault"}]}}`, of, at)
}

// moves reports whether the client said it sends file operations.
//
// Read from the initialise the fake kept, because that is what several
// servers read: jdtls and metals advertise willRenameFiles only to a
// client that declared it, and answer that they do no file operations
// at all otherwise.
func moves(params json.RawMessage) bool {
	var held struct {
		Capabilities struct {
			Workspace struct {
				FileOperations map[string]any `json:"fileOperations"`
			} `json:"workspace"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(params, &held); err != nil {
		return false
	}
	return held.Capabilities.Workspace.FileOperations["willRename"] == true
}

// receives reports whether the client said it can be sent diagnostics.
//
// Read from the initialise the fake kept, because a server has no other
// way to know and the ones that check simply publish nothing.
func receives(params json.RawMessage) bool {
	var held struct {
		Capabilities struct {
			TextDocument struct {
				PublishDiagnostics map[string]any `json:"publishDiagnostics"`
			} `json:"textDocument"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(params, &held); err != nil {
		return false
	}
	return held.Capabilities.TextDocument.PublishDiagnostics != nil
}

// rooted is the workspace the client opened, as it named it.
func rooted(params json.RawMessage) string {
	var held struct {
		RootURI string `json:"rootUri"`
	}
	if err := json.Unmarshal(params, &held); err != nil {
		return ""
	}
	return held.RootURI
}

// where is the file an answer names, which for one mode is a file
// outside the workspace techne was pointed at.
func where(mode, seen string) string {
	if mode == modeElsewhere {
		return "file://" + os.Getenv(elsewhere)
	}
	return seen
}

// oops sends an error response, which is how a server refuses a question
// that does not apply.
func oops(out io.Writer, id *int64, why string) {
	if id == nil {
		return
	}
	write(out, fmt.Sprintf(
		`{"jsonrpc":"2.0","id":%d,"error":{"code":-32602,"message":%q}}`, *id, why))
}

// document is the file a request names, so an answer can name it back.
func document(params json.RawMessage) string {
	var held struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(params, &held); err != nil {
		return ""
	}
	return held.TextDocument.URI
}

// capabilities is what the fake says it can do.
//
// The pull diagnostic provider is absent in the mode that publishes, so
// the engine has to choose between the two the way it would against a
// real server of each kind.
func capabilities(mode string, asked json.RawMessage) string {
	if mode == modeThin {
		// Rename without prepare, and nothing else at all. A client that
		// asks anyway is answered with an error, which is
		// indistinguishable from the question having no answer.
		return `{"capabilities":{"documentSymbolProvider":true,"renameProvider":true}}`
	}
	// A server that reports when it finishes rather than when asked, and
	// one that never finishes, both answer no diagnostic request: they
	// publish, or they would publish if they ever got that far.
	pull := `,"diagnosticProvider":{"interFileDependencies":false,"workspaceDiagnostics":false}`
	if mode == modePushes || mode == modeStuck || mode == modeUngated ||
		mode == modeSilentMove || buffered(mode) {
		pull = ""
	}
	actions := ""
	switch {
	case extracts(mode):
		actions = `,"codeActionProvider":{"codeActionKinds":["refactor.extract"],` +
			`"resolveProvider":true},"executeCommandProvider":{"commands":["fake.refactor"]}`
	case mode == modeCompiles:
		actions = `,"codeActionProvider":{"codeActionKinds":["quickfix"]}`
	}
	// Offered only to a client that said it sends file operations, which
	// is what the servers that answer it do.
	files := ""
	if mode != modeMoveless && moves(asked) {
		files = `,"workspace":{"fileOperations":{"willRename":{"filters":[` +
			`{"scheme":"file","pattern":{"glob":"**/*.fake","matches":"file"}}]}}}`
	}
	return `{"capabilities":{"documentSymbolProvider":true,"definitionProvider":true,` +
		`"referencesProvider":true,"implementationProvider":true,` +
		`"callHierarchyProvider":true,"workspaceSymbolProvider":true,` +
		`"typeHierarchyProvider":true,"documentFormattingProvider":true,` +
		`"renameProvider":{"prepareProvider":true}` + pull + files + actions + `}}`
}

// symbols is what the fake reports a document declares.
func symbols(mode string) string {
	switch mode {
	case modeEmpty:
		return `[]`
	case modeUnicode, "strict":
		// One name, on a line holding an emoji before it. The emoji is
		// two UTF-16 units and four bytes, so the name begins at unit 17
		// and byte 19: a client that took the one for the other reads
		// "e Stör" instead of "Störe".
		return `[
		  {"name":"Störe","kind":23,
		   "range":{"start":{"line":2,"character":17},"end":{"line":2,"character":22}},
		   "selectionRange":{"start":{"line":2,"character":17},"end":{"line":2,"character":22}}}]`
	case modeFlat:
		// The shape a server that does not build the tree sends, which
		// decoded as the tree yields names with every field empty.
		return `[
		  {"name":"Store","kind":23,
		   "location":{"uri":"","range":{"start":{"line":2,"character":0},
		     "end":{"line":4,"character":1}}}},
		  {"name":"After","kind":12,
		   "location":{"uri":"","range":{"start":{"line":8,"character":0},
		     "end":{"line":8,"character":20}}}}]`
	}
	// Store holds a field; Get is a method reported at the top level, as
	// a server does for a language whose methods are not written inside
	// the type. The last is a kind that declares nothing, which must be
	// dropped rather than reported.
	return `[
	  {"name":"Store","kind":23,
	   "range":{"start":{"line":2,"character":0},"end":{"line":4,"character":1}},
	   "selectionRange":{"start":{"line":2,"character":5},"end":{"line":2,"character":10}},
	   "children":[
	     {"name":"size","kind":8,
	      "range":{"start":{"line":3,"character":1},"end":{"line":3,"character":9}},
	      "selectionRange":{"start":{"line":3,"character":1},"end":{"line":3,"character":5}}}]},
	  {"name":"(*Store).Get","kind":6,"detail":"func() int",
	   "range":{"start":{"line":6,"character":0},"end":{"line":6,"character":40}},
	   "selectionRange":{"start":{"line":6,"character":17},"end":{"line":6,"character":20}}},
	  {"name":"After() : void","kind":12,"detail":"func()",
	   "range":{"start":{"line":8,"character":0},"end":{"line":8,"character":20}},
	   "selectionRange":{"start":{"line":8,"character":5},"end":{"line":8,"character":10}}},
	  {"name":"a string in a document","kind":15,
	   "range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}},
	   "selectionRange":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}]`
}

// at is the range covering Store's own name, which every answer below
// points at.
const at = `{"start":{"line":2,"character":5},"end":{"line":2,"character":10}}`

// defined is a definition, in each of the three shapes the protocol
// allows. A client reading only one of them reports the other two as a
// name that does not resolve.
func defined(mode, of string) string {
	switch mode {
	case modeOneLocation:
		return fmt.Sprintf(`{"uri":%q,"range":%s}`, of, at)
	case modeLinks:
		return fmt.Sprintf(`[{"targetUri":%q,"targetRange":%s,"targetSelectionRange":%s}]`,
			of, at, at)
	case modeUnresolved:
		return `null`
	}
	return fmt.Sprintf(`[{"uri":%q,"range":%s}]`, of, at)
}

// references are two uses, one inside a method and one inside a
// function, so an answer must name which declaration each is written in.
func references(of string) string {
	return fmt.Sprintf(`[
	  {"uri":%q,"range":{"start":{"line":6,"character":9},"end":{"line":6,"character":14}}},
	  {"uri":%q,"range":{"start":{"line":8,"character":5},"end":{"line":8,"character":10}}}]`,
		of, of)
}

func implementations(of string) string {
	return fmt.Sprintf(`[{"uri":%q,"range":%s}]`, of, at)
}

func hierarchy(of string) string {
	return fmt.Sprintf(`[{"name":"Get","kind":6,"uri":%q,
	  "range":{"start":{"line":6,"character":0},"end":{"line":6,"character":40}},
	  "selectionRange":{"start":{"line":6,"character":17},"end":{"line":6,"character":20}}}]`, of)
}

func incoming(of string) string {
	return fmt.Sprintf(`[{"from":{"name":"After","kind":12,"uri":%q,
	  "range":{"start":{"line":8,"character":0},"end":{"line":8,"character":20}},
	  "selectionRange":{"start":{"line":8,"character":5},"end":{"line":8,"character":10}}},
	  "fromRanges":[{"start":{"line":8,"character":5},"end":{"line":8,"character":10}}]}]`, of)
}

// supertypes is what the subject takes from: one type it incorporates.
func supertypes(of string) string {
	return fmt.Sprintf(`[{"name":"Store","kind":23,"uri":%q,
	  "range":{"start":{"line":2,"character":0},"end":{"line":4,"character":1}},
	  "selectionRange":{"start":{"line":2,"character":5},"end":{"line":2,"character":10}}}]`, of)
}

// subtypes is what takes from the subject.
func subtypes(of string) string {
	return fmt.Sprintf(`[{"name":"After","kind":12,"uri":%q,
	  "range":{"start":{"line":8,"character":0},"end":{"line":8,"character":20}},
	  "selectionRange":{"start":{"line":8,"character":5},"end":{"line":8,"character":10}}}]`, of)
}

// formatted is what the language's own formatter would change, which for
// a file already written that way is nothing.
func formatted(mode string) string {
	if mode == modeEmpty {
		return `[]`
	}
	return `[{"range":{"start":{"line":2,"character":0},"end":{"line":2,"character":4}},
	   "newText":"TYPE"}]`
}

func outgoing(of string) string {
	return fmt.Sprintf(`[{"to":{"name":"Store","kind":23,"uri":%q,
	  "range":{"start":{"line":2,"character":0},"end":{"line":4,"character":1}},
	  "selectionRange":{"start":{"line":2,"character":5},"end":{"line":2,"character":10}}},
	  "fromRanges":[{"start":{"line":6,"character":9},"end":{"line":6,"character":14}}]}]`, of)
}

// stillOpen names one declaration per file this server is holding.
func stillOpen(holding map[string]string) string {
	named := make([]string, 0, len(holding))
	for of := range holding {
		named = append(named, fmt.Sprintf(
			`{"name":%q,"kind":23,"location":{"uri":%q,"range":%s}}`,
			strings.TrimSuffix(path.Base(of), ".fake"), of, at))
	}
	slices.Sort(named)
	return "[" + strings.Join(named, ",") + "]"
}

// matches is a workspace query, in both shapes. The newer one may name a
// file and no range in it, which is the arm a client that assumed a
// range would drop.
func matches(mode, of string) string {
	if mode == modeUnranged {
		return fmt.Sprintf(`[{"name":"Store","kind":23,"location":{"uri":%q}}]`, of)
	}
	return fmt.Sprintf(`[
	  {"name":"Store","kind":23,"location":{"uri":%q,"range":%s}},
	  {"name":"size","kind":8,"location":{"uri":%q,
	    "range":{"start":{"line":3,"character":1},"end":{"line":3,"character":9}}}}]`,
		of, at, of)
}

func prepared(mode string, params json.RawMessage) string {
	switch mode {
	case modeUnnameable:
		return `null`
	case modeStrict:
		// The name on the unicode line begins at UTF-16 unit 17 and at
		// byte 19. A client that sent the byte count is pointing at
		// something else, and is told so the way a server tells anyone:
		// there is nothing here to rename.
		if positioned(params) != "2:17" {
			return `null`
		}
		return `{"start":{"line":2,"character":17},"end":{"line":2,"character":22}}`
	}
	return at
}

// positioned is the position a request carried, as line:character.
func positioned(params json.RawMessage) string {
	var held struct {
		Position struct {
			Line      int `json:"line"`
			Character int `json:"character"`
		} `json:"position"`
	}
	if err := json.Unmarshal(params, &held); err != nil {
		return ""
	}
	return fmt.Sprintf("%d:%d", held.Position.Line, held.Position.Character)
}

// wrong is what this server makes of a buffer: a fault on the line
// holding the word this language will not take.
//
// Read out of the buffer rather than from a fixed list, because what a
// gate asks is what the server makes of content nobody has written, and
// a fixed answer cannot tell that from the file on disk.
func wrong(buffer string) string {
	for i, line := range strings.Split(buffer, "\n") {
		at := strings.Index(line, broken)
		if at < 0 {
			continue
		}
		return fmt.Sprintf(`[{"range":{"start":{"line":%d,"character":%d},
		  "end":{"line":%d,"character":%d}},"severity":1,"code":"E900",
		  "source":"fakecheck","message":%q}]`,
			i, at, i, at+len(broken), broken+" is not a name this language takes")
	}
	return `[]`
}

// broken is the word the compiling mode will not take. A case writes it
// into content to make a change that parses and does not compile, which
// is the pair a parse gate cannot tell apart.
const broken = "undeclared"

// mends is the one quick fix this server offers for the fault it
// reported: the word it will not take, replaced with one it will.
func mends(params json.RawMessage, of string) string {
	var held struct {
		Context struct {
			Diagnostics []json.RawMessage `json:"diagnostics"`
			Only        []string          `json:"only"`
		} `json:"context"`
		Range json.RawMessage `json:"range"`
	}
	if err := json.Unmarshal(params, &held); err != nil ||
		len(held.Context.Diagnostics) == 0 || !slices.Contains(held.Context.Only, "quickfix") {
		// A server offers a fix for a fault it was told about. Asked
		// about a range with nothing wrong in it, it offers none.
		return `[]`
	}
	return fmt.Sprintf(`[{"title":"Declare it","kind":"quickfix","edit":{"changes":{%q:[
	  {"range":%s,"newText":"declared"}]}}}]`, of, held.Range)
}

// actions is the refactoring menu, which every server answers with
// several entries and none of which it marks preferred.
func actions(mode string) string {
	if !extracts(mode) {
		return `[]`
	}
	if mode == modeCommands {
		return `[{"title":"Extract into function","kind":"refactor.extract",
		  "command":{"title":"Extract","command":"fake.refactor","arguments":[]}}]`
	}
	return `[
	  {"title":"Extract into variable","kind":"refactor.extract","data":{"pick":"variable"}},
	  {"title":"Extract into function","kind":"refactor.extract","data":{"pick":"function"}}]`
}

// resolved is the edit behind one action, computed only when asked for,
// which is what every server that offers this kind does.
func resolved(params json.RawMessage, of, text string) string {
	var held struct {
		Title string `json:"title"`
		Kind  string `json:"kind"`
		Data  struct {
			Pick string `json:"pick"`
		} `json:"data"`
	}
	if err := json.Unmarshal(params, &held); err != nil {
		return `null`
	}
	return fmt.Sprintf(`{"title":%q,"kind":%q,"edit":%s}`,
		held.Title, held.Kind, lifted(of, text, held.Data.Pick))
}

// lifted is what the fake's extraction does: it appends a declaration to
// the file and names it whatever it likes.
func lifted(of, text, pick string) string {
	written := fmt.Sprintf("func %s() int { return 1 }\n", placeholder)
	if pick != "function" {
		// The other entry in the menu, so a case can tell which one was
		// taken rather than only that something was.
		written = "var extracted = 1\n"
	}
	end := strings.Count(text, "\n")
	return fmt.Sprintf(`{"changes":{%q:[
	  {"range":{"start":{"line":%d,"character":0},"end":{"line":%d,"character":0}},
	   "newText":%q}]}}`, of, end, end, written)
}

// functions is what a buffer declares, read out of the buffer itself:
// every line that opens a function, by the name it opens with.
func functions(text string) string {
	var out []string
	for i, line := range strings.Split(text, "\n") {
		name, at := opens(line)
		if name == "" {
			continue
		}
		out = append(out, fmt.Sprintf(
			`{"name":%q,"kind":12,
			  "range":{"start":{"line":%d,"character":0},"end":{"line":%d,"character":%d}},
			  "selectionRange":{"start":{"line":%d,"character":%d},
			                    "end":{"line":%d,"character":%d}}}`,
			name, i, i, len(line), i, at, i, at+len(name)))
	}
	return "[" + strings.Join(out, ",") + "]"
}

// opens is the name a line declares a function under, and where on the
// line it is written.
func opens(line string) (string, int) {
	const keyword = "func "
	if !strings.HasPrefix(line, keyword) {
		return "", 0
	}
	rest := line[len(keyword):]
	end := strings.Index(rest, "(")
	if end <= 0 {
		// A method, whose name is written after the receiver. The fake
		// reports what it can read, as a server reports what it parses.
		return "", 0
	}
	return rest[:end], len(keyword)
}

// wanted is the name a rename asks for.
func wanted(params json.RawMessage) string {
	var held struct {
		NewName string `json:"newName"`
	}
	if err := json.Unmarshal(params, &held); err != nil {
		return ""
	}
	return held.NewName
}

// naming renames the function the fake extracted, in the buffer it is
// holding rather than in the file on disk. A client converting the range
// against the file writes over something else.
func naming(of, text, fresh string) string {
	for i, line := range strings.Split(text, "\n") {
		at := strings.Index(line, placeholder)
		if at < 0 {
			continue
		}
		return fmt.Sprintf(`{"changes":{%q:[
		  {"range":{"start":{"line":%d,"character":%d},"end":{"line":%d,"character":%d}},
		   "newText":%q}]}}`, of, i, at, i, at+len(placeholder), fresh)
	}
	return `{"changes":{}}`
}

// viewOf is what this server currently makes of a file: the buffer it
// was given if it has one, and the file on disk otherwise. Every server
// works this way, and it is why a buffer nobody refreshed hides a
// change that is on disk.
func viewOf(holding map[string]string, of string) string {
	if buffer, given := holding[of]; given {
		return buffer
	}
	content, err := os.ReadFile(strings.TrimPrefix(of, "file://"))
	if err != nil {
		return ""
	}
	return string(content)
}

// sibling is the second file in the workspace, named from the first:
// the fake never learns the directory it is pointed at.
func sibling(of string) string { return strings.Replace(of, "a.fake", "b.fake", 1) }

// usedIn is one use of the declaration, in a second file.
//
// Read out of the buffer this server is holding for that file rather
// than out of a fixed answer, because that is where a server reads it: a
// file whose buffer no longer names the declaration holds no use of it
// as far as the server is concerned, whatever is on disk.
func usedIn(other, buffer string) string {
	if !strings.Contains(buffer, "Store") {
		return `[]`
	}
	return fmt.Sprintf(`[{"uri":%q,"range":{"start":{"line":0,"character":0},`+
		`"end":{"line":0,"character":5}}}]`, other)
}

// rewriting is what a server that edits only open buffers answers: the
// file it was asked about, and the second one only if the buffer it was
// given for that one still names the declaration.
func rewriting(of, other, buffer string) string {
	edits := fmt.Sprintf(`%q:[{"range":%s,"newText":"Vault"}]`, of, at)
	if strings.Contains(buffer, "Store") {
		edits += fmt.Sprintf(`,%q:[{"range":{"start":{"line":0,"character":0},`+
			`"end":{"line":0,"character":5}},"newText":"Vault"}]`, other)
	}
	return `{"changes":{` + edits + `}}`
}

// renamed is a workspace edit, in each of the two shapes. The map is
// what an older server sends; the ordered list is what a newer one
// sends, and it may carry file operations the map cannot express.
func renamed(mode, of string) string {
	edits := `[
	  {"range":{"start":{"line":6,"character":9},"end":{"line":6,"character":14}},
	   "newText":"Vault"},
	  {"range":{"start":{"line":2,"character":5},"end":{"line":2,"character":10}},
	   "newText":"Vault"}]`

	switch mode {
	case modeStrict:
		return fmt.Sprintf(`{"changes":{%q:[
		  {"range":{"start":{"line":2,"character":17},"end":{"line":2,"character":22}},
		   "newText":"Vault"}]}}`, of)
	case modeOrdered:
		return fmt.Sprintf(`{"documentChanges":[
		  {"textDocument":{"uri":%q,"version":1},"edits":%s},
		  {"kind":"rename","oldUri":%q,"newUri":%q}]}`,
			of, edits, of, strings.Replace(of, "a.fake", "vault.fake", 1))
	case modeOverlapping:
		return fmt.Sprintf(`{"changes":{%q:[
		  {"range":{"start":{"line":2,"character":0},"end":{"line":2,"character":10}},
		   "newText":"type Vault"},
		  {"range":{"start":{"line":2,"character":5},"end":{"line":2,"character":10}},
		   "newText":"Vault"}]}}`, of)
	}
	return fmt.Sprintf(`{"changes":{%q:%s}}`, of, edits)
}

// problems is one diagnostic of each severity the protocol grades, plus
// one with no severity at all, which a server is not required to send.
func problems() string {
	return `[
	  {"range":{"start":{"line":2,"character":5},"end":{"line":2,"character":10}},
	   "severity":1,"code":"E101","source":"fakecheck","message":"Store is never used"},
	  {"range":{"start":{"line":8,"character":5},"end":{"line":8,"character":10}},
	   "severity":2,"code":42,"message":"After shadows a builtin"},
	  {"range":{"start":{"line":3,"character":1},"end":{"line":3,"character":5}},
	   "message":"nobody graded this one"}]`
}

func reported() string {
	return `{"kind":"full","items":` + problems() + `}`
}

func answer(out io.Writer, id *int64, result string) {
	if id == nil {
		return
	}
	write(out, fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":%s}`, *id, result))
}

func write(out io.Writer, body string) {
	fmt.Fprintf(out, "Content-Length: %d\r\n\r\n%s", len(body), body)
}

// frame reads one message, the same way the client under test does.
func frame(from *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := from.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, split := strings.Cut(line, ":")
		if split && strings.EqualFold(strings.TrimSpace(name), "content-length") {
			length, _ = strconv.Atoi(strings.TrimSpace(value))
		}
	}
	if length < 0 {
		return nil, io.EOF
	}
	raw := make([]byte, length)
	_, err := io.ReadFull(from, raw)
	return raw, err
}

// pretending is a server declaration that runs this test binary as one.
//
// The mode that interrogates the client is declared with settings, so
// the section lookup is exercised rather than answered null twice.
func pretending(mode string) lsp.Server {
	var settings map[string]any
	if mode == modeAsks {
		settings = map[string]any{"fake": map[string]any{"strict": true}}
	}
	// Short, so a case about a server that never finishes loading does
	// not wait out the figure a real one is given.
	var waits time.Duration
	switch mode {
	case modeLoading:
		waits = 3 * loading
	case modeStuck:
		// Short, so a case about a server that never finishes does not
		// wait out the figure a real one is given.
		waits = time.Second
	}
	var lifts lsp.Refactor
	if extracts(mode) {
		lifts = lsp.Refactor{Kind: "refactor.extract", Titles: []string{"into function"}}
	}
	return lsp.Server{
		Loading:    waits,
		Extracts:   lifts,
		Name:       "fake",
		Command:    []string{os.Args[0]},
		LanguageID: "fake",
		Settings:   settings,
		Env:        map[string]string{acting: "1", pretend: mode},
		Serves: map[engine.Role]trust.Fidelity{
			engine.RoleOutline: trust.Resolved,
			engine.RoleSearch:  trust.Resolved,
			engine.RoleResolve: trust.Resolved,
			engine.RoleRelate:  trust.Resolved,
			engine.RolePlan:    trust.Resolved,
			engine.RoleVerify:  trust.Resolved,
		},
	}
}

// content is the file the fake reports about. The lines the fake's
// ranges name are the ones written here, so a position that is converted
// wrongly cuts the wrong text and the case says so.
const content = `package a

type Store struct {
	size int
}

func (s *Store) Get() int { return s.size }

func After() {}
`

// unicode holds a line where a character offset and a byte offset
// differ. The emoji is two UTF-16 units and four bytes, so every
// position after it on the line is two out if the two are confused.
const unicode = "package a\n\nvar 🌍 = 1; type Störe struct{}\n"

// declared is a language whose files this engine claims.
func declared() lang.Declaration {
	return lang.Declaration{
		Language:   "fake",
		Extensions: []string{".fake"},
		Comment:    lang.CommentStyle{Line: "// "},
		IsTest:     func(p string) bool { return filepath.Base(p) == "a_test.fake" },
		Namespace:  filepath.Dir,
		Visibility: func(name string) sema.Visibility {
			if name != "" && name[0] >= 'A' && name[0] <= 'Z' {
				return sema.Exported
			}
			return sema.Unexported
		},
	}
}

// workspace writes a file and returns the directory holding it.
func workspace(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, held := range files {
		at := filepath.Join(dir, name)
		assert.NoError(t, os.MkdirAll(filepath.Dir(at), 0o755),
			"the case can prepare the workspace")
		assert.NoError(t, os.WriteFile(at, []byte(held), 0o644),
			"the case can prepare the workspace")
	}
	return dir
}

// serving builds an engine over a workspace, and stops the server when
// the case is done.
func serving(t *testing.T, mode string, files map[string]string) *lsp.Engine {
	t.Helper()
	return servingAs(t, pretending(mode), files)
}

// servingAs is the same for a case that varies the declaration itself.
func servingAs(t *testing.T, server lsp.Server, files map[string]string) *lsp.Engine {
	t.Helper()
	e, err := lsp.New(workspace(t, files), declared(), server)
	assert.NoError(t, err, "an engine builds from a declaration and a server")
	stopping(t, e)
	return e
}

// reaching builds an engine whose server answers about a file outside
// the workspace, which is what a server indexing a wider tree than
// techne was pointed at does on every call.
func reaching(t *testing.T, files map[string]string) *lsp.Engine {
	t.Helper()
	away := filepath.Join(workspace(t, map[string]string{"far.fake": content}), "far.fake")

	server := pretending(modeElsewhere)
	server.Env[elsewhere] = away
	return servingAs(t, server, files)
}

// buildingOn is an engine over an empty workspace, for a case about the
// declaration rather than about any file.
func buildingOn(t *testing.T, server lsp.Server) *lsp.Engine {
	t.Helper()
	e, err := lsp.New(t.TempDir(), declared(), server)
	assert.NoError(t, err, "an engine builds from a declaration and a server")
	stopping(t, e)
	return e
}

// stopping closes an engine when the case is done, with a context of its
// own: the case's is cancelled the moment it finishes, and a shutdown
// needs one that outlives it.
func stopping(t *testing.T, e *lsp.Engine) {
	t.Helper()
	t.Cleanup(func() {
		ctx, stop := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
		defer stop()
		_ = e.Close(ctx)
	})
}

// names is what an outline found, in order.
func names(items []sema.Symbol) []string {
	out := make([]string, 0, len(items))
	for _, one := range items {
		out = append(out, one.Name)
	}
	return out
}

// named finds one declaration by name.
func named(items []sema.Symbol, name string) (sema.Symbol, bool) {
	for _, one := range items {
		if one.Name == name {
			return one, true
		}
	}
	return sema.Symbol{}, false
}
