// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package change

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"slices"
	"sync"

	"go.dokimi.dev/techne/core/edit"
)

// held keeps the plans of previews under their handles, so a commit does not plan again. It
// keeps the latest heldLimit plans in memory, and a caller whose handle is gone previews
// again. It is safe for concurrent use.
type held struct {
	mu    sync.Mutex
	plans map[string]kept
	// order are the handles of plans, oldest first.
	order []string
}

// kept is the plan of a preview with the request of the preview. A commit checks the plan
// through the language of the request, which the preview asked.
type kept struct {
	request edit.Request
	plan    edit.Plan
}

// newHeld returns an empty held.
func newHeld() *held {
	return &held{plans: map[string]kept{}}
}

// keep stores the plan of req and returns its handle. It drops the oldest plan beyond
// heldLimit.
func (h *held) keep(req edit.Request, plan edit.Plan) (string, error) {
	handle, err := handle()
	if err != nil {
		return "", err
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.plans[handle] = kept{request: req, plan: plan}
	h.order = append(h.order, handle)
	for len(h.order) > heldLimit {
		delete(h.plans, h.order[0])
		h.order = h.order[1:]
	}
	return handle, nil
}

// take returns the plan under handle and removes it, so a handle applies once. It reports
// whether a plan was under handle.
func (h *held) take(handle string) (kept, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	one, found := h.plans[handle]
	if !found {
		return kept{}, false
	}
	delete(h.plans, handle)
	h.order = slices.DeleteFunc(h.order, func(other string) bool { return other == handle })
	return one, true
}

// handle returns 16 random bytes as 32 hexadecimal characters. A handle authorises the
// write of the plan of a preview, so no handle can be derived from another.
func handle() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("change: name a plan: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

// heldLimit is the number of plans that the service keeps.
const heldLimit = 32
