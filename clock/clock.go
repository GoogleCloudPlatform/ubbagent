package clock

import "time"

// Clock is a simple interface that returns a "current" timestamp. This will generally be the
// current time, but the Clock interface can be mocked during testing to make testing time-sensitive
// components deterministic.
type Clock interface {
	Now() time.Time
}

// MockClock is an extension of Clock that adds the ability to set the current time. Now returns
// the value passed to SetNow until a new value is set.
// TODO(volkman): move MockClock to its own file.
type MockClock interface {
	Clock
	SetNow(time.Time)
}

// NewRealClock creates a new Clock instance that returns the current time.
func NewRealClock() Clock {
	return &realClock{}
}

// NewMockClock creates a new MockClock instance that initially returns time zero.
func NewMockClock() MockClock {
	return &mockClock{}
}

type realClock struct{}

func (rc *realClock) Now() time.Time {
	return time.Now()
}

type mockClock struct {
	now time.Time
}

func (mc *mockClock) Now() time.Time {
	return mc.now
}

func (mc *mockClock) SetNow(now time.Time) {
	mc.now = now
}
