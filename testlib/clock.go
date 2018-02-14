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

package testlib

import (
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/ubbagent/clock"
)

// MockClock is an extension of Clock that adds the ability to set the current time. Now returns
// the value passed to SetNow until a new value is set.
//
// Timers created by a MockClock will fire once the clock's time is set to or after the calculated
// fire time. This helps enable deterministic tests involving timers. However, because a MockClock's
// time doesn't continuously increase, a couple of considerations should be followed to avoid racy
// conditions in tests.
//
// 1. Avoid calls to Clock.Now() outside of the test thread. For example, when creating timers
//    asynchronously, avoid calls to clock.Now() and instead get the base time from the test thread.
//    e.g.,
//
//      func NewComponent(c clock.Clock) *Component {
//        c = &Component{clock: c}
//        go c.run(c.Now()) // Retrieve the initial time from this thread.
//      }
//
//      func (c *Component) run(now time.Time) {
//        fireAt := now.Add(someDelay)
//        for {
//          tmr := c.clock.NewTimerAt(fireAt)
//          select {
//          case <-c.someChan:
//            c.handleEvent()
//
//          case n := <-tmr.GetC():
//            c.somePeriodicOperation()
//            // Compute next fire time based on value received from timer.
//            fireAt = n.Add(someDelay)
//        }
//        tmr.Cancel()
//      }
//
// 2. When creating a new timer, use Clock.NewTimerAt(time.Time) to specify a point in time at which
//    the timer should fire, and calculate that time based on a known base time (#1). Using
//    Clock.NewTimer(time.Duration) can lead to race conditions in tests:
//
//      d := someDelay - c.clock.Now().Sub(c.lastFireTime)
//      // <-- if the MockClock's time is advanced here, the new timer may not fire when expected.
//      tmr := c.clock.NewTimer(d)
//
//    Instead:
//
//      now := c.clock.Now()
//      nextFire := now.Add(someDelay - now.Sub(c.lastFireTime))
//      tmr := c.clock.NewTimerAt(nextFire)
type MockClock interface {
	clock.Clock
	SetNow(time.Time)

	// GetNextFireTime returns the time that the next Timer will fire, or the zero value if no timers
	// are set.
	GetNextFireTime() time.Time
}

// NewMockClock creates a new MockClock instance that initially returns time zero.
func NewMockClock() MockClock {
	return &mockClock{
		timers: make(map[*mockTimer]bool),
	}
}

type mockClock struct {
	mutex  sync.Mutex
	now    time.Time
	timers map[*mockTimer]bool
}

func (mc *mockClock) Now() time.Time {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()
	return mc.now
}

func (mc *mockClock) SetNow(now time.Time) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()
	mc.now = now
	for mt := range mc.timers {
		// this call might result in the timer being removed from the set.
		mt.maybeFire(now)
	}
}

func (mc *mockClock) GetNextFireTime() time.Time {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()
	var earliest time.Time
	for mt := range mc.timers {
		if !mt.done && (earliest.IsZero() || mt.fireAt.Before(earliest)) {
			earliest = mt.fireAt
		}
	}
	return earliest
}

func (mc *mockClock) NewTimer(d time.Duration) clock.Timer {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()
	at := mc.now.Add(d)
	return mc.newTimer(at)
}

func (mc *mockClock) NewTimerAt(at time.Time) clock.Timer {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()
	return mc.newTimer(at)
}

// Assumes mc.mutex is held.
func (mc *mockClock) newTimer(at time.Time) clock.Timer {
	c := make(chan time.Time, 1)
	mt := &mockTimer{
		c:      c,
		owner:  mc,
		fireAt: at,
	}
	mc.timers[mt] = true

	// Call maybeFire to handle cases where the given duration is 0 or negative.
	mt.maybeFire(mc.now)
	return mt
}

type mockTimer struct {
	c      chan time.Time
	num    int
	owner  *mockClock
	fireAt time.Time
	done   bool
}

func (mt *mockTimer) GetC() <-chan time.Time {
	return mt.c
}

func (mt *mockTimer) Stop() bool {
	mt.owner.mutex.Lock()
	defer mt.owner.mutex.Unlock()
	if mt.done {
		return false
	}
	mt.done = true
	mt.remove()
	return true
}

// maybeFire fires a timer event into the channel if appropriate mock time has elapsed and the timer
// hasn't already fired or been stopped. Assumes that mt.owner.mutex is held.
func (mt *mockTimer) maybeFire(t time.Time) {
	if mt.done || mt.fireAt.After(t) {
		return
	}
	mt.c <- t
	mt.done = true
	mt.remove()
}

// remove removes this mockTimer from the owner mockClock. Assumes that mt.owner.mutex is held.
func (mt *mockTimer) remove() {
	delete(mt.owner.timers, mt)
}
