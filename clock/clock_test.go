package clock_test

import (
	"testing"
	"time"
	"ubbagent/clock"
)

func TestMockClock(t *testing.T) {
	mc := clock.NewMockClock()
	if ok := mc.Now().IsZero(); !ok {
		t.Fatal("Expected zero time")
	}

	mc.SetNow(time.Unix(1234, 0))
	if ok := mc.Now().Unix() == 1234; !ok {
		t.Fatal("Expected mock time to be 1234")
	}
}

func TestMockTimer(t *testing.T) {
	mc := clock.NewMockClock()
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
}
