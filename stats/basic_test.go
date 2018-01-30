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
	"fmt"
	"github.com/GoogleCloudPlatform/ubbagent/clock"
	"testing"
	"time"
)

func TestSimple(t *testing.T) {
	mc := clock.NewMockClock()
	s := newBasic(mc)

	mc.SetNow(time.Unix(1000, 0))

	s.Register("report1", []string{"handler1", "handler2"})
	s.SendSucceeded("report1", "handler1")
	s.SendSucceeded("report1", "handler2")

	snap := s.Snapshot()
	if want, got := 0, snap.CurrentFailureCount; want != got {
		t.Fatalf("snap.CurrentFailureCount: want=%v, got=%v", want, got)
	}
	if want, got := 0, snap.TotalFailureCount; want != got {
		t.Fatalf("snap.TotalFailureCount: want=%v, got=%v", want, got)
	}
	if want, got := time.Unix(1000, 0), snap.LastReportSuccess; want != got {
		t.Fatalf("snap.LastReportSuccess: want=%v, got=%v", want, got)
	}

	mc.SetNow(time.Unix(1100, 0))

	s.Register("report2", []string{"handler1", "handler2", "handler3"})
	s.SendSucceeded("report2", "handler1")

	// There's still one handler remaining, so the stats should not have updated yet.
	snap = s.Snapshot()
	if want, got := 0, snap.CurrentFailureCount; want != got {
		t.Fatalf("snap.CurrentFailureCount: want=%v, got=%v", want, got)
	}
	if want, got := 0, snap.TotalFailureCount; want != got {
		t.Fatalf("snap.TotalFailureCount: want=%v, got=%v", want, got)
	}
	if want, got := time.Unix(1000, 0), snap.LastReportSuccess; want != got {
		t.Fatalf("snap.LastReportSuccess: want=%v, got=%v", want, got)
	}

	s.SendFailed("report2", "handler2")

	// Check that the failure counts have increased.
	snap = s.Snapshot()
	if want, got := 1, snap.CurrentFailureCount; want != got {
		t.Fatalf("snap.CurrentFailureCount: want=%v, got=%v", want, got)
	}
	if want, got := 1, snap.TotalFailureCount; want != got {
		t.Fatalf("snap.TotalFailureCount: want=%v, got=%v", want, got)
	}
	if want, got := time.Unix(1000, 0), snap.LastReportSuccess; want != got {
		t.Fatalf("snap.LastReportSuccess: want=%v, got=%v", want, got)
	}

	// Multiple failures for the same send should only increment failure counts once.
	s.SendFailed("report2", "handler3")
	snap = s.Snapshot()
	if want, got := 1, snap.CurrentFailureCount; want != got {
		t.Fatalf("snap.CurrentFailureCount: want=%v, got=%v", want, got)
	}
	if want, got := 1, snap.TotalFailureCount; want != got {
		t.Fatalf("snap.TotalFailureCount: want=%v, got=%v", want, got)
	}

	s.Register("report3", []string{"handler1", "handler2"})
	s.SendSucceeded("report3", "handler1")
	s.SendSucceeded("report3", "handler2")

	// LastReportSuccess should move forward, and currentFailureCount should be reset to 0.
	snap = s.Snapshot()
	if want, got := 0, snap.CurrentFailureCount; want != got {
		t.Fatalf("snap.CurrentFailureCount: want=%v, got=%v", want, got)
	}
	if want, got := 1, snap.TotalFailureCount; want != got {
		t.Fatalf("snap.TotalFailureCount: want=%v, got=%v", want, got)
	}
	if want, got := time.Unix(1100, 0), snap.LastReportSuccess; want != got {
		t.Fatalf("snap.LastReportSuccess: want=%v, got=%v", want, got)
	}

	// Test that the pending set gets trimmed to MAX_PENDING.
	for i := 0; i < *maxPendingSends+10; i++ {
		s.Register(fmt.Sprintf("report%v", i), []string{"handler1", "handler2"})
		s.SendSucceeded("report3", "handler1")
	}

	if len(s.pending) > *maxPendingSends {
		t.Fatalf("Pending set length should have been trimmed to %v, but was %v", *maxPendingSends, len(s.pending))
	}
}
