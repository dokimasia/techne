// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsptest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.lsp.dev/uri"
)

// configured is the settings section that the Asks mode declares and asks the client for.
const configured = "fake"

// extractKind is the code action kind of the extraction that the Extracts and Commands modes
// offer.
const extractKind = "refactor.extract"

// performed is the command that the Commands mode performs an extraction with.
const performed = "fake.refactor"

// progressToken is the work-done progress token of the Loading and Stuck modes.
const progressToken = "loading"

// The ids of the requests that the script sends to the client.
const (
	idRegister      = 9001
	idConfiguration = 9002
	idFolders       = 9003
	idApplyEdit     = 9004
	idProgress      = 9100
	idPerform       = 9200
)

// The ranges in [Content] and [Emoji] that the responses of the script contain, as the protocol
// writes a range.
const (
	packageRange = `{"start":{"line":0,"character":0},"end":{"line":0,"character":9}}`
	packageName  = `{"start":{"line":0,"character":8},"end":{"line":0,"character":9}}`
	lineStart    = `{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}`
	unenclosed   = `{"start":{"line":0,"character":0},"end":{"line":0,"character":7}}`
	storeRange   = `{"start":{"line":2,"character":0},"end":{"line":4,"character":1}}`
	storeName    = `{"start":{"line":2,"character":5},"end":{"line":2,"character":10}}`
	typeKeyword  = `{"start":{"line":2,"character":0},"end":{"line":2,"character":4}}`
	sizeRange    = `{"start":{"line":3,"character":1},"end":{"line":3,"character":9}}`
	sizeName     = `{"start":{"line":3,"character":1},"end":{"line":3,"character":5}}`
	getRange     = `{"start":{"line":6,"character":0},"end":{"line":6,"character":43}}`
	getName      = `{"start":{"line":6,"character":16},"end":{"line":6,"character":19}}`
	storeInGet   = `{"start":{"line":6,"character":9},"end":{"line":6,"character":14}}`
	afterRange   = `{"start":{"line":8,"character":0},"end":{"line":8,"character":44}}`
	afterName    = `{"start":{"line":8,"character":5},"end":{"line":8,"character":10}}`
	storeInAfter = `{"start":{"line":8,"character":28},"end":{"line":8,"character":33}}`
	getInAfter   = `{"start":{"line":8,"character":37},"end":{"line":8,"character":40}}`
	stoereName   = `{"start":{"line":2,"character":17},"end":{"line":2,"character":22}}`
	fileRange    = `{"start":{"line":0,"character":0},"end":{"line":8,"character":44}}`
	shopRange    = `{"start":{"line":2,"character":0},"end":{"line":6,"character":43}}`
)

// The symbol kinds that the responses of the script use, as the protocol numbers them.
const (
	kindFile        = 1
	kindNamespace   = 3
	kindPackage     = 4
	kindClass       = 5
	kindMethod      = 6
	kindField       = 8
	kindConstructor = 9
	kindFunction    = 12
	kindString      = 15
	kindObject      = 19
	kindStruct      = 23
)

