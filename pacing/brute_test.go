package pacing

import (
	"math/rand"
	"testing"
)

// bruteForce 穷举所有整数计划，返回全局最小最大偏差；不可行返回 -1。
func bruteForce(inst *instance) int64 {
	m := len(inst.budget)
	f := inst.future
	rem := make([]int64, m)
	for i := 0; i < m; i++ {
		rem[i] = inst.budget[i] - inst.actual[i]
		if rem[i] < 0 {
			return -1
		}
	}
	// 每个活动的合法日支出序列（暂停日为 0，总和为 rem）
	perAct := make([][][]int64, m)
	for i := 0; i < m; i++ {
		cur := make([]int64, f)
		var gen func(j int, left int64)
		gen = func(j int, left int64) {
			if j == f {
				if left == 0 {
					cp := append([]int64(nil), cur...)
					perAct[i] = append(perAct[i], cp)
				}
				return
			}
			if inst.paused[i][j] {
				gen(j+1, left)
				return
			}
			for x := int64(0); x <= left; x++ {
				cur[j] = x
				gen(j+1, left-x)
			}
			cur[j] = 0
		}
		gen(0, rem[i])
		if len(perAct[i]) == 0 {
			return -1
		}
	}
	best := int64(-1)
	chosen := make([]int, m)
	dayUsed := make([]int64, f)
	var dfs func(i int)
	dfs = func(i int) {
		if i == m {
			dev := int64(0)
			for a := 0; a < m; a++ {
				s := inst.actual[a]
				for j := 0; j < f; j++ {
					s += perAct[a][chosen[a]][j]
					d := s - inst.target[a][inst.settled+1+j]
					if d < 0 {
						d = -d
					}
					if d > dev {
						dev = d
					}
				}
			}
			if best < 0 || dev < best {
				best = dev
			}
			return
		}
		for p := 0; p < len(perAct[i]); p++ {
			ok := true
			for j := 0; j < f; j++ {
				dayUsed[j] += perAct[i][p][j]
				if dayUsed[j] > inst.caps[j] {
					ok = false
				}
			}
			if ok {
				chosen[i] = p
				dfs(i + 1)
			}
			for j := 0; j < f; j++ {
				dayUsed[j] -= perAct[i][p][j]
			}
		}
	}
	dfs(0)
	return best
}

// planDeviation 计算一个已解出计划的实际最大偏差，并校验全部约束。
func planDeviation(inst *instance, spend [][]int64) int64 {
	m := len(inst.budget)
	f := inst.future
	for j := 0; j < f; j++ {
		sum := int64(0)
		for i := 0; i < m; i++ {
			x := spend[i][j]
			if x < 0 {
				panic("negative spend")
			}
			if inst.paused[i][j] && x != 0 {
				panic("paused day spend")
			}
			sum += x
		}
		if sum > inst.caps[j] {
			panic("day cap exceeded")
		}
	}
	dev := int64(0)
	for i := 0; i < m; i++ {
		s := inst.actual[i]
		for j := 0; j < f; j++ {
			s += spend[i][j]
			d := s - inst.target[i][inst.settled+1+j]
			if d < 0 {
				d = -d
			}
			if d > dev {
				dev = d
			}
		}
		if s != inst.budget[i] {
			panic("budget not exactly spent")
		}
	}
	return dev
}

func TestBruteForceOptimality(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for tc := 0; tc < 400; tc++ {
		days := 1 + rng.Intn(5)
		settled := rng.Intn(days) // 0..days-1
		f := days - settled
		m := 1 + rng.Intn(3)
		inst := &instance{days: days, settled: settled, future: f}
		for i := 0; i < m; i++ {
			b := int64(rng.Intn(9)) // 0..8，故意含极小预算以穷举
			inst.budget = append(inst.budget, b)
			a := int64(0)
			if settled > 0 && rng.Intn(2) == 0 {
				a = int64(rng.Intn(int(b) + 1)) // 历史支出不超过预算
			}
			inst.actual = append(inst.actual, a)
			s := Strategy(rng.Intn(3))
			inst.strat = append(inst.strat, s)
			p := make([]bool, f)
			for j := range p {
				p[j] = rng.Intn(4) == 0 // 25% 暂停
			}
			inst.paused = append(inst.paused, p)
			row := make([]int64, days+1)
			for d := 1; d <= days; d++ {
				row[d] = cumulativeTarget(b, days, s, d)
			}
			inst.target = append(inst.target, row)
		}
		for j := 0; j < f; j++ {
			inst.caps = append(inst.caps, int64(rng.Intn(10)))
		}

		want := bruteForce(inst)
		got := solveInstance(inst)
		if want < 0 {
			if got.Feasible {
				t.Fatalf("case %d: solver feasible but brute force infeasible\ninst=%+v", tc, inst)
			}
			continue
		}
		if !got.Feasible {
			t.Fatalf("case %d: solver infeasible but brute force dev=%d\ninst=%+v", tc, want, inst)
		}
		if got.MaxDeviation != want {
			t.Fatalf("case %d: deviation got %d want %d\ninst=%+v", tc, got.MaxDeviation, want, inst)
		}
		if d := planDeviation(inst, got.Spend); d != want {
			t.Fatalf("case %d: plan deviation %d != optimal %d", tc, d, want)
		}
	}
}
