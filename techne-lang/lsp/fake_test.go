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
	"path/filepath"
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
)

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
	// answered is what the client said when this server asked it
	// something, so a case can see a callback that a passing outline
	// would otherwise hide.
	answered := map[string]string{}
	// The workspace is a temporary directory the fake never learns the
	// name of, so every answer naming a file echoes back the one the
	// request named.
	var seen string

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
			answer(out, held.ID, capabilities(mode))

		case "textDocument/didOpen":
			if mode == modePushes {
				// A server with no pull request reports when it has
				// finished rather than when it is asked.
				write(out, fmt.Sprintf(
					`{"jsonrpc":"2.0","method":"textDocument/publishDiagnostics",`+
						`"params":{"uri":%q,"diagnostics":%s}}`, seen, problems()))
			}

		case "textDocument/documentSymbol":
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
			answer(out, held.ID, matches(mode, seen))
		case "textDocument/prepareRename":
			answer(out, held.ID, prepared(mode, held.Params))
		case "textDocument/rename":
			answer(out, held.ID, renamed(mode, where(mode, seen)))
		case "textDocument/diagnostic":
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
func capabilities(mode string) string {
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
	if mode == modePushes || mode == modeStuck || mode == modeUngated {
		pull = ""
	}
	return `{"capabilities":{"documentSymbolProvider":true,"definitionProvider":true,` +
		`"referencesProvider":true,"implementationProvider":true,` +
		`"callHierarchyProvider":true,"workspaceSymbolProvider":true,` +
		`"typeHierarchyProvider":true,"documentFormattingProvider":true,` +
		`"renameProvider":{"prepareProvider":true}` + pull + `}}`
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
	return lsp.Server{
		Loading:    waits,
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
		assert.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(held), 0o644),
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
