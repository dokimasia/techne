// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsptest

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"go.lsp.dev/uri"
)

// configured is the settings section that the Asks mode declares and asks the client for.
const configured = "fake"

// The sections under which the Asks mode requests the indentation of a file, as its
// declaration names them in [lsp.Server.Indentation].
const (
	indentOptions = "fake.options"
	indentSize    = "fake.size"
	indentSpaces  = "fake.spaces"
)

// extractKind is the code action kind of the extraction that the Extracts and Commands modes
// offer.
const extractKind = "refactor.extract"

// performed is the command that the Commands mode performs an extraction with.
const performed = "fake.refactor"

// progressToken is the work-done progress token of the Loading, Created and Stuck modes.
const progressToken = "loading"

// The error codes of LSP 3.17 with which the Cancels mode responds: RequestCancelled for a
// request that the server cancelled, and ContentModified for a request whose result a change
// of the content invalidated.
const (
	requestCancelled = -32800
	contentModified  = -32801
)

// The ids of the requests that the script sends to the client.
const (
	idRegister      = 9001
	idConfiguration = 9002
	idFolders       = 9003
	idApplyEdit     = 9004
	idIndentation   = 9005
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
	// getterName is the name of the interface of [Getter], which follows [Content].
	getterName = `{"start":{"line":9,"character":5},"end":{"line":9,"character":11}}`
)

// The names of [Literal] that the Shorthand mode responds with: the field size, the variable
// size, the shorthand properties of Make and of Copy, the type Item of the literal of Make, and
// the method size of Item.
const (
	fieldOfItem   = `{"start":{"line":3,"character":7},"end":{"line":3,"character":11}}`
	variableSize  = `{"start":{"line":6,"character":4},"end":{"line":6,"character":8}}`
	shorthandMake = `{"start":{"line":8,"character":32},"end":{"line":8,"character":36}}`
	shorthandCopy = `{"start":{"line":10,"character":32},"end":{"line":10,"character":36}}`
	itemInMake    = `{"start":{"line":8,"character":26},"end":{"line":8,"character":30}}`
	methodOfItem  = `{"start":{"line":12,"character":14},"end":{"line":12,"character":18}}`
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
	kindVariable    = 13
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

	// sending guards out, loaded, reported, opens, analysed and checks. In the Loading mode a
	// timer goroutine writes the end of the progress job, in the Quiet and Loads modes a
	// goroutine publishes the report of an open, and in the DiskChecks mode a goroutine runs
	// each check on disk.
	sending sync.Mutex
	out     io.Writer
	loaded  bool
	// reported is the report of each open document in the Quiet and Loads modes. opens counts
	// the opens and the closes of each document, so a delayed report of an earlier open is
	// dropped. analysed are the documents whose delayed report was sent.
	reported map[string]string
	opens    map[string]int
	analysed map[string]bool
	// checks counts the checks on disk of the DiskChecks mode.
	checks int

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

	// restarted reports that [RecordStarts] recorded a start before the start of this process.
	// muted reports that the Mutes mode responds with empty results.
	restarted, muted bool

	// delay is how long the Quiet and Loads modes take to publish the report of an open.
	delay time.Duration
}

