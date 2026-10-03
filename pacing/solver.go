package pacing

import "errors"

// ErrInvalidInput 表示输入参数不合法。
var ErrInvalidInput = errors.New("pacing: invalid input")

const (
	minBudget  = int64(10000)
	maxBudget  = int64(1000000000)
	maxDays    = 90
	maxActions = 32
)

// instance 是规范化后的求解实例。
type instance struct {
	days    int // 完整周期天数 N
	settled int // 已结算天数 k
	future  int // 未来天数 N-k
	budget  []int64
	actual  []int64    // 每个活动截至第 k 天的累计实际支出
	strat   []Strategy // 保留策略以便调试
	paused  [][]bool   // paused[i][j]，j 对应未来第 j 天
	caps    []int64    // 未来每日共享上限
	target  [][]int64  // target[i][d]，d 为 1..days（下标 0 未用）
}

// PlanInitial 首次排期：无历史支出，全部 N 天为未来。
// caps 长度必须为 days；每个活动的 Paused 长度也必须为 days。
func PlanInitial(days int, activities []Activity, caps []int64) (Result, error) {
	return solve(days, 0, activities, nil, caps)
}

// Replan 剩余周期重排：已结算连续前 settled 天（1 <= settled < days），
// actual[i] 为活动 i 截至第 settled 天的累计实际支出，历史支出不可改变。
// caps 与每个活动的 Paused 长度必须为 days-settled（仅未来天）。
func Replan(days, settled int, activities []Activity, actual []int64, caps []int64) (Result, error) {
	if settled < 1 || settled >= days {
		return Result{}, ErrInvalidInput
	}
	return solve(days, settled, activities, actual, caps)
}

// solve 统一求解入口。返回不可行时 Result.Feasible=false 且 error 为 nil。
func solve(days, settled int, activities []Activity, actual []int64, caps []int64) (Result, error) {
	inst, err := buildInstance(days, settled, activities, actual, caps)
	if err != nil {
		return Result{}, err
	}
	return solveInstance(inst), nil
}

// solveInstance 对规范化实例求精确最优解。不可行时返回 Result{Feasible:false}。
func solveInstance(inst *instance) Result {
	// 偏差上界：任何累计支出与目标之差都不超过最大预算。
	hi := int64(0)
	for _, b := range inst.budget {
		if b > hi {
			hi = b
		}
	}
	// 先判定是否存在任何可行计划（dev 取上界时偏差约束失效，
	// 仅剩每日上限、暂停与花完预算约束）。
	if feasibleWithDeviation(inst, hi) == nil {
		return Result{Feasible: false}
	}
	// 二分最小可行偏差（整数精确）。
	lo := int64(-1) // 不可行
	for hi-lo > 1 {
		mid := lo + (hi-lo)/2
		if feasibleWithDeviation(inst, mid) != nil {
			hi = mid
		} else {
			lo = mid
		}
	}
	return Result{
		Feasible:     true,
		MaxDeviation: hi,
		Spend:        feasibleWithDeviation(inst, hi),
	}
}

func buildInstance(days, settled int, activities []Activity, actual []int64, caps []int64) (*instance, error) {
	if days < 1 || days > maxDays || settled < 0 || settled >= days {
		return nil, ErrInvalidInput
	}
	m := len(activities)
	if m < 1 || m > maxActions {
		return nil, ErrInvalidInput
	}
	future := days - settled
	if len(caps) != future {
		return nil, ErrInvalidInput
	}
	for _, c := range caps {
		if c < 0 {
			return nil, ErrInvalidInput
		}
	}
	if actual == nil {
		actual = make([]int64, m)
	}
	if len(actual) != m {
		return nil, ErrInvalidInput
	}

	inst := &instance{
		days:    days,
		settled: settled,
		future:  future,
		budget:  make([]int64, m),
		actual:  make([]int64, m),
		strat:   make([]Strategy, m),
		paused:  make([][]bool, m),
		caps:    append([]int64(nil), caps...),
		target:  make([][]int64, m),
	}
	for i, a := range activities {
		if a.TotalBudget < minBudget || a.TotalBudget > maxBudget {
			return nil, ErrInvalidInput
		}
		if a.Strategy != StrategyEven && a.Strategy != StrategyFront && a.Strategy != StrategyBack {
			return nil, ErrInvalidInput
		}
		if len(a.Paused) != future {
			return nil, ErrInvalidInput
		}
		if actual[i] < 0 {
			return nil, ErrInvalidInput
		}
		inst.budget[i] = a.TotalBudget
		inst.actual[i] = actual[i]
		inst.strat[i] = a.Strategy
		inst.paused[i] = append([]bool(nil), a.Paused...)
		row := make([]int64, days+1)
		for d := 1; d <= days; d++ {
			row[d] = cumulativeTarget(a.TotalBudget, days, a.Strategy, d)
		}
		inst.target[i] = row
	}
	return inst, nil
}
