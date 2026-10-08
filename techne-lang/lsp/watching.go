// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package lsp

import (
	"cmp"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/bmatcuk/doublestar/v4"
	"go.dokimi.dev/techne/lang"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// everything is the kinds of events of a watcher that names none: creation, change and deletion.
const everything = protocol.WatchKindCreate | protocol.WatchKindChange | protocol.WatchKindDelete

// watching is the state of the files under the workspace root that a server watches: the
// watchers of each registration of workspace/didChangeWatchedFiles that the server sent with
// client/registerCapability, and the stamp of each file that a watcher matched at the last walk
// of the workspace.
//
// A server reads a file without a buffer from disk, and reads it again only when the client
// reports a change of the file. gopls watches the Go files and the go.mod, go.sum and
// go.work files of the workspace, and type-checks a package against its own copy of such a file
// until that report arrives. [watching.changes] returns the reports of the changes on disk since
// the last walk.
//
// A walk follows the rules of [lang.Visit]: it leaves out a directory that [lang.Vendored]
// names, and a path that a .gitignore file excludes. A registration walks the workspace at once
// for the stamps that the next walk compares. A change between a read of the server and its
// registration is not reported.
//
// watching is safe for concurrent use.
type watching struct {
	root string

	mu sync.Mutex
	// watchers are the watchers of each registration, by the id of the registration.
	watchers map[string][]watcher
	// stamps is the stamp of each file that a watcher matched at the last walk, by absolute path.
	stamps map[string]stamp
}

// newWatching returns the state of a server without watchers, in the workspace at root, an
// absolute path.
func newWatching(root string) *watching {
	return &watching{root: root, watchers: map[string][]watcher{}, stamps: map[string]stamp{}}
}

// register records the watchers of each registration of params for
// workspace/didChangeWatchedFiles, by the rule of [watched], and walks the workspace for the
// stamps of the files that they match. A file that an earlier watcher matched keeps its stamp,
// so its change before the registration is still reported. A registration of another method,
// or with options that do not decode, is left out and walks nothing.
func (w *watching) register(params *protocol.RegistrationParams) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, one := range params.Registrations {
		var options protocol.DidChangeWatchedFilesRegistrationOptions
		if one.Method != protocol.MethodWorkspaceDidChangeWatchedFiles ||
			protocol.Unmarshal(one.RegisterOptions, &options) != nil {
			continue
		}
		var kept []watcher
		for _, held := range options.Watchers {
			kept = append(kept, watched(held))
		}
		w.watchers[one.ID] = kept
		for full, at := range w.walk() {
			if _, known := w.stamps[full]; !known {
				w.stamps[full] = at
			}
		}
	}
}

// unregister removes the watchers of each registration of params.
func (w *watching) unregister(params *protocol.UnregistrationParams) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, one := range params.Unregisterations {
		delete(w.watchers, one.ID)
	}
}

// changes walks the workspace and returns an event for each file that a watcher matches and that
// was created, changed or deleted since the last walk, in the order of the paths. It leaves out
// the file of a buffer of buffers, whose change [Engine.told] reports and whose deletion
// [Engine.release] reports, and an event of a kind that no watcher of the file names. It
// returns no event and walks nothing for a server without watchers.
func (w *watching) changes(buffers map[string]sent) []protocol.FileEvent {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.watchers) == 0 {
		return nil
	}
	now := w.walk()
	paths := slices.Collect(maps.Keys(now))
	for full := range w.stamps {
		if _, kept := now[full]; !kept {
			paths = append(paths, full)
		}
	}
	slices.Sort(paths)

	var out []protocol.FileEvent
	for _, full := range paths {
		was, existed := w.stamps[full]
		is, exists := now[full]
		var event protocol.FileChangeType
		var asked protocol.WatchKind
		switch {
		case !existed:
			event, asked = protocol.FileChangeTypeCreated, protocol.WatchKindCreate
		case !exists:
			event, asked = protocol.FileChangeTypeDeleted, protocol.WatchKindDelete
		case was != is:
			event, asked = protocol.FileChangeTypeChanged, protocol.WatchKindChange
		default:
			continue
		}
		if _, buffered := buffers[full]; !buffered && w.kinds(full)&asked != 0 {
			out = append(out, protocol.FileEvent{URI: uri.File(full), Type: event})
		}
	}
	w.stamps = now
	return out
}

// walk returns the stamp of each file under the root that a watcher matches, by absolute path,
// by the rules of [lang.Visit]. A file that the walk cannot read has no stamp. The caller has
// locked mu.
func (w *watching) walk() map[string]stamp {
	out := map[string]stamp{}
	fsys := os.DirFS(w.root)
	matched := func(p string) bool { return w.kinds(filepath.Join(w.root, p)) != 0 }
	_ = lang.Visit(fsys, ".", matched, func(p string, d fs.DirEntry) {
		if info, ok := lang.Info(fsys, p, d); ok {
			out[filepath.Join(w.root, p)] = stampOf(info)
		}
	})
	return out
}

// kinds returns the union of the kinds of the watchers that match the file at full, an absolute
// path, and zero when no watcher matches it. The caller has locked mu.
func (w *watching) kinds(full string) protocol.WatchKind {
	var out protocol.WatchKind
	for _, held := range w.watchers {
		for _, one := range held {
			if one.matches(full) {
				out |= one.kinds
			}
		}
	}
	return out
}

// watcher is one watcher of a registration: a glob pattern, and the kinds of the events that the
// client reports for the files that the pattern matches.
type watcher struct {
	pattern string
	// base is the directory of a relative pattern, or empty for a pattern of absolute paths.
	base  string
	kinds protocol.WatchKind
}

// watched returns the watcher of held. Its pattern has the glob syntax of LSP 3.17: *, ?, **,
// {a,b}, [0-9] and [!0-9]. A pattern of absolute paths, such as the /root/**/*.{go,mod} of
// gopls, applies to the slash form of an absolute path, as an editor applies it. A relative
// pattern applies under the directory of its base, a workspace folder or a URI. A watcher
// without kinds watches events of every kind, as the protocol states. A watcher without a pattern, or
// whose pattern does not parse, matches no file.
func watched(held protocol.FileSystemWatcher) watcher {
	made := watcher{kinds: cmp.Or(held.Kind, everything)}
	switch pattern := held.GlobPattern.(type) {
	case protocol.Pattern:
		made.pattern = string(pattern)
	case *protocol.RelativePattern:
		made.pattern = string(pattern.Pattern)
		switch base := pattern.BaseURI.(type) {
		case *protocol.WorkspaceFolder:
			made.base = base.URI.FsPath()
		case protocol.URI:
			made.base = uri.URI(base).FsPath()
		}
	}
	return made
}

// matches reports whether v matches the file at full, an absolute path: a pattern of absolute
// paths by the slash form of full, and a relative pattern by the path of full under its base.
func (v watcher) matches(full string) bool {
	name := full
	if v.base != "" {
		relative, err := filepath.Rel(v.base, full)
		if err != nil || !filepath.IsLocal(relative) {
			return false
		}
		name = relative
	}
	matched, _ := doublestar.Match(v.pattern, filepath.ToSlash(name))
	return matched
}