// serve runs the script over stdin and stdout until the client sends exit or closes stdin,
// and returns the exit status of the process.
func serve(mode Mode) int {
	restarted := false
	if log := os.Getenv(envStarts); log != "" {
		if err := record(log, "started"); err != nil {
			return 4
		}
		recorded, err := os.ReadFile(log)
		if err != nil {
			return 4
		}
		restarted = bytes.Count(recorded, []byte("\n")) > 1
	}
	requests := os.Getenv(envRequests)
	delay := QuietDelay
	if given, err := time.ParseDuration(os.Getenv(envDelay)); err == nil {
		delay = given
	}
	s := &script{
		delay:     delay,
		restarted: restarted,
		mode:      mode,
		in:        bufio.NewReader(os.Stdin),
		out:       os.Stdout,
		replies:   map[string]string{},
		holding:   map[string]string{},
		versions:  map[string]int{},
		reported:  map[string]string{},
		opens:     map[string]int{},
		analysed:  map[string]bool{},
		synced:    true,
		outside:   os.Getenv(envOutside),
		renames:   os.Getenv(envRenames),
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
		if requests != "" && (m.ID != nil || m.Method == "textDocument/didSave") {
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
		if s.mode == Quiet {
			s.quietClose(s.seen)
		}
	case "workspace/didChangeWatchedFiles":
		s.synced = true
	case "initialized":
		switch s.mode {
		case DiskChecks:
			go s.diskCheck()
		case DiskStuck:
			s.send(progressOf(DiskToken, `{"kind":"begin","title":"check"}`))
		}
	case "textDocument/didSave":
		if s.mode == DiskChecks {
			go s.diskCheck()
		}
	case "textDocument/references":
		if s.mode == Exits && !s.restarted {
			return 0, true
		}
		s.request(m)
	default:
		if m.ID != nil {
			s.request(m)
		}
	}
	return 0, false
}

// scoped reports whether the mode reports and renames uses as the Scoped mode does.
func (s *script) scoped() bool { return s.mode == Scoped || s.mode == Projects }

// projectInfo responds to the command typescript.tsserverRequest with the request projectInfo
// of a file, as tsserver responds through typescript-language-server: the configuration file
// of the project when a tsconfig.json is in the directory of the file or above it and the
// client has a buffer of a file under the directory of that tsconfig.json, and the error No
// Project otherwise.
func (s *script) projectInfo(id *int64, params json.RawMessage) {
	var held struct {
		Command   string            `json:"command"`
		Arguments []json.RawMessage `json:"arguments"`
	}
	var args struct {
		File string `json:"file"`
	}
	if json.Unmarshal(params, &held) != nil || held.Command != tsserverCommand || len(held.Arguments) < 2 ||
		json.Unmarshal(held.Arguments[1], &args) != nil {
		s.refuse(id, "No Project.")
		return
	}
	root := uri.URI(s.root).FsPath()
	for dir := filepath.Dir(args.File); strings.HasPrefix(dir, root); dir = filepath.Dir(dir) {
		config := filepath.Join(dir, "tsconfig.json")
		if _, err := os.Stat(config); err != nil {
			continue
		}
		for open := range s.holding {
			if strings.HasPrefix(uri.URI(open).FsPath(), dir+string(filepath.Separator)) {
				s.answer(id, fmt.Sprintf(`{"type":"response","success":true,"body":{"configFileName":%q}}`, config))
				return
			}
		}
		break
	}
	s.refuse(id, "No Project.")
}

// tsserverCommand is the command of typescript-language-server to which the Projects mode
// responds.
const tsserverCommand = "typescript.tsserverRequest"

// lists are the requests whose result is a list, to which the Mutes mode responds with an empty
// list.
var lists = []string{
	"textDocument/documentSymbol", "textDocument/definition", "textDocument/references",
	"textDocument/implementation", "textDocument/prepareCallHierarchy", "callHierarchy/incomingCalls",
	"callHierarchy/outgoingCalls", "textDocument/prepareTypeHierarchy", "typeHierarchy/supertypes",
	"typeHierarchy/subtypes", "textDocument/codeAction", "textDocument/formatting",
}

// emptied is the result with which the Mutes mode responds to a request of method: a full
// report without items for textDocument/diagnostic, an empty list for a request whose result
// is a list, and null for any other.
func emptied(method string) string {
	switch {
	case method == "textDocument/diagnostic":
		return `{"kind":"full","items":[]}`
	case slices.Contains(lists, method):
		return "[]"
	}
	return "null"
}

// initialize responds to the handshake. Before the response it sends a log message and a
// registration, and the Asks, Loading, Created and Stuck modes send their own requests.
func (s *script) initialize(m message) (int, bool) {
	s.client = m.Params
	s.root = s.canonical(stringAt(m.Params, "rootUri"))
	s.seen = s.root + "/a" + Extension
	switch s.mode {
	case Silent:
		fmt.Fprintln(os.Stderr, Waiting)
		_, _ = io.Copy(io.Discard, s.in)
		return 0, true
	case Slow:
		time.Sleep(SlowStart)
	case Dies:
		fmt.Fprintln(os.Stderr, Dying)
		return 3, true
	case DiesLate:
		fmt.Fprintln(os.Stderr, Dying)
		time.Sleep(SlowStart)
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
		s.replies["indentation"] = s.ask(idIndentation, s.indenting())
	}
	if s.mode == Loading || s.mode == Stuck || s.mode == Created {
		s.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"window/workDoneProgress/create",`+
			`"params":{"token":%q}}`, idProgress, progressToken))
	}
	switch s.mode {
	case Loading, Stuck:
		s.send(progressed(`{"kind":"begin","title":"Loading"}`))
		if s.mode == Loading {
			go s.load(0)
		}
	case Created:
		go s.load(CreateTime)
	case Orphans:
		orphan()
	}
	s.answer(m.ID, s.capabilities())
	return 0, false
}

// indenting is the workspace/configuration request of the Asks mode for the indentation of a
// file. Its items request each section of the indentation for a.fake, then the width of b.fake,
// which a workspace can leave out, then the width without a scope, and last the width of the
// file that [Outside] names, when a declaration names one.
func (s *script) indenting() string {
	scoped := func(doc, section string) string {
		return fmt.Sprintf(`{"scopeUri":%q,"section":%q}`, doc, section)
	}
	items := []string{
		scoped(s.seen, indentOptions), scoped(s.seen, indentSize), scoped(s.seen, indentSpaces),
		scoped(s.root+"/b"+Extension, indentSize), fmt.Sprintf(`{"section":%q}`, indentSize),
	}
	if s.outside != "" {
		items = append(items, scoped(string(uri.File(s.outside)), indentSize))
	}
	return `"workspace/configuration","params":{"items":[` + strings.Join(items, ",") + `]}`
}

// orphan starts the child of the Orphans mode: the binary of the scripted server, which sleeps
// for [OrphanTime] with the stderr of the server and exits. The server does not wait for it.
func orphan() {
	child := exec.Command(os.Args[0])
	child.Env = append(os.Environ(), envOrphan+"=1")
	child.Stderr = os.Stderr
	_ = child.Start()
}

// progressed is a $/progress notification of the loading job with value.
func progressed(value string) string { return progressOf(progressToken, value) }

// progressOf is a $/progress notification of the job token with value.
func progressOf(token, value string) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","method":"$/progress","params":{"token":%q,"value":%s}}`,
		token, value)
}

