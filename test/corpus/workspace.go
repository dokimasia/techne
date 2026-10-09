// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: MIT

package corpus

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// The files that a run writes into the git directory of a clone.
const (
	// marked marks a clone as one that a run made, so a run resets it.
	marked = "techne-corpus"
	// prepared holds the digest of the prepare steps that ran, followed by
	// the untracked files that they left, one per line.
	prepared = "techne-corpus-prepared"
)

// ErrForeign reports a directory that a run did not clone, which a run does
// not change.
var ErrForeign = errors.New("corpus: the directory is not a clone of the corpus")

// ErrReadOnly reports a change to a repository that the manifest names by
// path.
var ErrReadOnly = errors.New("corpus: the repository is read-only")

// Workspace is the directory of one repository of the corpus.
type Workspace struct {
	// Log receives each command and its output.
	Log io.Writer
	// Root is the directory of the repository.
	Root string
	// Tools is the directory into which the prepare steps install tools,
	// outside Root. A read-only repository has none.
	Tools string
	// Repository is the entry of the manifest.
	Repository Repository
}

// Open returns the workspace of r.
//
// Open clones a repository with a URL into a directory under dir, at its
// commit, when the clone is missing, and moves a clone at another commit to
// it. It then resets the clone, which removes the files that an interrupted
// run left. Open returns ErrForeign for a directory under dir that a run did
// not clone.
//
// A repository without a URL is the directory r.Path, relative to base.
func Open(ctx context.Context, dir, base string, r Repository, log io.Writer) (*Workspace, error) {
	if !r.Writable() {
		root, err := filepath.Abs(filepath.Join(base, r.Path))
		if err != nil {
			return nil, fmt.Errorf("corpus: %s: %w", r.Name, err)
		}
		if info, err := os.Stat(root); err != nil || !info.IsDir() {
			return nil, fmt.Errorf("corpus: %s: %s is not a directory", r.Name, root)
		}
		return &Workspace{Repository: r, Root: root, Log: log}, nil
	}

	w := &Workspace{
		Repository: r,
		Root:       filepath.Join(dir, r.Name),
		Tools:      filepath.Join(dir, r.Name+".tools"),
		Log:        log,
	}
	if err := os.MkdirAll(w.Tools, 0o750); err != nil {
		return nil, fmt.Errorf("corpus: %w", err)
	}
	switch _, missing := os.Stat(w.Root); {
	case errors.Is(missing, fs.ErrNotExist):
		if err := w.clone(ctx); err != nil {
			return nil, err
		}
	case missing != nil:
		return nil, fmt.Errorf("corpus: %w", missing)
	case !w.marked():
		return nil, fmt.Errorf("%w: %s", ErrForeign, w.Root)
	}

	head, err := w.git(ctx, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(head)) != r.Commit {
		if err := w.checkout(ctx); err != nil {
			return nil, err
		}
	}
	if err := w.Reset(ctx, nil); err != nil {
		return nil, err
	}
	return w, nil
}

// Prepare runs the prepare steps of the repository, once for each commit and
// set of steps. It records the untracked files that the steps leave, which
// Reset keeps. Prepare does nothing for a read-only repository.
func (w *Workspace) Prepare(ctx context.Context) error {
	if !w.Repository.Writable() {
		return nil
	}
	digest := w.digest()
	if ran, _ := w.preparedBy(); ran == digest {
		return nil
	}
	for _, step := range w.Repository.Prepare {
		if _, err := w.run(ctx, w.expandAll(step)...); err != nil {
			return err
		}
	}
	untracked, err := w.Untracked(ctx)
	if err != nil {
		return err
	}
	record := append([]string{digest}, slices.Sorted(maps.Keys(untracked))...)
	if err := os.WriteFile(w.gitFile(prepared), []byte(strings.Join(record, "\n")+"\n"), 0o600); err != nil {
		return fmt.Errorf("corpus: %w", err)
	}
	return nil
}

// Build runs the build command of the repository and returns its output.
func (w *Workspace) Build(ctx context.Context) ([]byte, error) {
	return w.run(ctx, w.expandAll(w.Repository.Build)...)
}

// Ignored reports whether an ignore rule of the repository covers the file at
// p, a path relative to the root. techne refuses to read such a file, and
// follows the rules of git.
func (w *Workspace) Ignored(ctx context.Context, p string) (bool, error) {
	_, err := w.git(ctx, "check-ignore", "--quiet", "--", p)
	var exit *exec.ExitError
	switch {
	case err == nil:
		return true, nil
	case errors.As(err, &exit) && exit.ExitCode() == notIgnored:
		return false, nil
	}
	return false, err
}

