// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"go.dokimi.dev/techne/lang"
	"go.dokimi.dev/techne/service/workspace/files"
	"go.dokimi.dev/techne/tool"
)

// warm is the number of workspaces besides the workspace of a session whose engines the session
// keeps open. The first call in one more workspace closes the workspaces that served a call
// least recently and serve none now.
const warm = 3

// workspace is the server of one directory, and the directory that its write tools write
// through.
type workspace struct {
	server *Server
	files  *files.Root
	// calls is the number of calls that the workspace serves now. A [Session] closes no
	// workspace with calls, and its lock guards the count.
	calls int
}

// opened returns the workspace of dir, an absolute path without symbolic links, with the mock
// languages of mocks in the syntax of [Build].
func opened(dir, mocks string) (*workspace, error) {
	root, err := files.Open(dir)
	if err != nil {
		return nil, err
	}
	built, err := Build(lang.Workspace{FS: root.FS(), Root: dir}, root, mocks)
	if err != nil {
		return nil, errors.Join(err, root.Close())
	}
	return &workspace{server: built, files: root}, nil
}

// close stops the engines of w within ctx and releases its directory.
func (w *workspace) close(ctx context.Context) error {
	return errors.Join(w.server.Close(ctx), w.files.Close())
}

// Session is the tools of one session of the server. A call runs in the workspace of the
// session, at the root of [Command.Root], unless it names another directory.
//
// With a trusted folder, every tool takes the field [tool.WorkingDirectory]. A call that sets it
// to a directory under a trusted folder runs in the workspace of that directory, and the paths of
// the call and of its answer are relative to that directory. The session opens the workspace of
// a directory at its first call, and keeps the engines of at most [warm] such workspaces open
// beside its own.
//
// A Session is safe for concurrent use.
type Session struct {
	// Tools are the tools that a client can call.
	Tools *tool.Registry

	// home is the workspace of the session, at root.
	home *workspace
	root string
	// trusted are the trusted folders, as absolute paths without symbolic links.
	trusted []string
	mocks   string

	// mu guards others, used and the calls of each workspace of others.
	mu sync.Mutex
	// others are the open workspaces of the directories that calls named, by directory.
	others map[string]*workspace
	// used are the directories of others, the one that served a call least recently first.
	used []string
}

// Open returns the session of command: the workspace at the root that [Root] resolves, and the
// trusted folders of command, which Root resolves too. Every workspace of the session has the
// mock languages of mocks, in the syntax of [Build]. The caller closes the session. ctx bounds
// the close of the workspace of the session when Open fails after it opened it.
//
// It returns an error for:
//
//   - a root or a trusted folder that does not exist
//   - a trusted folder that is not a directory
//   - a workspace that [Build] does not build
func Open(ctx context.Context, command Command, mocks string) (*Session, error) {
	root, err := Root(command.Root)
	if err != nil {
		return nil, err
	}
	trusted, err := folders(command.Trusted)
	if err != nil {
		return nil, err
	}
	home, err := opened(root, mocks)
	if err != nil {
		return nil, err
	}

	s := &Session{
		Tools: home.server.Tools, home: home, root: root, trusted: trusted, mocks: mocks,
		others: map[string]*workspace{},
	}
	if len(trusted) == 0 {
		return s, nil
	}
	about := "a directory under " + strings.Join(trusted, " or ") + " to run the call in, in place of " +
		"the workspace root. The paths of the call and of its answer are relative to it"
	s.Tools = tool.NewRegistry()
	for _, one := range home.server.Tools.Tools() {
		d, err := tool.Directed(one, about, s.run)
		if err == nil {
			err = s.Tools.Add(d)
		}
		if err != nil {
			return nil, errors.Join(fmt.Errorf("app: %w", err), home.close(ctx))
		}
	}
	return s, nil
}