// diskCheck runs one check on disk of the DiskChecks mode after [DiskDelay]: the begin of the
// job [DiskToken], the interim report and the report of each file with the [Extension] suffix
// under the workspace root as the file is on disk, and the end of the job.
func (s *script) diskCheck() {
	time.Sleep(DiskDelay)
	s.sending.Lock()
	defer s.sending.Unlock()
	s.checks++
	write(s.out, progressOf(DiskToken, `{"kind":"begin","title":"check"}`))
	interim := strings.TrimSuffix(strings.Repeat(note("interim")+",", DiskInterim), ",")
	for _, doc := range s.onDisk() {
		content, err := os.ReadFile(uri.URI(doc).FsPath())
		if err != nil {
			continue
		}
		reported := append(unsound(string(content)), note("checks="+strconv.Itoa(s.checks)))
		write(s.out, published(doc, "["+interim+"]"))
		write(s.out, published(doc, "["+strings.Join(reported, ",")+"]"))
	}
	write(s.out, progressOf(DiskToken, `{"kind":"end"}`))
}

// load runs the Loading job of the Loading and Created modes: after waiting for begun, it
// begins the job unless begun is zero, which states that the job has begun, and it ends the job
// [LoadTime] later.
func (s *script) load(begun time.Duration) {
	if begun > 0 {
		time.Sleep(begun)
		s.send(progressed(`{"kind":"begin","title":"Loading"}`))
	}
	time.Sleep(LoadTime)
	s.sending.Lock()
	defer s.sending.Unlock()
	write(s.out, progressed(`{"kind":"end"}`))
	s.loaded = true
}

// isLoaded reports whether the Loading mode has ended its progress job.
func (s *script) isLoaded() bool {
	s.sending.Lock()
	defer s.sending.Unlock()
	return s.loaded
}

// opened keeps the buffer and the version of a didOpen notification. The Pushes, PushesOne and
// Quiet modes publish diagnostics for the file.
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
	case s.mode == Quiet || s.mode == Loads:
		s.sending.Lock()
		s.reported[s.seen] = reportOf(held.TextDocument.Text)
		s.opens[s.seen]++
		opened := s.opens[s.seen]
		s.sending.Unlock()
		go s.delayed(s.seen, opened)
	}
}

