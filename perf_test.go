package pacing

import (
	"math/rand"
	"runtime"
	"testing"
	"time"
)

// TestMaxScalePerformance runs the largest supported workload: 32
// campaigns over a 90-day period with budgets up to 1e9 cents, both as a
// first-time schedule and as a replan after 45 settled days. It must
// finish well within 5 seconds and 256 MiB.
func TestMaxScalePerformance(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	const (
		days      = 90
		campaigns = 32
		maxBudget = 1_000_000_000
	)
	build := func(elapsed int) Request {
		m := days - elapsed
		req := Request{Days: days, Caps: make([]int64, m)}
		// Caps sized so the workload is feasible but tight: total future
		// capacity slightly above the remaining budget sum.
		var totalBudget int64
		for i := 0; i < campaigns; i++ {
			c := Campaign{
				Budget:   10_000 + rng.Int63n(maxBudget-10_000),
				Strategy: Strategy(rng.Intn(3)),
				Paused:   make([]bool, m),
			}
			for d := range c.Paused {
				c.Paused[d] = rng.Intn(10) == 0
			}
			if elapsed > 0 {
				c.Actual = rng.Int63n(c.Budget / 2)
			}
			totalBudget += c.Budget - c.Actual
			req.Campaigns = append(req.Campaigns, c)
		}
		perDay := totalBudget/int64(m) + 1
		for d := range req.Caps {
			req.Caps[d] = perDay + rng.Int63n(perDay/2+1)
		}
		return req
	}

	run := func(name string, req Request) {
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		start := time.Now()
		res, err := Plan(req)
		elapsed := time.Since(start)
		runtime.ReadMemStats(&after)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !res.Feasible {
			t.Fatalf("%s: expected a feasible plan", name)
		}
		verifyPlan(t, req, res)
		allocMiB := float64(after.TotalAlloc-before.TotalAlloc) / (1 << 20)
		t.Logf("%s: %v, peak heap %d MiB, total alloc %.1f MiB, deviation %d",
			name, elapsed, after.HeapInuse/(1<<20), allocMiB, res.MaxDeviation)
		if elapsed > 5*time.Second {
			t.Fatalf("%s: took %v, limit is 5s", name, elapsed)
		}
		if after.HeapInuse > 256*(1<<20) {
			t.Fatalf("%s: heap in use %d MiB exceeds 256 MiB", name, after.HeapInuse/(1<<20))
		}
	}

	run("first-schedule", build(0))
	run("replan", build(days/2))
}
