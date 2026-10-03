package pacing

import "testing"

// verifyPlan checks every hard constraint of a feasible result and
// recomputes the actual maximum deviation from scratch.
func verifyPlan(t *testing.T, req Request, res Result) {
	t.Helper()
	if !res.Feasible {
		t.Fatalf("expected a feasible plan")
	}
	m := len(req.Caps)
	if len(res.Daily) != len(req.Campaigns) {
		t.Fatalf("got %d campaign plans, want %d", len(res.Daily), len(req.Campaigns))
	}
	elapsed := req.Days - m
	var maxDev int64
	for i, c := range req.Campaigns {
		if len(res.Daily[i]) != m {
			t.Fatalf("campaign %d: got %d days, want %d", i, len(res.Daily[i]), m)
		}
		cum := c.Actual
		for day := 0; day < m; day++ {
			spend := res.Daily[i][day]
			if spend < 0 {
				t.Fatalf("campaign %d day %d: negative spend %d", i, day, spend)
			}
			if c.Paused[day] && spend != 0 {
				t.Fatalf("campaign %d day %d: spend %d on paused day", i, day, spend)
			}
			cum += spend
			target := cumulativeTarget(c.Budget, req.Days, c.Strategy, elapsed+day+1)
			if d := abs64(cum - target); d > maxDev {
				maxDev = d
			}
		}
		if cum != c.Budget {
			t.Fatalf("campaign %d: total %d does not equal budget %d", i, cum, c.Budget)
		}
	}
	for day := 0; day < m; day++ {
		var sum int64
		for i := range req.Campaigns {
			sum += res.Daily[i][day]
		}
		if sum > req.Caps[day] {
			t.Fatalf("day %d: total spend %d exceeds cap %d", day, sum, req.Caps[day])
		}
	}
	if maxDev != res.MaxDeviation {
		t.Fatalf("reported deviation %d, recomputed %d", res.MaxDeviation, maxDev)
	}
}

func noPause(m int) []bool { return make([]bool, m) }

func TestSharedCapForcesYield(t *testing.T) {
	// Two even campaigns over 4 days, budgets 300 and 500. Ideal spend
	// is 75 + 125 = 200/day, but day 1 is capped at 100, so at least one
	// campaign must fall behind its target on day 1.
	req := Request{
		Days: 4,
		Caps: []int64{100, 200, 200, 300},
		Campaigns: []Campaign{
			{Budget: 300, Strategy: StrategyEven, Paused: noPause(4)},
			{Budget: 500, Strategy: StrategyEven, Paused: noPause(4)},
		},
	}
	res, err := Plan(req)
	if err != nil {
		t.Fatal(err)
	}
	verifyPlan(t, req, res)
	// Day 1 targets are 75 and 125 (sum 200) but the cap is 100, so the
	// best split is 25/75 leaving both campaigns exactly 50 behind.
	// Later days can catch up exactly (caps 200/200/300 fit the windows
	// of +/-50), so the optimum is 50.
	if res.MaxDeviation != 50 {
		t.Fatalf("max deviation: got %d want 50", res.MaxDeviation)
	}
}

func TestReplanOddDaysHistoryAheadAndBehind(t *testing.T) {
	// 5-day period, replan after 2 settled days. Campaign A (front
	// loaded) overshot its day-2 target of 240; campaign B (back
	// loaded) undershot its day-2 target of 80.
	req := Request{
		Days: 5,
		Caps: []int64{500, 500, 500},
		Campaigns: []Campaign{
			{Budget: 500, Strategy: StrategyFrontLoaded, Paused: noPause(3), Actual: 400},
			{Budget: 500, Strategy: StrategyBackLoaded, Paused: noPause(3), Actual: 10},
		},
	}
	res, err := Plan(req)
	if err != nil {
		t.Fatal(err)
	}
	verifyPlan(t, req, res)
	// Historical day-2 deviations do not count. A is 60 ahead of its
	// day-3 target of 340 (400 already spent, nothing unspent), which is
	// unavoidable; B can track its remaining targets exactly.
	if res.MaxDeviation != 60 {
		t.Fatalf("max deviation: got %d want 60", res.MaxDeviation)
	}
}

func TestInfeasibleDespiteSufficientTotalCapacity(t *testing.T) {
	// Total future capacity 1600 far exceeds the 1000 budget, but the
	// only unpaused day has cap 600.
	req := Request{
		Days: 2,
		Caps: []int64{600, 1000},
		Campaigns: []Campaign{
			{Budget: 1000, Strategy: StrategyEven, Paused: []bool{false, true}},
		},
	}
	res, err := Plan(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Feasible {
		t.Fatalf("expected infeasible, got plan %v", res.Daily)
	}
}

func TestInfeasibleZeroCaps(t *testing.T) {
	req := Request{
		Days: 3,
		Caps: []int64{0, 0, 0},
		Campaigns: []Campaign{
			{Budget: 10000, Strategy: StrategyEven, Paused: noPause(3)},
		},
	}
	res, err := Plan(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Feasible {
		t.Fatalf("expected infeasible with zero caps")
	}
}

func TestInfeasibleActualExceedsBudget(t *testing.T) {
	req := Request{
		Days: 3,
		Caps: []int64{100},
		Campaigns: []Campaign{
			{Budget: 100, Strategy: StrategyEven, Paused: noPause(1), Actual: 150},
		},
	}
	res, err := Plan(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Feasible {
		t.Fatalf("expected infeasible when actual exceeds budget")
	}
}

func TestFirstScheduleExactFit(t *testing.T) {
	// Caps exactly match the even plan: deviation must be 0.
	req := Request{
		Days: 4,
		Caps: []int64{100, 100, 100, 100},
		Campaigns: []Campaign{
			{Budget: 200, Strategy: StrategyEven, Paused: noPause(4)},
			{Budget: 200, Strategy: StrategyEven, Paused: noPause(4)},
		},
	}
	res, err := Plan(req)
	if err != nil {
		t.Fatal(err)
	}
	verifyPlan(t, req, res)
	if res.MaxDeviation != 0 {
		t.Fatalf("max deviation: got %d want 0", res.MaxDeviation)
	}
}

func TestPausedDayStillCountsForDeviation(t *testing.T) {
	// One campaign, 3 days, middle day paused. Budget 300, even target
	// 100/day. Best plan: 150/0/150, deviations 50/50/0 -> 50.
	req := Request{
		Days: 3,
		Caps: []int64{1000, 1000, 1000},
		Campaigns: []Campaign{
			{Budget: 300, Strategy: StrategyEven, Paused: []bool{false, true, false}},
		},
	}
	res, err := Plan(req)
	if err != nil {
		t.Fatal(err)
	}
	verifyPlan(t, req, res)
	if res.MaxDeviation != 50 {
		t.Fatalf("max deviation: got %d want 50", res.MaxDeviation)
	}
}
