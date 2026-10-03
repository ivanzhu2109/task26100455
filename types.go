// Package pacing plans daily spend for a batch of ad campaigns that share
// the agency's daily spend caps. It is a pure in-memory library: inputs and
// results live in Go values, nothing is persisted.
package pacing

// Strategy describes the shape of the cumulative target curve over the
// whole period.
type Strategy int

const (
	// StrategyEven spreads the budget uniformly over the period.
	StrategyEven Strategy = iota
	// StrategyFrontLoaded builds 60% of the target in the first half of
	// the period and 40% in the second half.
	StrategyFrontLoaded
	// StrategyBackLoaded builds 40% of the target in the first half of
	// the period and 60% in the second half.
	StrategyBackLoaded
)

// Campaign describes one activity to plan.
type Campaign struct {
	// Budget is the total budget in cents for the whole period.
	Budget int64
	// Strategy is the pacing strategy for the whole period.
	Strategy Strategy
	// Paused marks future days (same indexing as Request.Caps) on which
	// the campaign must not spend.
	Paused []bool
	// Actual is the cumulative settled spend in cents over the elapsed
	// days of the period. Zero for a first-time plan.
	Actual int64
}

// Request is a single planning call. It covers both first-time scheduling
// and replanning: the period always has Days days in total, and Caps
// describes the remaining future days.
type Request struct {
	// Days is the total number of full delivery days in the period.
	Days int
	// Caps is the shared daily spend cap in cents for each future day.
	// A cap may be zero. len(Caps) <= Days; the first Days-len(Caps)
	// days are already settled.
	Caps []int64
	// Campaigns is the batch of activities sharing the caps.
	Campaigns []Campaign
}

// Result is the outcome of a planning call.
type Result struct {
	// Feasible reports whether a plan satisfying every hard constraint
	// exists. When false, Daily is nil and MaxDeviation is meaningless.
	Feasible bool
	// MaxDeviation is the globally minimal value, in cents, of the
	// maximum absolute deviation between cumulative actual+planned
	// spend and the cumulative target, taken over every campaign and
	// every future day end.
	MaxDeviation int64
	// Daily[i][t] is the planned spend in cents of campaign i on future
	// day t. It is set only when Feasible is true.
	Daily [][]int64
}
