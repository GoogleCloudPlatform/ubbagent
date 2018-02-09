// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stats

import (
	"flag"
	"math"
	"sync"

	"github.com/GoogleCloudPlatform/ubbagent/clock"
	"github.com/golang/glog"
)

// The maximum number of pending sends to track before old items are dropped.
var maxPendingSends = flag.Int("max_pending_sends", 1000, "maximum number of pending sends that are tracked for agent stats")

// Basic is a stats.Recorder and stats.Provider that records and provides stats.Snapshot values.
// Storage is in-memory and all stats are reset when the agent is restarted.
type Basic struct {
	clock        clock.Clock
	mutex        sync.RWMutex
	pending      map[string]*pendingSend
	pendingCount int64
	current      Snapshot
}

func (s *Basic) Register(id string, handlers []string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.pendingCount++
	s.pending[id] = newPendingSend(handlers, s.pendingCount)

	// Trim the pending set if necessary
	if len(s.pending) > *maxPendingSends {
		oldestKey := ""
		var oldestOrder int64 = math.MaxInt64
		for k, v := range s.pending {
			if v.order < oldestOrder {
				oldestKey = k
				oldestOrder = v.order
			}
		}
		if oldestKey != "" {
			glog.Warningf("stats.Basic: too many pending sends; deleting send %v", oldestKey)
			delete(s.pending, oldestKey)
		}
	}
}

func (s *Basic) SendSucceeded(id string, handler string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	p, exists := s.pending[id]
	if !exists {
		// This might happen if the set of pending sends grows to large and older sends are dropped, or
		// if part of a send succeeded after the agent was restarted.
		glog.Warningf("stats.Basic: ignoring SendSucceeded from handler %v of unknown report id %v", handler, id)
		return
	}
	p.handlerSuccess(handler)
	if p.isSuccessful() {
		delete(s.pending, id)
		// Reset the "current" failure count: the number of failures since the last success
		s.current.CurrentFailureCount = 0
		// Set the last success time
		s.current.LastReportSuccess = s.clock.Now()
	}
}

func (s *Basic) SendFailed(id string, handler string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	// One or more failures means the full send failed. So we remove the pendingSend and increment
	// the failure count.
	if _, exists := s.pending[id]; exists {
		delete(s.pending, id)
		s.current.CurrentFailureCount++
		s.current.TotalFailureCount++
	} else {
		glog.Warningf("stats.Basic: ignoring SendFailed from handler %v of unknown report id %v", handler, id)
	}
}

func (s *Basic) Snapshot() Snapshot {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.current
}

func NewBasic() *Basic {
	return newBasic(clock.NewClock())
}

func newBasic(clock clock.Clock) *Basic {
	return &Basic{pending: make(map[string]*pendingSend), clock: clock}
}

type pendingSend struct {
	handlers map[string]bool
	order    int64
}

func (ps *pendingSend) handlerSuccess(handler string) {
	delete(ps.handlers, handler)
}

func (ps *pendingSend) isSuccessful() bool {
	return len(ps.handlers) == 0
}

func newPendingSend(handlers []string, order int64) *pendingSend {
	hm := make(map[string]bool)
	for _, h := range handlers {
		hm[h] = true
	}
	return &pendingSend{hm, order}
}
