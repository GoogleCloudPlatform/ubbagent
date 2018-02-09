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

// NewClock creates a new Clock instance that returns the current time.
func NewClock() Clock {
	return &realClock{}
}

// NewStoppedTimer creates a Timer that will never fire.
func NewStoppedTimer() Timer {
	return &stoppedTimer{c: make(chan time.Time)}
}

type stoppedTimer struct {
	c chan time.Time
}

func (t *stoppedTimer) GetC() <-chan time.Time {
	return t.c
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
