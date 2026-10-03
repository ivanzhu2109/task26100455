package pacing

import "testing"

func TestSmokeEvenExact(t *testing.T) {
	// 单活动均匀，上限恰好等于每日匀速支出 -> 偏差 0
	res, err := PlanInitial(4, []Activity{{TotalBudget: 10000, Strategy: StrategyEven, Paused: make([]bool, 4)}}, []int64{2500, 2500, 2500, 2500})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Feasible {
		t.Fatal("expected feasible")
	}
	if res.MaxDeviation != 0 {
		t.Fatalf("deviation got %d want 0", res.MaxDeviation)
	}
	for j, x := range res.Spend[0] {
		if x != 2500 {
			t.Fatalf("day %d spend got %d want 2500", j, x)
		}
	}
}

func TestSmokeInfeasibleCapTooSmall(t *testing.T) {
	// 总容量 3000 < 预算 10000 -> 不可行
	res, err := PlanInitial(4, []Activity{{TotalBudget: 10000, Strategy: StrategyEven, Paused: make([]bool, 4)}}, []int64{1000, 1000, 1000, 0})
	if err != nil {
		t.Fatal(err)
	}
	if res.Feasible {
		t.Fatal("expected infeasible")
	}
}