// message is one JSON-RPC message as the script reads it. A message without a method is a
// reply to a request of the script.
type message struct {
	ID     *int64          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// script is the state of one run of the scripted server.
type script struct {
	mode Mode
	in   *bufio.Reader

	// sending guards out and loaded. In the Loading mode a timer goroutine writes the end
	// of the progress job.
	sending sync.Mutex
	out     io.Writer
	loaded  bool

	// root is the workspace URI of initialize. seen is the document URI of the latest
	// request with a text document.
	root, seen string
	// client is the params of initialize, which declare the client capabilities.
	client json.RawMessage
	// replies are the client's replies to the requests of the Asks mode, by request name.
	replies map[string]string
	// holding is the buffer the client gave the server, and versions its version, by
	// document URI.
	holding  map[string]string
	versions map[string]int
	// synced reports whether the Watches mode's model of the files agrees with the disk.
	synced bool

	// outside is the absolute path that references and renames name, or empty.
	outside string
	// renames is the template of the answer to textDocument/rename, or empty.
	renames string
}

// serve runs the script over stdin and stdout until the client sends exit or closes stdin,
// and returns the exit status of the process.
func serve(mode Mode) int {
	if log := os.Getenv(envStarts); log != "" {
		if err := record(log, "started"); err != nil {
			return 4
		}
	}
	requests := os.Getenv(envRequests)
	s := &script{
		mode:     mode,
		in:       bufio.NewReader(os.Stdin),
		out:      os.Stdout,
		replies:  map[string]string{},
		holding:  map[string]string{},
		versions: map[string]int{},
		synced:   true,
		outside:  os.Getenv(envOutside),
		renames:  os.Getenv(envRenames),
	}
	for {
		raw, err := frame(s.in)
		if err != nil {
			return 0
		}
		var m message
		if err := json.Unmarshal(raw, &m); err != nil {
			return 1
		}
		if m.Method == "" {
			continue
		}
		if requests != "" && m.ID != nil {
			if err := record(requests, m.Method); err != nil {
				return 4
			}
		}
		if named := s.document(m.Params); named != "" {
			s.seen = named
		}
		if status, done := s.handle(m); done {
			return status
		}
	}
}

// record appends line to the file at log.
func record(log, line string) error {
	file, err := os.OpenFile(log, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(file, line); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

// handle acts on one request or notification. It reports the exit status and true when the
// process ends.
func (s *script) handle(m message) (int, bool) {
	switch m.Method {
	case "initialize":
		return s.initialize(m)
	case "exit":
		return 0, true
	case "textDocument/didOpen":
		s.opened(m.Params)
	case "textDocument/didChange":
		s.changed(m.Params)
	case "textDocument/didClose":
		delete(s.holding, s.seen)
		delete(s.versions, s.seen)
	case "workspace/didChangeWatchedFiles":
		s.synced = true
	default:
		if m.ID != nil {
			s.request(m)
		}
	}
	return 0, false
}

// initialize responds to the handshake. Before the response it sends a log message and a
// registration, and the Asks, Loading and Stuck modes send their own requests.
func (s *script) initialize(m message) (int, bool) {
	s.client = m.Params
	s.root = s.canonical(stringAt(m.Params, "rootUri"))
	s.seen = s.root + "/a" + Extension
	switch s.mode {
	case Silent:
		_, _ = io.Copy(io.Discard, s.in)
		return 0, true
	case Dies:
		fmt.Fprintln(os.Stderr, Dying)
		return 3, true
	}

	s.send(`{"jsonrpc":"2.0","method":"window/logMessage","params":{"type":3,"message":"starting"}}`)
	s.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"client/registerCapability",`+
		`"params":{"registrations":[]}}`, idRegister))
	if s.mode == Asks {
		s.replies["configuration"] = s.ask(idConfiguration, fmt.Sprintf(
			`"workspace/configuration","params":{"items":[{"section":%q},{"section":"absent"}]}`,
			configured))
		s.replies["folders"] = s.ask(idFolders, `"workspace/workspaceFolders","params":null`)
		s.replies["edit"] = s.ask(idApplyEdit, `"workspace/applyEdit","params":{"edit":{"changes":{}}}`)
	}
	if s.mode == Loading || s.mode == Stuck {
		s.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"window/workDoneProgress/create",`+
			`"params":{"token":%q}}`, idProgress, progressToken))
		s.send(progressed(`{"kind":"begin","title":"Loading"}`))
		if s.mode == Loading {
			go func() {
				time.Sleep(LoadTime)
				s.sending.Lock()
				defer s.sending.Unlock()
				write(s.out, progressed(`{"kind":"end"}`))
				s.loaded = true
			}()
		}
	}
	s.answer(m.ID, s.capabilities())
	return 0, false
}

// progressed is a $/progress notification of the loading job with value.
func progressed(value string) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","method":"$/progress","params":{"token":%q,"value":%s}}`,
		progressToken, value)
}

// isLoaded reports whether the Loading mode has ended its progress job.
func (s *script) isLoaded() bool {
	s.sending.Lock()
	defer s.sending.Unlock()
	return s.loaded
}

// opened keeps the buffer and the version of a didOpen notification. The Pushes and PushesOne
// modes publish diagnostics for the file.
func (s *script) opened(params json.RawMessage) {
	var held struct {
		TextDocument struct {
			Text    string `json:"text"`
			Version int    `json:"version"`
		} `json:"textDocument"`
	}
	if json.Unmarshal(params, &held) != nil {
		return
	}
	s.holding[s.seen] = held.TextDocument.Text
	s.versions[s.seen] = held.TextDocument.Version
	switch {
	case s.mode == PushesOne:
		faults := "[]"
		if strings.Contains(s.seen, "/two/") {
			faults = problems
		}
		s.publish(s.seen, faults)
	case s.mode == Pushes && s.declares("capabilities", "textDocument", "publishDiagnostics"):
		s.publish(s.seen, problems)
	}
}

// changed keeps the buffer and the version of a didChange notification. Every change replaces
// the whole document. The Watches mode stops agreeing with the disk.
func (s *script) changed(params json.RawMessage) {
	var held struct {
		TextDocument struct {
			Version int `json:"version"`
		} `json:"textDocument"`
		ContentChanges []struct {
			Text string `json:"text"`
		} `json:"contentChanges"`
	}
	if json.Unmarshal(params, &held) != nil || len(held.ContentChanges) == 0 {
		return
	}
	s.holding[s.seen] = held.ContentChanges[len(held.ContentChanges)-1].Text
	s.versions[s.seen] = held.TextDocument.Version
	if s.mode == Watches {
		s.synced = false
	}
}

