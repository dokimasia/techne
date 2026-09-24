// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package corpus

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Manifest lists the repositories of the corpus and the settings of a run.
type Manifest struct {
	// Budget is the longest time that a warm call of a read tool may take.
	Budget Duration `json:"budget"`
	// Sample is the number of files that a run samples per repository. Half
	// of them are the largest files, and the rest are drawn with Seed.
	Sample int `json:"sample"`
	// Seed draws the same files on every run.
	Seed uint64 `json:"seed"`
	// Repositories are the repositories, in the order that a run drives them.
	Repositories []Repository `json:"repositories"`
}

// Repository is one repository of the corpus.
//
// The strings of Prepare, Build and Env can contain four placeholders:
// {tools} for the directory of the tools of the repository, {clone} for
// the directory of the repository, and {PATH} and {HOME} for the variables
// of the test.
type Repository struct {
	// Name names the clone and the subtest of the repository.
	Name string `json:"name"`
	// Language is the language whose tools a run drives, as techne names it.
	Language string `json:"language"`
	// URL is the repository to clone. A repository without a URL is a
	// directory on this machine, at Path, which a run only reads.
	URL string `json:"url,omitempty"`
	// Path is the directory of a repository without a URL, relative to the
	// root of this repository.
	Path string `json:"path,omitempty"`
	// Tag names Commit for a reader. A run checks out Commit.
	Tag    string `json:"tag,omitempty"`
	Commit string `json:"commit,omitempty"`
	// Prepare are the commands that install the dependencies of the clone.
	// A run executes them once, in the directory of the clone.
	Prepare [][]string `json:"prepare,omitempty"`
	// Build is the command that builds the clone. It succeeds at Commit and
	// after each change that a run applies.
	Build []string `json:"build,omitempty"`
	// Errors is a pattern whose group captures the number of errors in the
	// output of Build. When Errors is set, a run ignores the exit status of
	// Build, and a change must not report more errors than Commit reports.
	Errors string `json:"errors,omitempty"`
	// Env are the variables of the commands and of the techne process, set
	// over the environment of the test.
	Env map[string]string `json:"env,omitempty"`
	// Exclude are the path prefixes that a run leaves out of the sample.
	Exclude []string `json:"exclude,omitempty"`
	// Warmup is how long a run waits for the language server to settle.
	Warmup Duration `json:"warmup"`
}

// Writable reports whether a run can change the repository: a clone can be
// changed, and a directory that the manifest names by path cannot.
func (r Repository) Writable() bool { return r.URL != "" }

// Count returns the number of errors in output: the sum of the numbers that
// the group of the Errors pattern of r captures at each match, so a build
// that runs two test suites reports the failures of both. It returns false
// when r has no pattern or output does not match it.
func (r Repository) Count(output []byte) (int, bool) {
	if r.Errors == "" {
		return 0, false
	}
	matches := regexp.MustCompile(r.Errors).FindAllSubmatch(output, -1)
	total := 0
	for _, found := range matches {
		count, err := strconv.Atoi(string(found[1]))
		if err != nil {
			return 0, false
		}
		total += count
	}
	return total, len(matches) > 0
}

// Load reads the manifest at path and checks every repository:
//
//   - a name that no other repository has, and a language
//   - either a URL with a commit of 40 hexadecimal digits and a build
//     command, or a path
//   - an Errors pattern with exactly one group, when it is set
//   - a positive warmup
func Load(path string) (Manifest, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("corpus: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(content, &m); err != nil {
		return Manifest{}, fmt.Errorf("corpus: %s: %w", path, err)
	}
	if m.Budget <= 0 || m.Sample <= 0 {
		return Manifest{}, fmt.Errorf("corpus: %s: the budget and the sample must be positive", path)
	}
	var names []string
	for _, r := range m.Repositories {
		if err := r.check(names); err != nil {
			return Manifest{}, fmt.Errorf("corpus: %s: %w", path, err)
		}
		names = append(names, r.Name)
	}
	return m, nil
}

// commit matches a full commit hash.
var commit = regexp.MustCompile(`^[0-9a-f]{40}$`)

// check returns an error for a repository that Load refuses, given the
// names of the repositories before it.
func (r Repository) check(names []string) error {
	switch {
	case r.Name == "" || r.Language == "":
		return fmt.Errorf("a repository has no name or no language")
	case slices.Contains(names, r.Name):
		return fmt.Errorf("two repositories are named %s", r.Name)
	case (r.URL == "") == (r.Path == ""):
		return fmt.Errorf("%s has both a URL and a path, or neither", r.Name)
	case r.URL != "" && !commit.MatchString(r.Commit):
		return fmt.Errorf("%s is not pinned to a full commit", r.Name)
	case r.URL != "" && len(r.Build) == 0:
		return fmt.Errorf("%s has no build command", r.Name)
	case r.Warmup <= 0:
		return fmt.Errorf("%s has no warmup", r.Name)
	}
	if r.Errors != "" {
		pattern, err := regexp.Compile(r.Errors)
		if err != nil || pattern.NumSubexp() != 1 {
			return fmt.Errorf("%s: the errors pattern needs exactly one group", r.Name)
		}
	}
	return nil
}

// Select returns the repositories that only names, as a list of names and
// languages separated by commas, in manifest order. An empty only selects
// every repository.
func (m Manifest) Select(only string) []Repository {
	if strings.TrimSpace(only) == "" {
		return m.Repositories
	}
	wanted := strings.Split(only, ",")
	for i := range wanted {
		wanted[i] = strings.TrimSpace(wanted[i])
	}
	var out []Repository
	for _, r := range m.Repositories {
		if slices.Contains(wanted, r.Name) || slices.Contains(wanted, r.Language) {
			out = append(out, r)
		}
	}
	return out
}

// Duration is a time.Duration that JSON writes as a string, such as "2s".
type Duration time.Duration

// UnmarshalJSON reads a duration in the form of time.ParseDuration.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(parsed)
	return nil
}

// MarshalJSON writes the duration in the form of time.Duration.String.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}