// reportOf returns the report of the Quiet mode for text, which lists an error at each
// occurrence of [Broken].
func reportOf(text string) string { return "[" + strings.Join(broken(text), ",") + "]" }

// delayed publishes the report of the document doc in the Quiet and Loads modes after the delay
// of [Delaying] or [QuietDelay], unless the client closed or opened doc again after the open
// that opens counted as opened, and marks doc as analysed.
func (s *script) delayed(doc string, opened int) {
	time.Sleep(s.delay)
	s.sending.Lock()
	defer s.sending.Unlock()
	if s.opens[doc] != opened {
		return
	}
	s.analysed[doc] = true
	write(s.out, published(doc, s.reported[doc]))
}

// unanalysed reports whether the Loads mode has not yet sent the delayed report of doc.
func (s *script) unanalysed(doc string) bool {
	s.sending.Lock()
	defer s.sending.Unlock()
	return s.mode == Loads && !s.analysed[doc]
}

// quietChange keeps the report of the buffer of the document doc in the Quiet mode, and
// publishes it unless the last and the new report are both empty.
func (s *script) quietChange(doc string) {
	fresh := reportOf(s.holding[doc])
	s.sending.Lock()
	defer s.sending.Unlock()
	last := s.reported[doc]
	s.reported[doc] = fresh
	if last == "[]" && fresh == "[]" {
		return
	}
	write(s.out, published(doc, fresh))
}

// quietClose drops the report of the document doc in the Quiet mode and the delayed report of
// its open, and publishes an empty report for it after [QuietClose].
func (s *script) quietClose(doc string) {
	s.sending.Lock()
	delete(s.reported, doc)
	s.opens[doc]++
	s.sending.Unlock()
	time.Sleep(QuietClose)
	s.publish(doc, "[]")
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
	switch s.mode {
	case Watches:
		s.synced = false
	case Quiet:
		s.quietChange(s.seen)
	}
}