// request responds to one request.
func (s *script) request(m message) {
	id := m.ID
	switch m.Method {
	case "textDocument/documentSymbol":
		s.answer(id, s.symbols())
	case "textDocument/definition":
		s.answer(id, s.definition(m.Params))
	case "textDocument/references":
		if s.mode == Hangs {
			return
		}
		s.answer(id, s.references())
	case "textDocument/implementation":
		if s.mode == Ungated {
			s.answer(id, "[]")
			return
		}
		s.answer(id, "["+location(s.seen, storeName)+"]")
	case "textDocument/prepareCallHierarchy":
		if s.mode == Uncallable {
			s.refuse(id, "Store is not a function")
			return
		}
		s.answer(id, "["+s.callable(position(m.Params))+"]")
	case "callHierarchy/incomingCalls":
		s.answer(id, s.incoming(stringAt(m.Params, "item", "name")))
	case "callHierarchy/outgoingCalls":
		s.answer(id, s.outgoing(stringAt(m.Params, "item", "name")))
	case "textDocument/prepareTypeHierarchy", "typeHierarchy/supertypes":
		s.answer(id, "["+s.item("Store", kindStruct, storeRange, storeName)+"]")
	case "typeHierarchy/subtypes":
		s.answer(id, "["+s.item("After", kindFunction, afterRange, afterName)+"]")
	case "textDocument/formatting":
		s.answer(id, s.formatting())
	case "textDocument/codeAction":
		s.answer(id, s.actions(m.Params))
	case "codeAction/resolve":
		s.answer(id, s.resolve(m.Params))
	case "workspace/executeCommand":
		s.ask(idPerform, fmt.Sprintf(`"workspace/applyEdit","params":{"edit":%s}`, s.lift("function")))
		s.answer(id, "null")
	case "workspace/willRenameFiles":
		s.answer(id, s.move(m.Params))
	case "textDocument/prepareRename":
		s.answer(id, s.prepare(m.Params))
	case "textDocument/rename":
		s.rename(id, m.Params)
	case "textDocument/diagnostic":
		s.answer(id, `{"kind":"full","items":`+s.diagnose(s.seen)+`}`)
	case "workspace/diagnostic":
		s.answer(id, s.workspaceReport())
	default:
		s.answer(id, "null")
	}
}

// capabilities is the answer to initialize for the mode. The pull diagnostic provider is
// absent in the modes that publish or never report, and willRenameFiles is advertised only
// to a client that declared it sends it.
func (s *script) capabilities() string {
	if s.mode == Thin {
		return `{"capabilities":{"documentSymbolProvider":true,"renameProvider":true}}`
	}
	fields := []string{
		`"documentSymbolProvider":true`, `"definitionProvider":true`, `"referencesProvider":true`,
		`"implementationProvider":true`, `"callHierarchyProvider":true`,
		`"typeHierarchyProvider":true`, `"documentFormattingProvider":true`,
		`"renameProvider":{"prepareProvider":true}`,
	}
	switch s.mode {
	case Pushes, Ungated, SilentMove, Opened, Short:
	default:
		fields = append(fields, fmt.Sprintf(
			`"diagnosticProvider":{"interFileDependencies":true,"workspaceDiagnostics":%t}`,
			s.mode == WorkspaceDiagnostics))
	}
	switch s.mode {
	case Extracts, Commands:
		fields = append(fields,
			fmt.Sprintf(`"codeActionProvider":{"codeActionKinds":[%q],"resolveProvider":true}`, extractKind),
			fmt.Sprintf(`"executeCommandProvider":{"commands":[%q]}`, performed))
	case Compiles, Unbound, WorkspaceDiagnostics:
		fields = append(fields, `"codeActionProvider":{"codeActionKinds":["quickfix"]}`)
	}
	if s.mode != Moveless && s.declares("capabilities", "workspace", "fileOperations", "willRename") {
		fields = append(fields, fmt.Sprintf(`"workspace":{"fileOperations":{"willRename":{"filters":[`+
			`{"scheme":"file","pattern":{"glob":"**/*%s","matches":"file"}}]}}}`, Extension))
	}
	return `{"capabilities":{` + strings.Join(fields, ",") + `}}`
}