// notIgnored is the exit status of git check-ignore for a path that no ignore
// rule covers.
const notIgnored = 1

// Untracked returns the untracked files of the repository that no ignore
// rule covers.
func (w *Workspace) Untracked(ctx context.Context) (map[string]bool, error) {
	return w.listed(ctx, "--others", "--exclude-standard")
}

// IgnoredFiles returns the untracked files of the repository that an ignore
// rule covers, such as the output of a build.
func (w *Workspace) IgnoredFiles(ctx context.Context) (map[string]bool, error) {
	return w.listed(ctx, "--others", "--ignored", "--exclude-standard")
}

// Sweep removes each file that an ignore rule covers, that before does not
// list and that ends in one of extensions: a source that the build of a
// change generated, whose errors the language server of the next change
// would report. It keeps every other file, such as the rest of the output of
// a build and the files that a language server writes into the clone.
func (w *Workspace) Sweep(ctx context.Context, before map[string]bool, extensions []string) error {
	if !w.Repository.Writable() {
		return ErrReadOnly
	}
	ignored, err := w.IgnoredFiles(ctx)
	if err != nil {
		return err
	}
	for _, p := range slices.Sorted(maps.Keys(ignored)) {
		if before[p] || !slices.Contains(extensions, filepath.Ext(p)) {
			continue
		}
		if err := os.Remove(filepath.Join(w.Root, p)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("corpus: %w", err)
		}
	}
	return nil
}

// Reset restores the tracked files of the clone to its commit, and removes
// each untracked file that neither keep nor the prepare steps list.
func (w *Workspace) Reset(ctx context.Context, keep map[string]bool) error {
	if !w.Repository.Writable() {
		return ErrReadOnly
	}
	if _, err := w.git(ctx, "reset", "--quiet", "--hard", w.Repository.Commit); err != nil {
		return err
	}
	strays, err := w.strays(ctx, keep)
	if err != nil {
		return err
	}
	for _, p := range strays {
		if err := os.Remove(filepath.Join(w.Root, p)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("corpus: %w", err)
		}
	}
	return nil
}

// Dirty returns the files of the clone that differ from its commit, in path
// order: each tracked file that differs from the commit, and each untracked
// file that neither keep nor the prepare steps list. It leaves out the files
// that an ignore rule covers, which Reset keeps.
func (w *Workspace) Dirty(ctx context.Context, keep map[string]bool) ([]string, error) {
	if !w.Repository.Writable() {
		return nil, ErrReadOnly
	}
	changed, err := w.git(ctx, "diff", "--name-only", "-z", w.Repository.Commit)
	if err != nil {
		return nil, err
	}
	strays, err := w.strays(ctx, keep)
	if err != nil {
		return nil, err
	}
	return slices.Sorted(slices.Values(append(separated(changed), strays...))), nil
}