// request responds to one request.
func (s *script) request(m message) {
	id := m.ID
	if s.mode == Mutes && !s.restarted &&
		(m.Method == "textDocument/references" || m.Method == "textDocument/definition") {
		s.muted = true
	}
	if s.muted {
		s.answer(id, emptied(m.Method))
		return
	}
	if s.mode == Cancels && (m.Method == "textDocument/definition" || m.Method == "textDocument/references") {
		code, why := requestCancelled, "The request has been cancelled"
		if m.Method == "textDocument/references" {
			code, why = contentModified, "The content was modified, and the request cancelled"
		}
		s.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"error":{"code":%d,"message":%q}}`, *id, code, why))
		return
	}
	switch m.Method {
	case "textDocument/documentSymbol":
		s.answer(id, s.symbols())
	case "textDocument/definition":
		s.answer(id, s.definition(m.Params))
	case "textDocument/references":
		if s.mode == Hangs {
			return
		}
		if line, character := position(m.Params); s.mode == FromUse && line == 2 && character == 5 {
			s.answer(id, "[]")
			return
		}
		s.answer(id, s.references())
	case "textDocument/implementation":
		switch s.mode {
		case Ungated:
			s.answer(id, "[]")
			return
		case Untyped:
			s.refuse(id, NotAType)
			return
		case Contextual:
			s.answer(id, "["+location(s.seen, storeName)+","+location(s.seen, storeInAfter)+","+
				location(s.seen, getterName)+"]")
			return
		case Projected:
			s.answer(id, s.types(s.seen))
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
		if s.mode == Projects {
			s.projectInfo(id, m.Params)
			return
		}
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
		`"implementationProvider":true`,
		`"typeHierarchyProvider":true`, `"documentFormattingProvider":true`,
		`"renameProvider":{"prepareProvider":true}`,
	}
	if s.mode != Uncalled {
		fields = append(fields, `"callHierarchyProvider":true`)
	}
	switch s.mode {
	case Pushes, Ungated, SilentMove, Opened, Short, Quiet, Loads:
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
	case Minified, Uncalled:
		return "[" + strings.Join(bundled(s.holding[s.seen]), ",") + "]"
	case Aims:
		text := s.holding[s.seen]
		return "[" + strings.Join(append(variables(text), bundled(text)...), ",") + "]"
	case Nested:
		return "[" + strings.Join(nested(s.holding[s.seen]), ",") + "]"
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
	if s.unanalysed(s.seen) {
		return "[]"
	}
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
	case Exports:
		if line, _ := position(params); line == 0 {
			return "[" + location(s.seen, storeName) + "]"
		}
		return "[" + location(s.seen, unenclosed) + "]"
	case Redeclares:
		if line, _ := position(params); line == 8 {
			return "[" + location(s.seen, afterName) + "]"
		}
		return "[" + location(s.seen, storeName) + "]"
	case Projected:
		return s.imported(params)
	case Minified, Nested, Uncalled:
		found := called(s.holding[s.seen], bundleCallee)
		if len(found) == 0 {
			return "null"
		}
		return "[" + location(s.seen, found[0]) + "]"
	case Shorthand:
		switch line, _ := position(params); line {
		case 8:
			return "[" + strings.Join([]string{
				location(s.seen, variableSize), location(s.seen, fieldOfItem),
				location(s.seen, shorthandMake), location(s.seen, shorthandCopy), location(s.seen, itemInMake),
			}, ",") + "]"
		case 10:
			return "[" + strings.Join([]string{
				location(s.seen, variableSize), location(s.seen, fieldOfItem), location(s.seen, methodOfItem),
				location(s.seen, shorthandMake), location(s.seen, shorthandCopy),
			}, ",") + "]"
		}
		return "[]"
	}
	return "[" + location(s.seen, storeName) + "]"
}

// references is the answer to textDocument/references for the mode.
func (s *script) references() string {
	switch {
	case s.mode == Unenclosed:
		return "[" + location(s.seen, unenclosed) + "]"
	case s.mode == Opened || s.mode == Short || s.mode == Qualified:
		other := sibling(s.seen)
		at, used := use(s.view(other))
		if !used {
			return "[]"
		}
		if s.mode == Qualified {
			at = qualified(s.view(other))
		}
		return "[" + location(other, at) + "]"
	case s.scoped():
		other := sibling(s.seen)
		at, used := use(s.holding[other])
		if !used {
			return "[]"
		}
		return "[" + location(other, at) + "]"
	case (s.mode == Loading || s.mode == Stuck || s.mode == Created) && !s.isLoaded():
		return "[]"
	case s.mode == Receivers || s.mode == Impls || s.mode == Aims || s.mode == Shorthand:
		return "[]"
	case s.mode == Minified || s.mode == Nested:
		var out []string
		for _, doc := range s.files() {
			for _, at := range called(s.view(doc), bundleCallee) {
				out = append(out, location(doc, at))
			}
		}
		return "[" + strings.Join(out, ",") + "]"
	case s.mode == Uncalled:
		var out []string
		for _, doc := range s.files() {
			for _, at := range worded(s.view(doc), bundleCallee) {
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
	if other := sibling(moved); s.scoped() {
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
	case FromUse:
		if line, character := position(params); line == 2 && character == 5 {
			return "null"
		}
	}
	return storeName
}

// rename responds to textDocument/rename for the mode, and in the [Default] mode with a map of
// two edits of one file, the later edit first.
func (s *script) rename(id *int64, params json.RawMessage) {
	line, character := position(params)
	switch {
	case s.mode == FromUse && line == 2 && character == 5:
		s.answer(id, "null")
	case s.mode == Conflicts:
		s.refuse(id, "renaming this type conflicts with func in same block")
	case s.mode == Opened || s.mode == Short || s.mode == Qualified || s.scoped():
		s.answer(id, s.rewrite())
	case s.mode == Watches && !s.synced:
		s.refuse(id, "Resource is out of sync with file system.")
	case s.mode == Extracts || s.mode == Commands:
		fresh := stringAt(params, "newName")
		if !identifier(fresh) {
			s.refuse(id, fmt.Sprintf("%s: %q", InvalidName, fresh))
			return
		}
		s.answer(id, s.nameExtracted(fresh))
	case s.mode == Aims:
		s.answer(id, s.renameAt(params))
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

// renameAt is the answer to textDocument/rename in the Aims mode: one edit of the latest
// document that replaces the word at the position of params with the new name. A position
// outside every word replaces nothing and inserts the name there.
func (s *script) renameAt(params json.RawMessage) string {
	line, character := position(params)
	lines := strings.Split(s.holding[s.seen], "\n")
	from, to := character, character
	if line >= 0 && line < len(lines) {
		text := []rune(lines[line])
		for from > 0 && from <= len(text) && identifying(text[from-1]) {
			from--
		}
		for to < len(text) && identifying(text[to]) {
			to++
		}
	}
	return fmt.Sprintf(`{"changes":{%q:[{"range":%s,"newText":%q}]}}`,
		s.seen, ranged(line, from, to), stringAt(params, "newName"))
}

// identifier reports whether name is an identifier: a letter or an underscore, then letters,
// digits and underscores.
func identifier(name string) bool {
	for i, r := range name {
		if r != '_' && !unicode.IsLetter(r) && (i == 0 || !unicode.IsDigit(r)) {
			return false
		}
	}
	return name != ""
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
	case Compiles, Unbound, DiskChecks, DiskStuck:
		return "[" + strings.Join(broken(s.view(doc)), ",") + "]"
	case WorkspaceDiagnostics:
		return "[" + strings.Join(s.faults(doc), ",") + "]"
	case Asks:
		var out []string
		for _, name := range []string{"configuration", "folders", "edit", "indentation"} {
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
	case Minified, Nested, Uncalled:
		return "[]"
	}
	return problems
}

// faults are the diagnostics of the WorkspaceDiagnostics mode for the document doc: an error at
// each occurrence of [Broken], an error at the first use of Store while no file declares it,
// and the error [EmptyFile] for an empty document.
func (s *script) faults(doc string) []string {
	out := broken(s.view(doc))
	if s.view(doc) == "" {
		out = append(out, fmt.Sprintf(`{"range":%s,"severity":1,"code":"E902","source":"fakecheck","message":%q}`,
			ranged(0, 0, 0), EmptyFile))
	}
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
	out := s.onDisk()
	for doc := range s.holding {
		if !slices.Contains(out, doc) {
			out = append(out, doc)
		}
	}
	slices.Sort(out)
	return out
}

// onDisk returns the URIs of the files with the [Extension] suffix under the workspace root,
// sorted. It reads no buffer, so a goroutine of the DiskChecks mode can call it.
func (s *script) onDisk() []string {
	var out []string
	_ = filepath.WalkDir(uri.URI(s.root).FsPath(), func(at string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Ext(at) == Extension {
			out = append(out, string(uri.File(at)))
		}
		return nil
	})
	return out
}

// broken is one error diagnostic at each occurrence of [Broken] in text.
func broken(text string) []string {
	return occurrences(text, Broken, `"code":"E900","source":"fakecheck"`, Broken+" is not a name this language takes")
}

// unsound is one error diagnostic at each occurrence of [Unsound] in text, as the check on disk
// of the DiskChecks mode reports it.
func unsound(text string) []string {
	return occurrences(text, Unsound, `"code":"E428","source":"fakedisk"`, Unsound+" is defined twice")
}

// occurrences is one error diagnostic at each occurrence of word in text, with the code and the
// source that labels writes and message.
func occurrences(text, word, labels, message string) []string {
	var out []string
	for i, line := range strings.Split(text, "\n") {
		for from := 0; ; {
			at := strings.Index(line[from:], word)
			if at < 0 {
				break
			}
			at += from
			out = append(out, fmt.Sprintf(`{"range":{"start":{"line":%d,"character":%d},`+
				`"end":{"line":%d,"character":%d}},"severity":1,%s,"message":%q}`,
				i, at, i, at+len(word), labels, message))
			from = at + len(word)
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
func (s *script) publish(doc, diagnostics string) { s.send(published(doc, diagnostics)) }

// published is the textDocument/publishDiagnostics notification of diagnostics for the
// document doc.
func published(doc, diagnostics string) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/publishDiagnostics",`+
		`"params":{"uri":%q,"diagnostics":%s}}`, doc, diagnostics)
}

