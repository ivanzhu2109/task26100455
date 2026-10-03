package pacing

import "testing"

func TestCumulativeTargetEven(t *testing.T) {
	// B=10000, N=4: 2500/5000/7500/10000
	want := []int64{2500, 5000, 7500, 10000}
	for d := 1; d <= 4; d++ {
		if got := cumulativeTarget(10000, 4, StrategyEven, d); got != want[d-1] {
			t.Fatalf("even d=%d got %d want %d", d, got, want[d-1])
		}
	}
}

func TestCumulativeTargetFrontBackEvenDays(t *testing.T) {
	// B=10000, N=4, 半段=2天
	// 前置: 3000/6000/8000/10000
	wantF := []int64{3000, 6000, 8000, 10000}
	// 后置: 2000/4000/7000/10000
	wantB := []int64{2000, 4000, 7000, 10000}
	for d := 1; d <= 4; d++ {
		if got := cumulativeTarget(10000, 4, StrategyFront, d); got != wantF[d-1] {
			t.Fatalf("front d=%d got %d want %d", d, got, wantF[d-1])
		}
		if got := cumulativeTarget(10000, 4, StrategyBack, d); got != wantB[d-1] {
			t.Fatalf("back d=%d got %d want %d", d, got, wantB[d-1])
		}
	}
}

func TestCumulativeTargetOddDaysMiddleSplit(t *testing.T) {
	// B=10000, N=5, 半段=2.5天, 中间第3天两半段各占半天
	// 前置: 前半段日速率 6000/2.5=2400 -> 2400/4800; 第3天末=6000+0.4*10000*(0.5/2.5)=6800
	wantF := []int64{2400, 4800, 6800, 8400, 10000}
	// 后置: 前半段日速率 4000/2.5=1600 -> 1600/3200; 第3天末=4000+0.6*10000*(0.5/2.5)=5200
	wantB := []int64{1600, 3200, 5200, 7600, 10000}
	for d := 1; d <= 5; d++ {
		if got := cumulativeTarget(10000, 5, StrategyFront, d); got != wantF[d-1] {
			t.Fatalf("front odd d=%d got %d want %d", d, got, wantF[d-1])
		}
		if got := cumulativeTarget(10000, 5, StrategyBack, d); got != wantB[d-1] {
			t.Fatalf("back odd d=%d got %d want %d", d, got, wantB[d-1])
		}
	}
}

func TestCumulativeTargetFloorAndEndpoints(t *testing.T) {
	// 取整: B=10001, N=3 均匀 -> 3333.66->3333, 6667.33->6667, 10001
	want := []int64{3333, 6667, 10001}
	for d := 1; d <= 3; d++ {
		if got := cumulativeTarget(10001, 3, StrategyEven, d); got != want[d-1] {
			t.Fatalf("floor d=%d got %d want %d", d, got, want[d-1])
		}
	}
	// 末日常目标恒等于总预算（全策略、奇偶周期）
	for _, n := range []int{1, 2, 3, 89, 90} {
		for _, s := range []Strategy{StrategyEven, StrategyFront, StrategyBack} {
			if got := cumulativeTarget(1000000000, n, s, n); got != 1000000000 {
				t.Fatalf("n=%d s=%d final got %d want 1000000000", n, s, got)
			}
		}
	}
}