// symbols is the answer to textDocument/documentSymbol for the mode.
func (s *script) symbols() string {
	switch s.mode {
	case Empty:
		return "[]"
	case Unicode, Strict:
		return "[" + symbol("Störe", kindStruct, "", stoereName, stoereName, "") + "]"
	case Flat:
		return fmt.Sprintf(`[{"name":"Store","kind":%d,"location":%s},`+
			`{"name":"Get","kind":%d,"location":%s,"containerName":"Store"},`+
			`{"name":"After","kind":%d,"location":%s}]`,
			kindStruct, location(s.seen, storeRange), kindMethod, location(s.seen, getRange),
			kindFunction, location(s.seen, afterRange))
	case Extracts, Commands:
		return functions(s.holding[s.seen])
	case Minified:
		return bundled(s.holding[s.seen])
	case Receivers:
		return "[" + strings.Join([]string{
			symbol("Store", kindStruct, "", ranged(2, 0, 19), ranged(2, 5, 10), ""),
			symbol("(*Store).Get", kindMethod, "func() int", ranged(4, 0, 38), ranged(4, 16, 19), ""),
			symbol("Cache", kindStruct, "", ranged(6, 0, 19), ranged(6, 5, 10), ""),
			symbol("(*Cache).Get", kindMethod, "func() int", ranged(8, 0, 38), ranged(8, 16, 19), ""),
		}, ",") + "]"
	case Impls:
		return "[" + strings.Join([]string{
			symbol("Store", kindStruct, "", ranged(2, 0, 19), ranged(2, 5, 10), ""),
			symbol("impl Getter for Store<T>", kindObject, "", ranged(4, 0, 38), ranged(4, 9, 14),
				symbol("Get", kindMethod, "", ranged(4, 16, 38), ranged(4, 16, 19), "")),
			symbol("Cache", kindStruct, "", ranged(6, 0, 19), ranged(6, 5, 10), ""),
			symbol("impl Getter for &Cache", kindObject, "", ranged(8, 0, 38), ranged(8, 9, 14),
				symbol("Get", kindMethod, "", ranged(8, 16, 38), ranged(8, 16, 19), "")),
		}, ",") + "]"
	case Wrapped:
		store := symbol("Store", kindClass, "", storeRange, storeName,
			symbol("Store", kindConstructor, "", sizeRange, sizeName, ""))
		shop := symbol("Shop", kindNamespace, "", shopRange, typeKeyword,
			store+","+symbol("Get", kindMethod, "func() int", getRange, getName, ""))
		after := symbol("<function>", kindFunction, "", afterRange, afterName,
			symbol("After", kindFunction, "func() int", afterRange, afterName, ""))
		return "[" + symbol("a.fake", kindFile, "", fileRange, lineStart, shop+","+after) + "]"
	}
	out := []string{
		symbol("Store", kindStruct, "", storeRange, storeName,
			symbol("size", kindField, "", sizeRange, sizeName, "")),
		symbol("(*Store).Get", kindMethod, "func() int", getRange, getName, ""),
		symbol("After(java.lang.String) : int", kindFunction, "func() int", afterRange, afterName, ""),
		symbol("a string in a document", kindString, "", lineStart, lineStart, ""),
	}
	if s.mode == Pointed {
		out = slices.Insert(out, 0, symbol("a", kindPackage, "", packageRange, packageName, ""))
	}
	return "[" + strings.Join(out, ",") + "]"
}

// symbol is one DocumentSymbol. Its children are DocumentSymbol objects joined with commas,
// or empty.
func symbol(name string, kind int, detail, whole, selection, children string) string {
	out := fmt.Sprintf(`{"name":%q,"kind":%d,"range":%s,"selectionRange":%s`, name, kind, whole, selection)
	if detail != "" {
		out += fmt.Sprintf(`,"detail":%q`, detail)
	}
	if children != "" {
		out += `,"children":[` + children + `]`
	}
	return out + "}"
}

// definition is the answer to textDocument/definition for the mode.
func (s *script) definition(params json.RawMessage) string {
	switch s.mode {
	case OneLocation:
		return location(s.seen, storeName)
	case Links:
		return fmt.Sprintf(`[{"targetUri":%q,"targetRange":%s,"targetSelectionRange":%s}]`,
			s.seen, storeRange, storeName)
	case Unresolved, Unbound:
		return "null"
	case Unicode, Strict:
		return "[" + location(s.seen, stoereName) + "]"
	case Pointed, Receivers, Impls, Wrapped:
		line, character := position(params)
		return "[" + location(s.seen, point(line, character)) + "]"
	case Minified:
		found := called(s.holding[s.seen], bundleCallee)
		if len(found) == 0 {
			return "null"
		}
		return "[" + location(s.seen, found[0]) + "]"
	}
	return "[" + location(s.seen, storeName) + "]"
}

// references is the answer to textDocument/references for the mode.
func (s *script) references() string {
	switch {
	case s.mode == Unenclosed:
		return "[" + location(s.seen, unenclosed) + "]"
	case s.mode == Opened || s.mode == Short:
		other := sibling(s.seen)
		at, used := use(s.view(other))
		if !used {
			return "[]"
		}
		return "[" + location(other, at) + "]"
	case s.mode == Scoped:
		other := sibling(s.seen)
		at, used := use(s.holding[other])
		if !used {
			return "[]"
		}
		return "[" + location(other, at) + "]"
	case (s.mode == Loading || s.mode == Stuck) && !s.isLoaded():
		return "[]"
	case s.mode == Receivers || s.mode == Impls:
		return "[]"
	case s.mode == Minified:
		var out []string
		for _, doc := range s.files() {
			for _, at := range called(s.view(doc), bundleCallee) {
				out = append(out, location(doc, at))
			}
		}
		return "[" + strings.Join(out, ",") + "]"
	}
	target := s.target()
	return "[" + location(target, storeInGet) + "," + location(target, storeInAfter) + "]"
}

