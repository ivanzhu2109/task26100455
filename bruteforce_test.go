package pacing

import (
	"math/rand"
	"testing"
)

// bruteForce solves the minimax problem exactly by dynamic programming
// over the joint cumulative-spend state space. It is only practical for
// tiny budgets and horizons, which is exactly what these tests use.
//
// The state after future day t is the vector of cumulative planned spend
// per campaign; dp maps state -> minimal achievable max deviation so far.
func bruteForce(req Request) (int64, bool) {
	m := len(req.Caps)
	k := len(req.Campaigns)
	elapsed := req.Days - m
	remaining := make([]int64, k)
	for i, c := range req.Campaigns {
		remaining[i] = c.Budget - c.Actual
		if remaining[i] < 0 {
			return 0, false
		}
	}
	if m == 0 {
		for _, r := range remaining {
			if r != 0 {
				return 0, false
			}
		}
		return 0, true
	}

	type state struct {
		cum [4]int64 // up to 4 campaigns in brute-force tests
	}
	// dev computes the max |cum+actual-target| over campaigns at day t.
	dev := func(st state, t int) int64 {
		var worst int64
		for i, c := range req.Campaigns {
			target := cumulativeTarget(c.Budget, req.Days, c.Strategy, elapsed+t)
			if d := abs64(st.cum[i] + c.Actual - target); d > worst {
				worst = d
			}
		}
		return worst
	}

	const inf = int64(1) << 62
	cur := map[state]int64{state{}: 0}
	var alloc [4]int64
	for t := 1; t <= m; t++ {
		next := make(map[state]int64)
		for st, best := range cur {
			// emit records one day's allocation leading to state ns.
			emit := func(ns state) {
				for j := 0; j < k; j++ {
					if ns.cum[j] > remaining[j] {
						return
					}
				}
				cost := dev(ns, t)
				if best > cost {
					cost = best
				}
				if old, seen := next[ns]; !seen || cost < old {
					next[ns] = cost
				}
			}
			// rec enumerates every allocation of the day's spend across
			// campaigns, respecting pauses and remaining budgets.
			var rec func(i int, left int64)
			rec = func(i int, left int64) {
				if i == k {
					ns := st
					for j := 0; j < k; j++ {
						ns.cum[j] += alloc[j]
					}
					emit(ns)
					return
				}
				if req.Campaigns[i].Paused[t-1] {
					alloc[i] = 0
					rec(i+1, left)
					return
				}
				maxSpend := remaining[i] - st.cum[i]
				if maxSpend > left {
					maxSpend = left
				}
				for x := int64(0); x <= maxSpend; x++ {
					alloc[i] = x
					rec(i+1, left-x)
				}
			}
			rec(0, req.Caps[t-1])
		}
		cur = next
		if len(cur) == 0 {
			return 0, false
		}
	}
	best := inf
	found := false
	for st, cost := range cur {
		ok := true
		for j := 0; j < k; j++ {
			if st.cum[j] != remaining[j] {
				ok = false
				break
			}
		}
		if ok && cost < best {
			best = cost
			found = true
		}
	}
	if !found {
		return 0, false
	}
	return best, true
}

func TestBruteForceMatchesLibrary(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for trial := 0; trial < 300; trial++ {
		days := 2 + rng.Intn(3) // 2..4 day period
		m := 1 + rng.Intn(days) // 1..days future days
		k := 1 + rng.Intn(3)    // 1..3 campaigns
		req := Request{Days: days, Caps: make([]int64, m)}
		for t := range req.Caps {
			req.Caps[t] = int64(rng.Intn(15))
		}
		for i := 0; i < k; i++ {
			c := Campaign{
				Budget:   int64(rng.Intn(25)),
				Strategy: Strategy(rng.Intn(3)),
				Paused:   make([]bool, m),
			}
			for t := range c.Paused {
				c.Paused[t] = rng.Intn(3) == 0
			}
			if elapsed := days - m; elapsed > 0 {
				// Random settled spend, sometimes over budget.
				c.Actual = int64(rng.Intn(int(c.Budget) + 5))
			}
			req.Campaigns = append(req.Campaigns, c)
		}
		res, err := Plan(req)
		if err != nil {
			t.Fatalf("trial %d: %v", trial, err)
		}
		wantDev, wantFeasible := bruteForce(req)
		if res.Feasible != wantFeasible {
			t.Fatalf("trial %d: feasibility mismatch: got %v want %v\nreq=%+v", trial, res.Feasible, wantFeasible, req)
		}
		if !res.Feasible {
			continue
		}
		if res.MaxDeviation != wantDev {
			t.Fatalf("trial %d: deviation got %d want %d\nreq=%+v", trial, res.MaxDeviation, wantDev, req)
		}
		verifyPlan(t, req, res)
	}
}