// types is the answer of the Projected mode to textDocument/implementation in the document doc:
// the name of each type of a line that starts with type.
func (s *script) types(doc string) string {
	var found []string
	for n, line := range strings.Split(s.view(doc), "\n") {
		if rest, typed := strings.CutPrefix(line, "type "); typed {
			name := word(rest)
			found = append(found, location(doc, ranged(n, len("type "), len("type ")+len(name))))
		}
	}
	return "[" + strings.Join(found, ",") + "]"
}

// imports starts the line of the Projected mode that binds Store, as an import binds a name.
const imports = "// imports "

// imported is the answer of the Projected mode to textDocument/definition, by the rule of
// [Projected].
func (s *script) imported(params json.RawMessage) string {
	line, character := position(params)
	lines := strings.Split(s.view(s.seen), "\n")
	if line < 0 || line >= len(lines) || wordAt(lines[line], character) != "Store" {
		return "[]"
	}
	if strings.HasPrefix(lines[line], imports) {
		return "[" + location(s.root+"/a"+Extension, storeName) + "]"
	}
	for n, one := range lines {
		if rest, binds := strings.CutPrefix(one, imports); binds && strings.HasPrefix(rest, "Store") {
			return "[" + location(s.seen, ranged(n, len(imports), len(imports)+len("Store"))) + "]"
		}
	}
	return "[]"
}

