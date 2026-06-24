// Package proxy implements outbound proxy selection for providers.
//
// A provider may have multiple proxies attached (many-to-many). Selection of
// which proxy to use for a given outbound call follows the provider's
// ProxyStrategy: failover, round_robin, or random.
package proxy

import (
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
)

// ResolvedProxy is a candidate proxy for an outbound call.
type ResolvedProxy struct {
	ID       string
	URL      string // scheme://[user:pass@]host:port
	Priority int    // lower = higher priority (failover ordering)
}

// Selector picks a proxy from candidates and receives success/failure feedback
// so failover can advance past a broken proxy.
type Selector interface {
	// Pick returns the next proxy to try. ok=false when there are no candidates.
	Pick(candidates []ResolvedProxy) (ResolvedProxy, bool)
	// MarkResult reports whether the given proxy id succeeded or failed.
	// Only failover uses this to advance; round_robin/random ignore it.
	MarkResult(id string, success bool)
}

// NewSelector builds a Selector for a strategy. Unknown/empty → failover.
func NewSelector(strategy string) Selector {
	switch strategy {
	case "round_robin":
		return &roundRobinSelector{}
	case "random":
		return &randomSelector{}
	default:
		return &failoverSelector{failed: map[string]struct{}{}}
	}
}

// failoverSelector always prefers the highest-priority (lowest Priority value)
// proxy that hasn't recently failed. When all have failed it resets and retries
// the primary.
type failoverSelector struct {
	mu     sync.Mutex
	failed map[string]struct{}
}

func (f *failoverSelector) Pick(candidates []ResolvedProxy) (ResolvedProxy, bool) {
	if len(candidates) == 0 {
		return ResolvedProxy{}, false
	}
	sorted := append([]ResolvedProxy(nil), candidates...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Priority < sorted[j].Priority })

	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range sorted {
		if _, bad := f.failed[c.ID]; !bad {
			return c, true
		}
	}
	// All marked bad — reset and return the primary for a retry.
	f.failed = map[string]struct{}{}
	return sorted[0], true
}

func (f *failoverSelector) MarkResult(id string, success bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if success {
		delete(f.failed, id)
		return
	}
	f.failed[id] = struct{}{}
}

// roundRobinSelector cycles through candidates in order using a counter.
type roundRobinSelector struct {
	n atomic.Int64
}

func (r *roundRobinSelector) Pick(candidates []ResolvedProxy) (ResolvedProxy, bool) {
	if len(candidates) == 0 {
		return ResolvedProxy{}, false
	}
	idx := r.n.Add(1) - 1
	return candidates[int(idx%int64(len(candidates)))], true
}

func (r *roundRobinSelector) MarkResult(string, bool) {}

// randomSelector picks uniformly at random.
type randomSelector struct{}

func (s *randomSelector) Pick(candidates []ResolvedProxy) (ResolvedProxy, bool) {
	if len(candidates) == 0 {
		return ResolvedProxy{}, false
	}
	return candidates[rand.Intn(len(candidates))], true
}

func (s *randomSelector) MarkResult(string, bool) {}
