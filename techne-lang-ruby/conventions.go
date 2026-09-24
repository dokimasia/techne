// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package ruby

import (
	"path"
	"strings"
)

// IsTest reports whether p is a test file by the names of Minitest and
// RSpec: a base name that ends in _test.rb or _spec.rb, and a file inside a
// directory named spec.
func IsTest(p string) bool {
	base := path.Base(p)
	return strings.HasSuffix(base, "_test.rb") ||
		strings.HasSuffix(base, "_spec.rb") ||
		strings.HasPrefix(p, "spec/") ||
		strings.Contains(p, "/spec/")
}