// callable is the call hierarchy item at a position: After on line 8 and Get elsewhere.
func (s *script) callable(line, _ int) string {
	if line == 8 {
		return s.item("After", kindFunction, afterRange, afterName)
	}
	return s.item("Get", kindMethod, getRange, getName)
}

// incoming is the answer to callHierarchy/incomingCalls for the item named name. After
// calls Get, and nothing calls After.
func (s *script) incoming(name string) string {
	if name != "Get" {
		return "[]"
	}
	return fmt.Sprintf(`[{"from":%s,"fromRanges":[%s]}]`,
		s.item("After", kindFunction, afterRange, afterName), getInAfter)
}

// outgoing is the answer to callHierarchy/outgoingCalls for the item named name. After calls
// Get, and Get calls nothing. The item of Get names the file that [Outside] names, or the latest
// document, and the call site is a range in the file of After.
func (s *script) outgoing(name string) string {
	if name != "After" {
		return "[]"
	}
	callee := fmt.Sprintf(`{"name":"Get","kind":%d,"uri":%q,"range":%s,"selectionRange":%s}`,
		kindMethod, s.target(), getRange, getName)
	return fmt.Sprintf(`[{"to":%s,"fromRanges":[%s]}]`, callee, getInAfter)
}

// item is a call or type hierarchy item in the latest document.
func (s *script) item(name string, kind int, whole, selection string) string {
	return fmt.Sprintf(`{"name":%q,"kind":%d,"uri":%q,"range":%s,"selectionRange":%s}`,
		name, kind, s.seen, whole, selection)
}

// formatting is the answer to textDocument/formatting: one edit that writes the keyword on
// line 2 in capitals, or no edit in the Empty mode.
func (s *script) formatting() string {
	if s.mode == Empty {
		return "[]"
	}
	return `[{"range":` + typeKeyword + `,"newText":"TYPE"}]`
}

// actions is the answer to textDocument/codeAction for the mode.
func (s *script) actions(params json.RawMessage) string {
	switch s.mode {
	case Compiles, Unbound, WorkspaceDiagnostics:
		return s.mends(params)
	case Extracts:
		return fmt.Sprintf(`[{"title":"Extract into variable","kind":%q,"data":{"pick":"variable"}},`+
			`{"title":"Extract into function","kind":%q,"data":{"pick":"function"}}]`,
			extractKind, extractKind)
	case Commands:
		return fmt.Sprintf(`[{"title":"Extract into function","kind":%q,`+
			`"command":{"title":"Extract","command":%q,"arguments":[]}}]`, extractKind, performed)
	}
	return "[]"
}

// mends is the one quick fix for the first diagnostic of the request, which replaces the range
// of the diagnostic with "declared". A request without a diagnostic, or without the kind
// quickfix, receives no action.
func (s *script) mends(params json.RawMessage) string {
	var held struct {
		Context struct {
			Diagnostics []struct {
				Range json.RawMessage `json:"range"`
			} `json:"diagnostics"`
			Only []string `json:"only"`
		} `json:"context"`
	}
	if json.Unmarshal(params, &held) != nil || len(held.Context.Diagnostics) == 0 ||
		!slices.Contains(held.Context.Only, "quickfix") {
		return "[]"
	}
	return fmt.Sprintf(`[{"title":"Declare it","kind":"quickfix","edit":{"changes":{%q:[`+
		`{"range":%s,"newText":"declared"}]}}}]`, s.seen, held.Context.Diagnostics[0].Range)
}

// resolve is the answer to codeAction/resolve: the action with the edit of the extraction
// it picks.
func (s *script) resolve(params json.RawMessage) string {
	var held struct {
		Title string `json:"title"`
		Kind  string `json:"kind"`
		Data  struct {
			Pick string `json:"pick"`
		} `json:"data"`
	}
	if json.Unmarshal(params, &held) != nil {
		return "null"
	}
	return fmt.Sprintf(`{"title":%q,"kind":%q,"edit":%s}`, held.Title, held.Kind, s.lift(held.Data.Pick))
}

// lift is the edit of an extraction: it appends a function named [Placeholder] to the buffer
// of the latest document, or a variable when pick is not "function".
func (s *script) lift(pick string) string {
	written := fmt.Sprintf("func %s() int { return 1 }\n", Placeholder)
	if pick != "function" {
		written = "var extracted = 1\n"
	}
	end := strings.Count(s.holding[s.seen], "\n")
	return fmt.Sprintf(`{"changes":{%q:[{"range":%s,"newText":%q}]}}`, s.seen, point(end, 0), written)
}

// move is the answer to workspace/willRenameFiles: it renames Store inside the file that
// moves, as a server does for a language that ties a file name to its declaration.
func (s *script) move(params json.RawMessage) string {
	if s.mode == SilentMove {
		return `{"changes":{}}`
	}
	var held struct {
		Files []struct {
			OldURI string `json:"oldUri"`
		} `json:"files"`
	}
	if json.Unmarshal(params, &held) != nil || len(held.Files) == 0 {
		return "null"
	}
	moved := held.Files[0].OldURI
	edits := fmt.Sprintf(`%q:[{"range":%s,"newText":"Vault"}]`, moved, storeName)
	if other := sibling(moved); s.mode == Scoped {
		if at, used := use(s.holding[other]); used {
			edits += fmt.Sprintf(`,%q:[{"range":%s,"newText":"Vault"}]`, other, at)
		}
	}
	return `{"changes":{` + edits + `}}`
}