// Close closes every workspace of the session within ctx, its own last. It returns the errors of
// the closes joined.
func (s *Session) Close(ctx context.Context) error {
	s.mu.Lock()
	others := slices.Collect(maps.Values(s.others))
	clear(s.others)
	s.used = nil
	s.mu.Unlock()

	var failed []error
	for _, w := range others {
		failed = append(failed, w.close(ctx))
	}
	return errors.Join(append(failed, s.home.close(ctx))...)
}

// run runs the tool named name with input in the workspace of the directory wd, as a
// [tool.Elsewhere]. Every workspace offers the tools of the workspace of the session, because
// [Build] builds each of them. It returns the error of [Session.directory] for wd.
func (s *Session) run(ctx context.Context, wd, name string, input json.RawMessage) (tool.Result, error) {
	dir, err := s.directory(wd)
	if err != nil {
		return tool.Result{}, err
	}
	w, closing, err := s.acquire(dir)
	if err != nil {
		return tool.Result{}, err
	}
	defer s.release(w)
	for _, one := range closing {
		stop(ctx, one)
	}
	served, _ := w.server.Tools.Tool(name)
	return served.Execute(ctx, input)
}

// directory returns wd as an absolute path without symbolic links, as [resolve] resolves it
// against the root of the session. It returns an error for a path that does not exist or is
// not a directory, and for a directory that is neither the root of the session nor under a
// trusted folder.
func (s *Session) directory(wd string) (string, error) {
	dir, err := resolve("the working directory", wd, s.root)
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return "", fmt.Errorf("app: the working directory %q is not a directory", wd)
	}
	if dir != s.root && !slices.ContainsFunc(s.trusted, func(folder string) bool { return within(dir, folder) }) {
		return "", fmt.Errorf("app: the working directory %q is under no trusted folder: %s",
			wd, strings.Join(s.trusted, ", "))
	}
	return dir, nil
}

// acquire returns the workspace of dir with one more call: the workspace of the session for its
// root, and otherwise the open workspace of dir, which acquire opens when there is none. It
// returns the workspaces that it removes from the session to keep [warm] open, which serve no
// call. The caller closes them.
func (s *Session) acquire(dir string) (*workspace, []*workspace, error) {
	if dir == s.root {
		return s.home, nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	w, open := s.others[dir]
	if !open {
		var err error
		if w, err = opened(dir, s.mocks); err != nil {
			return nil, nil, err
		}
		s.others[dir] = w
	}
	w.calls++
	s.used = append(slices.DeleteFunc(s.used, func(one string) bool { return one == dir }), dir)

	var closing []*workspace
	for _, least := range slices.Clone(s.used) {
		if len(s.others) <= warm {
			break
		}
		if idle := s.others[least]; idle.calls == 0 {
			closing = append(closing, idle)
			delete(s.others, least)
			s.used = slices.DeleteFunc(s.used, func(one string) bool { return one == least })
		}
	}
	return w, closing, nil
}

// release ends a call of w that [Session.acquire] counted.
func (s *Session) release(w *workspace) {
	if w == s.home {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	w.calls--
}

// stop closes w within [shutting], on a context that ctx does not cancel. It leaves out the
// error of the close, because w has no call in flight, and the next call in its directory opens
// it again.
func stop(ctx context.Context, w *workspace) {
	stopping, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutting)
	defer cancel()
	_ = w.close(stopping)
}

// folders returns each folder of given as [Root] resolves it. It returns an error for a folder
// that does not exist or is not a directory.
func folders(given []string) ([]string, error) {
	out := make([]string, 0, len(given))
	for _, folder := range given {
		resolved, err := resolve("the trusted folder", folder, "")
		if err != nil {
			return nil, err
		}
		if info, err := os.Stat(resolved); err != nil || !info.IsDir() {
			return nil, fmt.Errorf("app: the trusted folder %q is not a directory", folder)
		}
		out = append(out, resolved)
	}
	return out, nil
}

// within reports whether dir is folder or a directory under it. Both are absolute paths
// without symbolic links.
func within(dir, folder string) bool {
	relative, err := filepath.Rel(folder, dir)
	return err == nil && filepath.IsLocal(relative)
}
