// Copyright 2018 Google LLC
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

package testlib_test

import (
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/ubbagent/testlib"
)

func TestMockClock(t *testing.T) {
	mc := testlib.NewMockClock()
	if ok := mc.Now().IsZero(); !ok {
		t.Fatal("Expected zero time")
	}

	mc.SetNow(time.Unix(1234, 0))
	if ok := mc.Now().Unix() == 1234; !ok {
		t.Fatal("Expected mock time to be 1234")
	}
}

func TestMockTimer(t *testing.T) {
	mc := testlib.NewMockClock()
	mc.SetNow(time.Unix(10, 0))
	mt := mc.NewTimer(10 * time.Second)

	// Ensure timer doesn't fire before its time
	mc.SetNow(time.Unix(15, 0))
	select {
	case <-mt.GetC():
		t.Fatal("Timer should not have fired yet")
	default:
	}

	// Ensure timer fires when the clock hits the right time.
	mc.SetNow(time.Unix(20, 0))
	select {
	case firedAt := <-mt.GetC():
		if !firedAt.Equal(time.Unix(20, 0)) {
			t.Fatalf("Fired-at time unexpected: %+v", firedAt)
		}
	default:
		t.Fatal("Timer should have fired")
	}

	// Ensure timer does not fire again.
	mc.SetNow(time.Unix(21, 0))
	select {
	case <-mt.GetC():
		t.Fatal("Timer should not have fired again")
	default:
	}

	// Ensure stopping the timer indicates it already fired
	if mt.Stop() {
		t.Fatal("Fired timer.Stop() should return false")
	}

	// Ensure a stopped timer does not fire
	mt2 := mc.NewTimer(10 * time.Second)
	mc.SetNow(time.Unix(30, 0))
	if !mt2.Stop() {
		t.Fatal("Non-fired timer.Stop() should return true")
	}
	mc.SetNow(time.Unix(100, 0))
	select {
	case <-mt2.GetC():
		t.Fatal("Stopped timer should not have fired")
	default:
	}

	// Ensure timers with a duration <= 0 fire immediately.
	mt3 := mc.NewTimer(0)
	select {
	case <-mt3.GetC():
	default:
		t.Fatal("Timer should have fired")
	}

	mt4 := mc.NewTimer(-1)
	select {
	case <-mt4.GetC():
	default:
		t.Fatal("Timer should have fired")
	}

	// test NewTimerAt
	mt5 := mc.NewTimerAt(time.Unix(200, 0))
	mc.SetNow(time.Unix(201, 0))
	select {
	case <-mt5.GetC():
	default:
		t.Fatal("Timer should have fired")
	}

	// test NewTimerAt with time in the past
	mt6 := mc.NewTimerAt(time.Unix(0, 0))
	select {
	case <-mt6.GetC():
	default:
		t.Fatal("Timer should have fired")
	}
}
