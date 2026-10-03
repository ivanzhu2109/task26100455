// Package pacing 提供广告活动预算节奏规划能力。
// 全部数据保存在内存中，由上层直接调用，不做持久化。
package pacing

// Strategy 表示活动的节奏策略。
type Strategy int

const (
	// StrategyEven 均匀：按整个周期匀速形成累计目标。
	StrategyEven Strategy = iota
	// StrategyFront 前置：周期前半段匀速形成 60% 目标，后半段 40%。
	StrategyFront
	// StrategyBack 后置：周期前半段匀速形成 40% 目标，后半段 60%。
	StrategyBack
)

// Activity 描述一个广告活动。
type Activity struct {
	// TotalBudget 总预算，单位美分，范围 [10000, 1000000000]。
	TotalBudget int64
	// Strategy 节奏策略。
	Strategy Strategy
	// Paused 未来每日是否暂停，长度为未来天数（N-k）。
	// Paused[j] 对应第 k+1+j 天（1-based）。
	Paused []bool
}

// Result 是规划结果。
type Result struct {
	// Feasible 为 false 表示无可行计划，其余字段无效。
	Feasible bool
	// MaxDeviation 所有活动所有未来日末 |累计实际+累计计划-累计目标| 的最大值的最优（最小）值，单位美分。
	MaxDeviation int64
	// Spend 计划支出：Spend[i][j] 为活动 i 在第 k+1+j 天的计划支出（美分）。
	Spend [][]int64
}
