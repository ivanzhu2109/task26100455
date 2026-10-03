package pacing

// cumulativeTarget 返回第 d 天结束时的累计目标（美分，向下取整）。
// 目标曲线仅由总预算 budget、完整周期天数 days 与策略 strategy 决定，
// 暂停与历史支出偏离均不移动曲线。d 取 1..days。
//
// 精确比例（奇数天周期的中间一天在两个半段中各占半天）：
//
//	均匀: B*d/N
//	前置: 2d<=N 时 6Bd/(5N)，否则 B(4d+N)/(5N)
//	后置: 2d<=N 时 4Bd/(5N)，否则 B(6d-N)/(5N)
//
// 先按精确比例计算整日末累计值，再向下取整到美分；
// 不对每日增量取整再累加。全部使用整数运算，结果精确。
func cumulativeTarget(budget int64, days int, strategy Strategy, d int) int64 {
	n := int64(days)
	dd := int64(d)
	switch strategy {
	case StrategyFront:
		if 2*dd <= n {
			return 6 * budget * dd / (5 * n)
		}
		return budget * (4*dd + n) / (5 * n)
	case StrategyBack:
		if 2*dd <= n {
			return 4 * budget * dd / (5 * n)
		}
		return budget * (6*dd - n) / (5 * n)
	default: // StrategyEven
		return budget * dd / n
	}
}
