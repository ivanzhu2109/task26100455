package pacing

import "testing"

func TestCumulativeTargetEven(t *testing.T) {
	// Budget 100 over 4 days: 25 per day, exact.
	want := []int64{0, 25, 50, 75, 100}
	for tt := 0; tt <= 4; tt++ {
		if got := cumulativeTarget(100, 4, StrategyEven, tt); got != want[tt] {
			t.Fatalf("even t=%d: got %d want %d", tt, got, want[tt])
		}
	}
}

func TestCumulativeTargetFloorsExactRatio(t *testing.T) {
	// Budget 100 over 3 days: exact ratios 100/3, 200/3, 100 floored.
	want := []int64{0, 33, 66, 100}
	for tt := 0; tt <= 3; tt++ {
		if got := cumulativeTarget(100, 3, StrategyEven, tt); got != want[tt] {
			t.Fatalf("even t=%d: got %d want %d", tt, got, want[tt])
		}
	}
}

func TestCumulativeTargetFrontLoaded(t *testing.T) {
	// Budget 1000 over 4 days: 60% (600) over first 2 days, 40% after.
	want := []int64{0, 300, 600, 800, 1000}
	for tt := 0; tt <= 4; tt++ {
		if got := cumulativeTarget(1000, 4, StrategyFrontLoaded, tt); got != want[tt] {
			t.Fatalf("front t=%d: got %d want %d", tt, got, want[tt])
		}
	}
}

func TestCumulativeTargetBackLoaded(t *testing.T) {
	// Budget 1000 over 4 days: 40% (400) over first 2 days, 60% after.
	want := []int64{0, 200, 400, 700, 1000}
	for tt := 0; tt <= 4; tt++ {
		if got := cumulativeTarget(1000, 4, StrategyBackLoaded, tt); got != want[tt] {
			t.Fatalf("back t=%d: got %d want %d", tt, got, want[tt])
		}
	}
}

func TestCumulativeTargetOddDaysMiddleSplit(t *testing.T) {
	// Budget 500 over 5 days. Each half is 2.5 days; the middle day
	// (day 3) belongs half to each segment.
	for tt := 0; tt <= 5; tt++ {
		got := cumulativeTarget(500, 5, StrategyFrontLoaded, tt)
		var want int64
		switch {
		case 2*tt <= 5:
			want = 500 * 6 * int64(tt) / 25
		default:
			want = 500 * (5 + 4*int64(tt)) / 25
		}
		if got != want {
			t.Fatalf("front odd t=%d: got %d want %d", tt, got, want)
		}
	}
	// The middle day must gain exactly one day's uniform share: the
	// increment from day 2 to day 3 equals budget/days = 100.
	d2 := cumulativeTarget(500, 5, StrategyFrontLoaded, 2)
	d3 := cumulativeTarget(500, 5, StrategyFrontLoaded, 3)
	if d3-d2 != 100 {
		t.Fatalf("middle day increment: got %d want 100", d3-d2)
	}
	b2 := cumulativeTarget(500, 5, StrategyBackLoaded, 2)
	b3 := cumulativeTarget(500, 5, StrategyBackLoaded, 3)
	if b3-b2 != 100 {
		t.Fatalf("back middle day increment: got %d want 100", b3-b2)
	}
	// Front loaded builds 60/day over the first 2.5 days.
	if got := cumulativeTarget(500, 5, StrategyFrontLoaded, 2); got != 240 {
		t.Fatalf("front day2: got %d want 240", got)
	}
	if got := cumulativeTarget(500, 5, StrategyBackLoaded, 3); got != 260 {
		t.Fatalf("back day3: got %d want 260", got)
	}
}

func TestCumulativeTargetEndpoints(t *testing.T) {
	for _, s := range []Strategy{StrategyEven, StrategyFrontLoaded, StrategyBackLoaded} {
		if got := cumulativeTarget(1_000_000_000, 90, s, 0); got != 0 {
			t.Fatalf("strategy %d t=0: got %d want 0", s, got)
		}
		if got := cumulativeTarget(1_000_000_000, 90, s, 90); got != 1_000_000_000 {
			t.Fatalf("strategy %d t=90: got %d want full budget", s, got)
		}
	}
}
