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

package clock

import (
	"sync"
	"time"
)

// Clock is a simple interface that returns a "current" timestamp. This will generally be the
// current time, but the Clock interface can be mocked during testing to make testing time-sensitive
// components deterministic.
type Clock interface {
	// Now returns the current time, as defined by this Clock.
	Now() time.Time

	// NewTimer creates a new Timer associated with this Clock.
	NewTimer(d time.Duration) Timer

	// NewTimerAt creates a new Timer that fires at or after the given time.
	NewTimerAt(at time.Time) Timer
}

// MockClock is an extension of Clock that adds the ability to set the current time. Now returns
// the value passed to SetNow until a new value is set.
// TODO(volkman): move MockClock to its own file.
type MockClock interface {
	Clock
	SetNow(time.Time)

	// GetNextFireTime returns the time that the next Timer will fire, or the zero value if no timers
	// are set.
	GetNextFireTime() time.Time
}

// Timer mimics a time.Timer, providing a channel that delivers a signal after a certain amount of
// time has elapsed. When associated with a MockClock, Timer delivers its signal when the
// MockClock's time is programmatically set to a certain point.
type Timer interface {

	// GetC returns this Timer's signal channel. For real clocks, this simply returns a time.Timer.C.
	GetC() <-chan time.Time

	// Stop stops the timer. Like time.Timer.Stop(), it returns true if the call stops the timer,
	// false if the timer has already expired or been stopped. See documentation for time.Timer.Stop()
	// for more information about this method's behavior.
	Stop() bool
}

// NewRealClock creates a new Clock instance that returns the current time.
func NewRealClock() Clock {
	return &realClock{}
}

// NewMockClock creates a new MockClock instance that initially returns time zero.
func NewMockClock() MockClock {
	return &mockClock{
		timers: make(map[*mockTimer]bool),
	}
}

// NewStoppedTimer creates a Timer that will never fire.
func NewStoppedTimer() Timer {
	return &stoppedTimer{c: make(chan time.Time)}
}

type stoppedTimer struct {
	c chan time.Time
}

func (st *stoppedTimer) GetC() <-chan time.Time {
	return st.c
}

func (t *stoppedTimer) Stop() bool {
	return false
}

type realClock struct{}

func (rc *realClock) Now() time.Time {
	return time.Now()
}

func (rc *realClock) NewTimer(d time.Duration) Timer {
	return &realTimer{t: time.NewTimer(d)}
}

func (rc *realClock) NewTimerAt(at time.Time) Timer {
	duration := at.Sub(time.Now())
	return &realTimer{t: time.NewTimer(duration)}
}

type realTimer struct {
	t *time.Timer
}

func (t *realTimer) GetC() <-chan time.Time {
	return t.t.C
}

func (t *realTimer) Stop() bool {
	return t.t.Stop()
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

func (mc *mockClock) NewTimer(d time.Duration) Timer {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()
	at := mc.now.Add(d)
	return mc.newTimer(at)
}

func (mc *mockClock) NewTimerAt(at time.Time) Timer {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()
	return mc.newTimer(at)
}

// Assumes mc.mutex is held.
func (mc *mockClock) newTimer(at time.Time) Timer {
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
