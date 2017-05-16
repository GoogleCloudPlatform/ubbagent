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