// Files returns the tracked files of the repository that end in one of
// extensions, outside the prefixes that the manifest excludes, in path
// order.
func (w *Workspace) Files(ctx context.Context, extensions []string) ([]string, error) {
	listed, err := w.git(ctx, "ls-files", "-z")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, p := range separated(listed) {
		excluded := slices.ContainsFunc(w.Repository.Exclude, func(prefix string) bool {
			return strings.HasPrefix(p, prefix)
		})
		if !excluded && slices.Contains(extensions, filepath.Ext(p)) {
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return out, nil
}

// separated returns the paths of the output of a git command run with -z,
// which ends each path with a NUL byte.
func separated(listed []byte) []string {
	var out []string
	for p := range strings.SplitSeq(string(listed), "\x00") {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Environ returns the environment of the commands and of the techne process:
// the environment of the test with the variables of the repository set over
// it.
func (w *Workspace) Environ() []string {
	env := os.Environ()
	for _, name := range slices.Sorted(maps.Keys(w.Repository.Env)) {
		env = append(env, name+"="+w.expand(w.Repository.Env[name]))
	}
	return env
}

// clone makes and marks an empty repository at the root. It then checks out
// the commit of the repository.
func (w *Workspace) clone(ctx context.Context) error {
	if _, err := w.command(ctx, filepath.Dir(w.Root), w.Log, "git", "init", "--quiet", w.Root); err != nil {
		return err
	}
	if _, err := w.git(ctx, "remote", "add", "origin", w.Repository.URL); err != nil {
		return err
	}
	if err := os.WriteFile(w.gitFile(marked), []byte(w.Repository.URL+"\n"), 0o600); err != nil {
		return fmt.Errorf("corpus: %w", err)
	}
	return w.checkout(ctx)
}

// checkout fetches the commit of the repository alone, without its history,
// and checks it out.
func (w *Workspace) checkout(ctx context.Context) error {
	if _, err := w.git(ctx, "fetch", "--quiet", "--depth", "1", "origin", w.Repository.Commit); err != nil {
		return err
	}
	_, err := w.git(ctx, "checkout", "--quiet", "--force", "--detach", w.Repository.Commit)
	return err
}

// marked reports whether a run cloned the root.
func (w *Workspace) marked() bool {
	_, err := os.Stat(w.gitFile(marked))
	return err == nil
}

// digest returns a digest of the commit and the prepare steps.
func (w *Workspace) digest() string {
	encoded, _ := json.Marshal([]any{w.Repository.Commit, w.Repository.Prepare})
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

// preparedBy returns the digest of the prepare steps that ran and the
// untracked files that they left.
func (w *Workspace) preparedBy() (string, map[string]bool) {
	content, err := os.ReadFile(w.gitFile(prepared))
	if err != nil {
		return "", nil
	}
	lines := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
	left := map[string]bool{}
	for _, p := range lines[1:] {
		left[p] = true
	}
	return lines[0], left
}

// listed returns the paths that git ls-files lists with args.
func (w *Workspace) listed(ctx context.Context, args ...string) (map[string]bool, error) {
	listed, err := w.git(ctx, append([]string{"ls-files", "-z"}, args...)...)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, p := range separated(listed) {
		out[p] = true
	}
	return out, nil
}

// strays returns the untracked files that neither keep nor the prepare steps
// list, in path order.
func (w *Workspace) strays(ctx context.Context, keep map[string]bool) ([]string, error) {
	untracked, err := w.Untracked(ctx)
	if err != nil {
		return nil, err
	}
	_, left := w.preparedBy()
	var out []string
	for p := range untracked {
		if !keep[p] && !left[p] {
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return out, nil
}

// expand replaces the placeholders of the manifest in s.
func (w *Workspace) expand(s string) string {
	return strings.NewReplacer(
		"{tools}", w.Tools,
		"{clone}", w.Root,
		"{PATH}", os.Getenv("PATH"),
		"{HOME}", os.Getenv("HOME"),
	).Replace(s)
}

// expandAll replaces the placeholders of the manifest in each argument.
func (w *Workspace) expandAll(argv []string) []string {
	out := make([]string, len(argv))
	for i, arg := range argv {
		out[i] = w.expand(arg)
	}
	return out
}

// gitFile returns the path of a file in the git directory of the clone.
func (w *Workspace) gitFile(name string) string {
	return filepath.Join(w.Root, ".git", name)
}

// git runs git in the root of the repository, and leaves its output out of
// the log.
func (w *Workspace) git(ctx context.Context, args ...string) ([]byte, error) {
	return w.command(ctx, w.Root, io.Discard, append([]string{"git"}, args...)...)
}

// run runs argv in the root of the repository, and writes its output to the
// log.
func (w *Workspace) run(ctx context.Context, argv ...string) ([]byte, error) {
	return w.command(ctx, w.Root, w.Log, argv...)
}

// command runs argv in dir with the environment of the repository and
// returns its output. It writes the command to the log and the output to
// echo. The error of a failed command ends with the last lines of the
// output.
func (w *Workspace) command(ctx context.Context, dir string, echo io.Writer, argv ...string) ([]byte, error) {
	fmt.Fprintf(w.Log, "$ %s\n", strings.Join(argv, " "))
	// The manifest names the commands of each repository.
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // the manifest's command
	cmd.Dir = dir
	cmd.Env = w.Environ()
	var out bytes.Buffer
	cmd.Stdout = io.MultiWriter(&out, echo)
	cmd.Stderr = cmd.Stdout
	if err := cmd.Run(); err != nil {
		return out.Bytes(), fmt.Errorf("corpus: %s: %w\n%s", strings.Join(argv, " "), err, Tail(out.Bytes(), 20))
	}
	return out.Bytes(), nil
}

// Tail returns the last n lines of output.
func Tail(output []byte, n int) string {
	lines := strings.Split(strings.TrimRight(string(output), "\n"), "\n")
	return strings.Join(lines[max(len(lines)-n, 0):], "\n")
}
