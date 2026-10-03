package pacing

import "fmt"

// Plan computes an optimal daily spend plan for the request.
//
// The minimax problem is solved exactly: for a candidate deviation D every
// campaign's cumulative planned spend at the end of future day t is forced
// into the window target(t) +/- D (shifted by the settled actuals), which
// turns feasibility into a flow problem with lower/upper bounds on a
// time-expanded network. Binary search over integer D then yields the
// exact optimum, and any feasible flow at that D is a valid optimal plan.
func Plan(req Request) (Result, error) {
	if err := validate(req); err != nil {
		return Result{}, err
	}
	m := len(req.Caps)
	k := len(req.Campaigns)
	remaining := make([]int64, k)
	for i, c := range req.Campaigns {
		remaining[i] = c.Budget - c.Actual
	}
	if m == 0 {
		for _, r := range remaining {
			if r != 0 {
				return Result{Feasible: false}, nil
			}
		}
		return Result{Feasible: true, MaxDeviation: 0, Daily: make([][]int64, k)}, nil
	}
	for _, r := range remaining {
		if r < 0 {
			// Settled spend already exceeds the total budget.
			return Result{Feasible: false}, nil
		}
	}

	elapsed := req.Days - m
	targets := make([][]int64, k)
	for i, c := range req.Campaigns {
		targets[i] = make([]int64, m+1)
		for t := 1; t <= m; t++ {
			targets[i][t] = cumulativeTarget(c.Budget, req.Days, c.Strategy, elapsed+t)
		}
	}

	// Upper bound on the optimal deviation: any plan keeps cumulative
	// planned spend within [0, remaining], so the deviation at day t is
	// bounded by the farther of the two window ends from the target.
	var hi int64
	for i := range req.Campaigns {
		act := req.Campaigns[i].Actual
		for t := 1; t <= m; t++ {
			if d := abs64(targets[i][t] - act); d > hi {
				hi = d
			}
			if d := abs64(targets[i][t] - act - remaining[i]); d > hi {
				hi = d
			}
		}
	}

	// Feasibility is monotone in D; if even hi is infeasible no plan can
	// satisfy the hard constraints at all.
	if _, ok := solveAtDeviation(req, targets, remaining, hi); !ok {
		return Result{Feasible: false}, nil
	}
	lo := int64(-1)
	for hi-lo > 1 {
		mid := lo + (hi-lo)/2
		if _, ok := solveAtDeviation(req, targets, remaining, mid); ok {
			hi = mid
		} else {
			lo = mid
		}
	}
	flows, _ := solveAtDeviation(req, targets, remaining, hi)

	daily := make([][]int64, k)
	for i := range daily {
		daily[i] = make([]int64, m)
		for t := 0; t < m; t++ {
			daily[i][t] = flows[i*m+t]
		}
	}
	return Result{Feasible: true, MaxDeviation: hi, Daily: daily}, nil
}

// solveAtDeviation builds the feasibility network for candidate deviation
// d and returns the per-day injection flows (id i*m+(t-1)) when feasible.
func solveAtDeviation(req Request, targets [][]int64, remaining []int64, d int64) ([]int64, bool) {
	m := len(req.Caps)
	k := len(req.Campaigns)
	const inf = int64(1) << 60

	// Node layout: 0 = source, 1 = sink, 2+t = day node (t 0-based),
	// then per campaign a chain of m+1 cumulative nodes.
	dayNode := func(t int) int { return 2 + t }
	chainNode := func(i, t int) int { return 2 + m + i*(m+1) + t }
	numNodes := 2 + m + k*(m+1)

	edges := make([]lbEdge, 0, k*(2*m+1)+2*m+1)
	for t := 0; t < m; t++ {
		edges = append(edges, lbEdge{from: 0, to: dayNode(t), lo: 0, hi: req.Caps[t], id: -1})
	}
	for i, c := range req.Campaigns {
		act := c.Actual
		r := remaining[i]
		for t := 1; t <= m; t++ {
			lo := targets[i][t] - d - act
			if lo < 0 {
				lo = 0
			}
			hi := targets[i][t] + d - act
			if hi > r {
				hi = r
			}
			if hi < lo {
				return nil, false
			}
			edges = append(edges, lbEdge{from: chainNode(i, t-1), to: chainNode(i, t), lo: lo, hi: hi, id: -1})
			// Day t's spend enters the chain at node t-1 so that the
			// edge (t-1)->t carries the cumulative spend after day t.
			// Paused days keep a zero-capacity edge so that flow ids stay
			// dense across the whole campaign/day grid.
			injectCap := inf
			if c.Paused[t-1] {
				injectCap = 0
			}
			edges = append(edges, lbEdge{from: dayNode(t - 1), to: chainNode(i, t-1), lo: 0, hi: injectCap, id: i*m + (t - 1)})
		}
		edges = append(edges, lbEdge{from: chainNode(i, m), to: 1, lo: r, hi: r, id: -1})
	}
	// Circulation edge so the lower-bound reduction can balance flow.
	edges = append(edges, lbEdge{from: 1, to: 0, lo: 0, hi: inf, id: -1})

	return feasibleFlow(numNodes, edges)
}

func validate(req Request) error {
	if req.Days <= 0 {
		return fmt.Errorf("pacing: Days must be positive, got %d", req.Days)
	}
	if len(req.Caps) > req.Days {
		return fmt.Errorf("pacing: %d future caps exceed the %d-day period", len(req.Caps), req.Days)
	}
	for t, c := range req.Caps {
		if c < 0 {
			return fmt.Errorf("pacing: negative cap %d on future day %d", c, t)
		}
	}
	for i, c := range req.Campaigns {
		if c.Budget < 0 {
			return fmt.Errorf("pacing: campaign %d has negative budget %d", i, c.Budget)
		}
		if c.Actual < 0 {
			return fmt.Errorf("pacing: campaign %d has negative actual spend %d", i, c.Actual)
		}
		if len(c.Paused) != len(req.Caps) {
			return fmt.Errorf("pacing: campaign %d has %d pause flags for %d future days", i, len(c.Paused), len(req.Caps))
		}
		if c.Strategy < StrategyEven || c.Strategy > StrategyBackLoaded {
			return fmt.Errorf("pacing: campaign %d has unknown strategy %d", i, c.Strategy)
		}
	}
	return nil
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
