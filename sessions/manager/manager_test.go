package manager

import (
	"testing"
	"time"
)

func TestGatewayStartInterval(t *testing.T) {
	for _, tc := range []struct {
		concurrency uint32
		want        time.Duration
	}{
		{1, concurrencyTimeWindowSec * time.Second},
		{5, time.Second},
	} {
		if got := CalcInterval(tc.concurrency); got != tc.want {
			t.Errorf("concurrency=%d interval=%v, want %v", tc.concurrency, got, tc.want)
		}
	}
}
