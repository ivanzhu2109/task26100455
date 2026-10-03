package pacing

import (
	"math/rand"
	"runtime"
	"testing"
	"time"
)

// buildMaxScale 构造最大规模实例：32 活动、90 天、预算覆盖完整范围。
func buildMaxScale(rng *rand.Rand) (int, []Activity, []int64) {
	days := 90
	acts := make([]Activity, 32)
	total := int64(0)
	for i := range acts {
		b := int64(10000)
		if i > 0 { // 覆盖完整预算范围
			b = 10000 + rng.Int63n(1000000000-10000)
		}
		p := make([]bool, days)
		for j := range p {
			p[j] = rng.Intn(10) == 0 // 10% 暂停
		}
		acts[i] = Activity{TotalBudget: b, Strategy: Strategy(rng.Intn(3)), Paused: p}
		total += b
	}
	// 上限设为日均需求的 1.5 倍，保证有解且仍需活动间让额度。
	caps := make([]int64, days)
	capPerDay := total * 3 / 2 / int64(days)
	for j := range caps {
		caps[j] = capPerDay
	}
	return days, acts, caps
}

func memInUse() int64 {
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return int64(ms.HeapInuse)
}

func TestMaxScalePerformance(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	days, acts, caps := buildMaxScale(rng)

	before := memInUse()
	start := time.Now()
	res, err := PlanInitial(days, acts, caps)
	elapsed := time.Since(start)
	peak := memInUse()
	if err != nil {
		t.Fatal(err)
	}
	if !res.Feasible {
		t.Fatal("expected feasible at max scale")
	}
	t.Logf("PlanInitial: %v, heap delta %d KiB, deviation %d", elapsed, (peak-before)/1024, res.MaxDeviation)
	if elapsed > 5*time.Second {
		t.Fatalf("PlanInitial too slow: %v", elapsed)
	}
	if peak-before > 256*1024*1024 {
		t.Fatalf("memory exceeded: %d bytes", peak-before)
	}

	// 重排：结算前 45 天，实际支出围绕目标波动。
	settled := 45
	actual := make([]int64, len(acts))
	futureActs := make([]Activity, len(acts))
	futureCaps := caps[settled:]
	for i, a := range acts {
		t45 := cumulativeTarget(a.TotalBudget, days, a.Strategy, settled)
		delta := t45 / 20
		actual[i] = t45 + int64(rng.Intn(int(delta+1))) - delta/2
		fp := make([]bool, days-settled)
		copy(fp, a.Paused[settled:])
		futureActs[i] = Activity{TotalBudget: a.TotalBudget, Strategy: a.Strategy, Paused: fp}
	}

	before = memInUse()
	start = time.Now()
	res2, err := Replan(days, settled, futureActs, actual, futureCaps)
	elapsed = time.Since(start)
	peak = memInUse()
	if err != nil {
		t.Fatal(err)
	}
	if !res2.Feasible {
		t.Fatal("expected feasible replan at max scale")
	}
	t.Logf("Replan: %v, heap delta %d KiB, deviation %d", elapsed, (peak-before)/1024, res2.MaxDeviation)
	if elapsed > 5*time.Second {
		t.Fatalf("Replan too slow: %v", elapsed)
	}
	if peak-before > 256*1024*1024 {
		t.Fatalf("memory exceeded: %d bytes", peak-before)
	}
}