// prepare is the answer to textDocument/prepareRename for the mode.
func (s *script) prepare(params json.RawMessage) string {
	switch s.mode {
	case Unnameable:
		return "null"
	case Strict:
		if line, character := position(params); line != 2 || character != 17 {
			return "null"
		}
		return stoereName
	}
	return storeName
}

// rename responds to textDocument/rename for the mode, and in the [Default] mode with a map of
// two edits of one file, the later edit first.
func (s *script) rename(id *int64, params json.RawMessage) {
	switch {
	case s.mode == Conflicts:
		s.refuse(id, "renaming this type conflicts with func in same block")
	case s.mode == Opened || s.mode == Short || s.mode == Scoped:
		s.answer(id, s.rewrite())
	case s.mode == Watches && !s.synced:
		s.refuse(id, "Resource is out of sync with file system.")
	case s.mode == Extracts || s.mode == Commands:
		s.answer(id, s.nameExtracted(stringAt(params, "newName")))
	case s.renames != "":
		s.answer(id, strings.NewReplacer("{root}", s.root, "{file}", s.seen).Replace(s.renames))
	case s.mode == Strict:
		s.answer(id, fmt.Sprintf(`{"changes":{%q:[{"range":%s,"newText":"Vault"}]}}`, s.seen, stoereName))
	default:
		target := s.target()
		s.answer(id, fmt.Sprintf(`{"changes":{%q:[{"range":%s,"newText":"Vault"},`+
			`{"range":%s,"newText":"Vault"}]}}`, target, storeInGet, storeName))
	}
}

// rewrite renames Store in the latest document for the Opened and Short modes. The Opened mode
// also renames the first use of Store in b.fake while the server has a buffer of b.fake.
func (s *script) rewrite() string {
	edits := fmt.Sprintf(`%q:[{"range":%s,"newText":"Vault"}]`, s.seen, storeName)
	other := sibling(s.seen)
	if buffer, held := s.holding[other]; held && s.mode != Short {
		if at, used := use(buffer); used {
			edits += fmt.Sprintf(`,%q:[{"range":%s,"newText":"Vault"}]`, other, at)
		}
	}
	return `{"changes":{` + edits + `}}`
}

// nameExtracted renames the function that an extraction added to the buffer of the latest
// document. The range is counted in the buffer, which is not the file on disk.
func (s *script) nameExtracted(fresh string) string {
	for i, line := range strings.Split(s.holding[s.seen], "\n") {
		if at := strings.Index(line, Placeholder); at >= 0 {
			return fmt.Sprintf(`{"changes":{%q:[{"range":{"start":{"line":%d,"character":%d},`+
				`"end":{"line":%d,"character":%d}},"newText":%q}]}}`,
				s.seen, i, at, i, at+len(Placeholder), fresh)
		}
	}
	return `{"changes":{}}`
}

// diagnose is the list of diagnostics for the document doc, for the mode.
func (s *script) diagnose(doc string) string {
	switch s.mode {
	case Compiles, Unbound:
		return "[" + strings.Join(broken(s.view(doc)), ",") + "]"
	case WorkspaceDiagnostics:
		return "[" + strings.Join(s.faults(doc), ",") + "]"
	case Asks:
		var out []string
		for _, name := range []string{"configuration", "folders", "edit"} {
			out = append(out, note(name+"="+s.replies[name]))
		}
		return "[" + strings.Join(out, ",") + "]"
	case Echoes:
		first, _, _ := strings.Cut(s.holding[doc], "\n")
		held := make([]string, 0, len(s.holding))
		for one := range s.holding {
			held = append(held, path.Base(one))
		}
		slices.Sort(held)
		return "[" + note("first="+first) + "," + note("holding="+strings.Join(held, ",")) + "," +
			note("version="+strconv.Itoa(s.versions[doc])) + "]"
	case Extracts, Commands:
		var out []string
		for line := range strings.SplitSeq(s.holding[doc], "\n") {
			if name, _ := opens(line); name != "" {
				out = append(out, note("declares="+name))
			}
		}
		return "[" + strings.Join(out, ",") + "]"
	case Minified:
		return "[]"
	}
	return problems
}

// faults are the diagnostics of the WorkspaceDiagnostics mode for the document doc: an error at
// each occurrence of [Broken], and an error at the first use of Store while no file declares
// it.
func (s *script) faults(doc string) []string {
	out := broken(s.view(doc))
	for _, other := range s.files() {
		if strings.Contains(s.view(other), "type Store") {
			return out
		}
	}
	if at, used := use(s.view(doc)); used {
		out = append(out, fmt.Sprintf(`{"range":%s,"severity":1,"code":"E901","source":"fakecheck",`+
			`"message":"Store is not declared"}`, at))
	}
	return out
}

