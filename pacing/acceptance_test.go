package pacing

import "testing"

// checkResult 校验所有有解结果的硬约束：每日共享上限、暂停日为 0、
// 逐活动预算守恒（历史+计划恰好等于总预算），并返回实际最大偏差。
func checkResult(t *testing.T, days, settled int, acts []Activity, actual []int64, caps []int64, res Result) int64 {
	t.Helper()
	if !res.Feasible {
		t.Fatal("expected feasible")
	}
	f := days - settled
	for j := 0; j < f; j++ {
		sum := int64(0)
		for i := range acts {
			x := res.Spend[i][j]
			if x < 0 {
				t.Fatalf("negative spend act %d day %d", i, j)
			}
			if acts[i].Paused[j] && x != 0 {
				t.Fatalf("paused day spend act %d day %d: %d", i, j, x)
			}
			sum += x
		}
		if sum > caps[j] {
			t.Fatalf("day %d cap exceeded: %d > %d", j, sum, caps[j])
		}
	}
	dev := int64(0)
	for i, a := range acts {
		s := actual[i]
		for j := 0; j < f; j++ {
			s += res.Spend[i][j]
			d := s - cumulativeTarget(a.TotalBudget, days, a.Strategy, settled+1+j)
			if d < 0 {
				d = -d
			}
			if d > dev {
				dev = d
			}
		}
		if s != a.TotalBudget {
			t.Fatalf("act %d budget not conserved: got %d want %d", i, s, a.TotalBudget)
		}
	}
	if dev != res.MaxDeviation {
		t.Fatalf("reported deviation %d != plan deviation %d", res.MaxDeviation, dev)
	}
	return dev
}

// 共享上限迫使多个活动相互让出额度。
func TestSharedCapForcesYield(t *testing.T) {
	// 两个均匀活动各需 5000/天，第 1 天共享上限 6000 迫使双方退让。
	// 最优：第 1 天各 3000，第 2 天各 7000，最大偏差 2000。
	acts := []Activity{
		{TotalBudget: 10000, Strategy: StrategyEven, Paused: make([]bool, 2)},
		{TotalBudget: 10000, Strategy: StrategyEven, Paused: make([]bool, 2)},
	}
	res, err := PlanInitial(2, acts, []int64{6000, 14000})
	if err != nil {
		t.Fatal(err)
	}
	if dev := checkResult(t, 2, 0, acts, []int64{0, 0}, []int64{6000, 14000}, res); dev != 2000 {
		t.Fatalf("deviation got %d want 2000", dev)
	}
}

// 奇数天周期，历史支出落后后的重排。
func TestReplanOddDaysHistoryBehind(t *testing.T) {
	// N=5 前置，目标 [2400 4800 6800 8400 10000]；已结算 2 天，实际仅 1000（落后）。
	// 剩余 9000，未来 3 天上限各 3000 -> 只能 3000/天，
	// 累计 [4000 7000 10000]，偏差 [2800 1400 0] -> 2800。
	acts := []Activity{{TotalBudget: 10000, Strategy: StrategyFront, Paused: make([]bool, 3)}}
	res, err := Replan(5, 2, acts, []int64{1000}, []int64{3000, 3000, 3000})
	if err != nil {
		t.Fatal(err)
	}
	if dev := checkResult(t, 5, 2, acts, []int64{1000}, []int64{3000, 3000, 3000}, res); dev != 2800 {
		t.Fatalf("deviation got %d want 2800", dev)
	}
}

// 奇数天周期，历史支出超前后的重排。
func TestReplanOddDaysHistoryAhead(t *testing.T) {
	// N=5 前置；已结算 2 天，实际 6000（超前于目标 4800）。
	// 剩余 4000，上限 [1000 1000 2000] -> 累计 [7000 8000 10000]，
	// 对目标 [6800 8400 10000] 偏差 [200 400 0] -> 400。
	acts := []Activity{{TotalBudget: 10000, Strategy: StrategyFront, Paused: make([]bool, 3)}}
	res, err := Replan(5, 2, acts, []int64{6000}, []int64{1000, 1000, 2000})
	if err != nil {
		t.Fatal(err)
	}
	if dev := checkResult(t, 5, 2, acts, []int64{6000}, []int64{1000, 1000, 2000}, res); dev != 400 {
		t.Fatalf("deviation got %d want 400", dev)
	}
}

// 暂停日偏差也参与优化。
func TestPausedDayCountsInDeviation(t *testing.T) {
	// N=3 均匀，目标 [3333 6666 10000]；第 2 天暂停 -> S(1)=S(2)。
	// 最优 S(1)=4999 或 5000，偏差 max(1666,1667,0)=1667。
	acts := []Activity{{TotalBudget: 10000, Strategy: StrategyEven, Paused: []bool{false, true, false}}}
	res, err := PlanInitial(3, acts, []int64{10000, 10000, 10000})
	if err != nil {
		t.Fatal(err)
	}
	if dev := checkResult(t, 3, 0, acts, []int64{0}, []int64{10000, 10000, 10000}, res); dev != 1667 {
		t.Fatalf("deviation got %d want 1667", dev)
	}
}

// 总容量足够但暂停集中导致无解。
func TestInfeasibleDespiteEnoughTotalCapacity(t *testing.T) {
	// 两个活动前 3 天均暂停，仅第 4 天可投；第 4 天上限 15000 < 20000。
	// 未来总容量 45000 远大于总预算 20000，但仍无解。
	paused := []bool{true, true, true, false}
	acts := []Activity{
		{TotalBudget: 10000, Strategy: StrategyEven, Paused: paused},
		{TotalBudget: 10000, Strategy: StrategyEven, Paused: paused},
	}
	res, err := PlanInitial(4, acts, []int64{10000, 10000, 10000, 15000})
	if err != nil {
		t.Fatal(err)
	}
	if res.Feasible {
		t.Fatal("expected infeasible: pauses concentrate spend beyond day-4 cap")
	}
}

// 重排中历史支出已超过总预算 -> 无解（不能少花或挪用）。
func TestReplanOverspentInfeasible(t *testing.T) {
	acts := []Activity{{TotalBudget: 10000, Strategy: StrategyEven, Paused: make([]bool, 2)}}
	res, err := Replan(4, 2, acts, []int64{12000}, []int64{10000, 10000})
	if err != nil {
		t.Fatal(err)
	}
	if res.Feasible {
		t.Fatal("expected infeasible: actual spend exceeds total budget")
	}
}

// 上限为零的日期等价于全活动暂停。
func TestZeroCapDays(t *testing.T) {
	// 第 2 天共享上限为 0；等价于第 2 天暂停，最优偏差 1667（同上例）。
	acts := []Activity{{TotalBudget: 10000, Strategy: StrategyEven, Paused: make([]bool, 3)}}
	res, err := PlanInitial(3, acts, []int64{10000, 0, 10000})
	if err != nil {
		t.Fatal(err)
	}
	if dev := checkResult(t, 3, 0, acts, []int64{0}, []int64{10000, 0, 10000}, res); dev != 1667 {
		t.Fatalf("deviation got %d want 1667", dev)
	}
}
