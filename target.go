package pacing

// cumulativeTarget returns the cumulative target spend in cents at the end
// of day t (0 <= t <= days) for the given total budget, period length and
// strategy. The value is the exact proportional target floored to cents;
// daily increments are never rounded individually.
//
// The two halves of a front/back loaded period are split by duration: for
// an odd number of days the middle day belongs half to each segment, which
// is why the formulas below are expressed over the doubled timeline 2t.
func cumulativeTarget(budget int64, days int, s Strategy, t int) int64 {
	n := int64(days)
	tt := int64(t)
	switch s {
	case StrategyFrontLoaded:
		if 2*tt <= n {
			// 60% of the budget spread uniformly over the first half.
			return budget * 6 * tt / (5 * n)
		}
		// 60% reached at the midpoint, then 40% over the second half.
		return budget * (n + 4*tt) / (5 * n)
	case StrategyBackLoaded:
		if 2*tt <= n {
			// 40% of the budget spread uniformly over the first half.
			return budget * 4 * tt / (5 * n)
		}
		// 40% reached at the midpoint, then 60% over the second half.
		return budget * (6*tt - n) / (5 * n)
	default: // StrategyEven
		return budget * tt / n
	}
}