// workspaceReport returns the result of workspace/diagnostic, with a full report for each file
// that [script.files] returns.
func (s *script) workspaceReport() string {
	var out []string
	for _, doc := range s.files() {
		out = append(out, fmt.Sprintf(`{"kind":"full","uri":%q,"version":null,"items":[%s]}`,
			doc, strings.Join(s.faults(doc), ",")))
	}
	return `{"items":[` + strings.Join(out, ",") + `]}`
}

// files returns the URIs of the files with the [Extension] suffix under the workspace root and
// of the buffers of the server, sorted.
func (s *script) files() []string {
	var out []string
	_ = filepath.WalkDir(uri.URI(s.root).FsPath(), func(at string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Ext(at) == Extension {
			out = append(out, string(uri.File(at)))
		}
		return nil
	})
	for doc := range s.holding {
		if !slices.Contains(out, doc) {
			out = append(out, doc)
		}
	}
	slices.Sort(out)
	return out
}

// broken is one error diagnostic at each occurrence of [Broken] in text.
func broken(text string) []string {
	var out []string
	for i, line := range strings.Split(text, "\n") {
		for from := 0; ; {
			at := strings.Index(line[from:], Broken)
			if at < 0 {
				break
			}
			at += from
			out = append(out, fmt.Sprintf(`{"range":{"start":{"line":%d,"character":%d},`+
				`"end":{"line":%d,"character":%d}},"severity":1,"code":"E900","source":"fakecheck",`+
				`"message":%q}`, i, at, i, at+len(Broken), Broken+" is not a name this language takes"))
			from = at + len(Broken)
		}
	}
	return out
}

// problems are the [Default] diagnostics: an error with a string code, a warning with a
// numeric code, and a diagnostic without a severity.
const problems = `[` +
	`{"range":` + storeName + `,"severity":1,"code":"E101","source":"fakecheck",` +
	`"message":"Store is never used"},` +
	`{"range":` + afterName + `,"severity":2,"code":42,"message":"After shadows a builtin"},` +
	`{"range":` + sizeName + `,"message":"nobody graded this one"}]`

// note is an informational diagnostic on line 0 with message.
func note(message string) string {
	return fmt.Sprintf(`{"range":%s,"severity":3,"message":%q}`, lineStart, message)
}

// publish sends textDocument/publishDiagnostics for the document doc.
func (s *script) publish(doc, diagnostics string) {
	s.send(fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/publishDiagnostics",`+
		`"params":{"uri":%q,"diagnostics":%s}}`, doc, diagnostics))
}

// view is the text the server analyses for the document doc: the buffer the client gave it,
// or the file on disk when the client gave none.
func (s *script) view(doc string) string {
	if buffer, held := s.holding[doc]; held {
		return buffer
	}
	content, err := os.ReadFile(uri.URI(doc).FsPath())
	if err != nil {
		return ""
	}
	return string(content)
}

// target is the document that references and renames name: the file that [Outside] names,
// or the latest document.
func (s *script) target() string {
	if s.outside != "" {
		return string(uri.File(s.outside))
	}
	return s.seen
}

// declares reports whether the params of initialize set the value at keys to anything but
// null or false.
func (s *script) declares(keys ...string) bool {
	var at any
	if json.Unmarshal(s.client, &at) != nil {
		return false
	}
	for _, key := range keys {
		object, isObject := at.(map[string]any)
		if !isObject {
			return false
		}
		at = object[key]
	}
	return at != nil && at != false
}

// document is the URI of the text document that params name, or empty. The Canonical mode
// resolves symbolic links in it.
func (s *script) document(params json.RawMessage) string {
	return s.canonical(stringAt(params, "textDocument", "uri"))
}

// canonical resolves symbolic links in the file URI doc in the Canonical mode, and returns
// doc unchanged in every other mode. The directory is resolved when the file does not exist.
func (s *script) canonical(doc string) string {
	at := uri.URI(doc).FsPath()
	if s.mode != Canonical || at == "" {
		return doc
	}
	if real, err := filepath.EvalSymlinks(at); err == nil {
		return string(uri.File(real))
	}
	if dir, err := filepath.EvalSymlinks(filepath.Dir(at)); err == nil {
		return string(uri.File(filepath.Join(dir, filepath.Base(at))))
	}
	return doc
}

// stringAt is the string at keys in the JSON object raw, or empty.
func stringAt(raw json.RawMessage, keys ...string) string {
	var at any
	if json.Unmarshal(raw, &at) != nil {
		return ""
	}
	for _, key := range keys {
		object, isObject := at.(map[string]any)
		if !isObject {
			return ""
		}
		at = object[key]
	}
	held, _ := at.(string)
	return held
}

// position is the line and character of the position that params name.
func position(params json.RawMessage) (line, character int) {
	var held struct {
		Position struct {
			Line      int `json:"line"`
			Character int `json:"character"`
		} `json:"position"`
	}
	if json.Unmarshal(params, &held) != nil {
		return -1, -1
	}
	return held.Position.Line, held.Position.Character
}

