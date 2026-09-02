// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package change

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"

	"go.dokimi.dev/techne/core/edit"
)

// held keeps the plans previews computed, so applying one costs no
// second planning run.
//
// A handle rather than the plan itself, because the plan goes to an
// agent and comes back: a rename over thirty sites is several kilobytes
// that a model would have to reproduce exactly, and reproducing bytes
// exactly is the least reliable thing a model does. What crosses the
// wire is sixteen bytes of hex.
//
// The cost is that a plan does not outlive the process. A caller whose
// handle is gone previews again, which is the call it just made.
type held struct {
	mu    sync.Mutex
	plans map[string]edit.Plan
	// order is the handles oldest first, so the bound drops the plan
	// least likely to still be wanted.
	order []string
}

func newHeld() *held {
	return &held{plans: map[string]edit.Plan{}}
}

// keep stores a plan and returns the handle it is fetched by.
func (h *held) keep(plan edit.Plan) (string, error) {
	handle, err := handle()
	if err != nil {
		return "", err
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.plans[handle] = plan
	h.order = append(h.order, handle)
	for len(h.order) > heldLimit {
		delete(h.plans, h.order[0])
		h.order = h.order[1:]
	}
	return handle, nil
}

// take fetches a plan and forgets it.
//
// Once, because applying is not something to do twice by accident: a
// handle replayed after the files moved on would be admitted against
// preconditions that no longer describe the workspace, and refused, but
// a handle replayed immediately would apply the same change again.
func (h *held) take(handle string) (edit.Plan, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	plan, there := h.plans[handle]
	if !there {
		return edit.Plan{}, false
	}
	delete(h.plans, handle)
	for i, one := range h.order {
		if one == handle {
			h.order = append(h.order[:i], h.order[i+1:]...)
			break
		}
	}
	return plan, true
}

// handle returns a name no caller could have guessed.
//
// Unguessable rather than sequential: a handle is the authority to write
// a change somebody else previewed, and a counter would let a caller
// apply a plan it never saw.
func handle() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("change: name a plan: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

// heldLimit is how many previews are kept. A session previews far more
// often than it applies, and a plan nobody came back for is one the
// caller changed its mind about.
const heldLimit = 32