// wordAt is the identifier of line that contains the character at column, or the empty
// string when none does.
func wordAt(line string, column int) string {
	if column < 0 || column >= len(line) || !identifying(rune(line[column])) {
		return ""
	}
	start, end := column, column
	for start > 0 && identifying(rune(line[start-1])) {
		start--
	}
	for end < len(line) && identifying(rune(line[end])) {
		end++
	}
	return line[start:end]
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

// qualified is the range of the first occurrence of Store in text together with the qualifier
// before it, such as a. in a.Store, or the empty string when text has no occurrence.
func qualified(text string) string {
	for i, line := range strings.Split(text, "\n") {
		at := strings.Index(line, "Store")
		if at < 0 {
			continue
		}
		start := at
		for start > 1 && line[start-1] == '.' && identifying(rune(line[start-2])) {
			start--
			for start > 0 && identifying(rune(line[start-1])) {
				start--
			}
		}
		return ranged(i, start, at+len("Store"))
	}
	return ""
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

// bundled returns the symbols of the answer to textDocument/documentSymbol in the Minified mode:
// one function for each func keyword of text, from the keyword to the brace that closes its
// body.
func bundled(text string) []string {
	return bodied(text, func(name, whole, selection string) string {
		return symbol(name, kindFunction, "", whole, selection, "")
	})
}

// nested returns the symbols of the answer to textDocument/documentSymbol in the Nested mode: for
// each func keyword of text, a class that contains one method, both with the name and the range
// of the function.
func nested(text string) []string {
	return bodied(text, func(name, whole, selection string) string {
		return symbol(name, kindClass, "", whole, selection, symbol(name, kindMethod, "", whole, selection, ""))
	})
}

// variables returns the symbols that the answer to textDocument/documentSymbol in the Aims mode
// lists before those of [bundled]: a variable for each name that a line that starts with var
// lists before =. Each name of one line has the range of the line after var, and the range of
// the name as its selection, as gopls gives each name of var a, b = 1, 2 the range of
// a, b = 1, 2.
func variables(text string) []string {
	const keyword = "var "
	var out []string
	for i, line := range strings.Split(text, "\n") {
		listed, declares := strings.CutPrefix(line, keyword)
		if !declares {
			continue
		}
		names, _, _ := strings.Cut(listed, " =")
		whole, from := ranged(i, len(keyword), len(line)), len(keyword)
		for name := range strings.SplitSeq(names, ", ") {
			out = append(out, symbol(name, kindVariable, "", whole, ranged(i, from, from+len(name)), ""))
			from += len(name) + len(", ")
		}
	}
	return out
}

// bodied returns the symbols that render returns for each func keyword of text. It passes the
// name of the function, its range from the keyword to the brace that closes its body, and the
// range of its name.
func bodied(text string, render func(name, whole, selection string) string) []string {
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
			out = append(out, render(name, ranged(i, at, end), ranged(i, start, start+len(name))))
			from = end
		}
	}
	return out
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

// worded returns the range of each occurrence of name as a word in text, which the Uncalled
// mode reports as a reference.
func worded(text, name string) []string {
	var out []string
	for i, line := range strings.Split(text, "\n") {
		for from := 0; ; {
			at := strings.Index(line[from:], name)
			if at < 0 {
				break
			}
			at += from
			from = at + len(name)
			before := at == 0 || !identifying(rune(line[at-1]))
			after := from == len(line) || !identifying(rune(line[from]))
			if before && after {
				out = append(out, ranged(i, at, from))
			}
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