// point is an empty range at a line and character.
func point(line, character int) string {
	return fmt.Sprintf(`{"start":{"line":%d,"character":%d},"end":{"line":%d,"character":%d}}`,
		line, character, line, character)
}

// location is a Location in the document doc.
func location(doc, whole string) string {
	return fmt.Sprintf(`{"uri":%q,"range":%s}`, doc, whole)
}

// ranged is the range of one line from one character to another.
func ranged(line, from, to int) string {
	return fmt.Sprintf(`{"start":{"line":%d,"character":%d},"end":{"line":%d,"character":%d}}`,
		line, from, line, to)
}

// use is the range of the first occurrence of Store in text, and whether there is one.
func use(text string) (string, bool) {
	for i, line := range strings.Split(text, "\n") {
		if at := strings.Index(line, "Store"); at >= 0 {
			return ranged(i, at, at+len("Store")), true
		}
	}
	return "", false
}

// sibling is the URI of b.fake in the directory of the a.fake that doc names.
func sibling(doc string) string {
	return strings.Replace(doc, "/a"+Extension, "/b"+Extension, 1)
}

// functions is the answer to textDocument/documentSymbol in the Extracts and Commands modes:
// one function for each line of text that opens one.
func functions(text string) string {
	var out []string
	for i, line := range strings.Split(text, "\n") {
		name, at := opens(line)
		if name == "" {
			continue
		}
		out = append(out, symbol(name, kindFunction, "", ranged(i, 0, len(line)), ranged(i, at, at+len(name)), ""))
	}
	return "[" + strings.Join(out, ",") + "]"
}

// bundleCallee is the name of the function that After calls in a [Bundle].
const bundleCallee = "F0"

// bundled is the answer to textDocument/documentSymbol in the Minified mode: one function for
// each func keyword of text, from the keyword to the brace that closes its body.
func bundled(text string) string {
	const keyword = "func "
	var out []string
	for i, line := range strings.Split(text, "\n") {
		for from := 0; ; {
			at := strings.Index(line[from:], keyword)
			if at < 0 {
				break
			}
			at += from
			name, _, _ := strings.Cut(line[at+len(keyword):], "(")
			end := strings.Index(line[at:], "}")
			if end < 0 {
				break
			}
			end += at + 1
			start := at + len(keyword)
			out = append(out, symbol(name, kindFunction, "", ranged(i, at, end), ranged(i, start, start+len(name)), ""))
			from = end
		}
	}
	return "[" + strings.Join(out, ",") + "]"
}

// called returns the range of each occurrence of name before a parenthesis in text, in order.
func called(text, name string) []string {
	var out []string
	for i, line := range strings.Split(text, "\n") {
		for from := 0; ; {
			at := strings.Index(line[from:], name+"(")
			if at < 0 {
				break
			}
			at += from
			out = append(out, ranged(i, at, at+len(name)))
			from = at + len(name)
		}
	}
	return out
}

// opens is the name of the function that line opens and the character it starts at. A method
// opens no function by this rule, because its name follows the receiver.
func opens(line string) (string, int) {
	const keyword = "func "
	rest, isFunc := strings.CutPrefix(line, keyword)
	if !isFunc {
		return "", 0
	}
	end := strings.Index(rest, "(")
	if end <= 0 {
		return "", 0
	}
	return rest[:end], len(keyword)
}

// send writes one message to the client.
func (s *script) send(body string) {
	s.sending.Lock()
	defer s.sending.Unlock()
	write(s.out, body)
}

// answer sends the result of the request id.
func (s *script) answer(id *int64, result string) {
	s.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":%s}`, *id, result))
}

// refuse sends an error response to the request id, with the LSP 3.17 code RequestFailed.
func (s *script) refuse(id *int64, why string) {
	s.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"error":{"code":-32803,"message":%q}}`, *id, why))
}

// ask sends a request to the client and reads messages until the reply to it arrives. It
// returns the result verbatim, "refused: " and the message of an error reply, or
// "unanswered" when the client closes the stream first. It drops every other message.
func (s *script) ask(id int64, method string) string {
	s.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":%s}`, id, method))
	for {
		raw, err := frame(s.in)
		if err != nil {
			return "unanswered"
		}
		var reply message
		if json.Unmarshal(raw, &reply) != nil || reply.Method != "" || reply.ID == nil || *reply.ID != id {
			continue
		}
		if reply.Error != nil {
			return "refused: " + reply.Error.Message
		}
		return string(reply.Result)
	}
}

// write frames one message with its Content-Length header.
func write(out io.Writer, body string) {
	fmt.Fprintf(out, "Content-Length: %d\r\n\r\n%s", len(body), body)
}

// frame reads one message and returns its body. It returns io.EOF for a header without a
// Content-Length.
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
	body := make([]byte, length)
	_, err := io.ReadFull(from, body)
	return body, err
}
