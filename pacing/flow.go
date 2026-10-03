package pacing

// 本文件实现给定最大偏差 dev 时的可行性判定。
//
// 判定模型（循环流，边带下界）：
//   每个活动 i 是一条时间链，边 u[i][d] -> u[i][d+1] 承载第 d 天末的累计支出 S_i(d)，
//   上下界为 [max(A_i, T_i(d)-dev), min(B_i, T_i(d)+dev)]；
//   源 S -> u[i][k+1] 固定注入历史累计 A_i；u[i][N] -> 汇 T 固定取出总预算 B_i；
//   日节点 D_d 由 S 供给当日共享上限 cap_d，D_d -> u[i][d] 注入当日支出 x[i][d]
//   （暂停日不加此边）；T -> S 闭合循环。
// 下界通过超级源汇归约：可行当且仅当超级源出边全部饱和。
// 所有容量为整数，最大流整数定理保证存在整数解，直接读出 x[i][d]。

const infCap = int64(1) << 60

type mfEdge struct {
	to  int
	rev int
	cap int64
}

type maxFlow struct {
	g     [][]mfEdge
	level []int
	iter  []int
}

func newMaxFlow(n int) *maxFlow {
	return &maxFlow{g: make([][]mfEdge, n), level: make([]int, n), iter: make([]int, n)}
}

func (mf *maxFlow) addEdge(from, to int, cap int64) int {
	mf.g[from] = append(mf.g[from], mfEdge{to: to, rev: len(mf.g[to]), cap: cap})
	mf.g[to] = append(mf.g[to], mfEdge{to: from, rev: len(mf.g[from]) - 1, cap: 0})
	return len(mf.g[from]) - 1
}

func (mf *maxFlow) bfs(s, t int) bool {
	for i := range mf.level {
		mf.level[i] = -1
	}
	queue := make([]int, 0, len(mf.g))
	mf.level[s] = 0
	queue = append(queue, s)
	for head := 0; head < len(queue); head++ {
		v := queue[head]
		for _, e := range mf.g[v] {
			if e.cap > 0 && mf.level[e.to] < 0 {
				mf.level[e.to] = mf.level[v] + 1
				queue = append(queue, e.to)
			}
		}
	}
	return mf.level[t] >= 0
}

func (mf *maxFlow) dfs(v, t int, f int64) int64 {
	if v == t {
		return f
	}
	for ; mf.iter[v] < len(mf.g[v]); mf.iter[v]++ {
		i := mf.iter[v]
		e := &mf.g[v][i]
		if e.cap > 0 && mf.level[v] < mf.level[e.to] {
			d := mf.dfs(e.to, t, min64(f, e.cap))
			if d > 0 {
				e.cap -= d
				mf.g[e.to][e.rev].cap += d
				return d
			}
		}
	}
	return 0
}

func (mf *maxFlow) maxFlow(s, t int) int64 {
	flow := int64(0)
	for mf.bfs(s, t) {
		for i := range mf.iter {
			mf.iter[i] = 0
		}
		for {
			f := mf.dfs(s, t, infCap)
			if f == 0 {
				break
			}
			flow += f
		}
	}
	return flow
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// feasibleWithDeviation 判定在给定最大偏差 dev 下是否存在可行计划。
// 若可行，返回每个活动每个未来日的支出 spend[i][j]；否则返回 nil。
func feasibleWithDeviation(inst *instance, dev int64) [][]int64 {
	m := len(inst.budget)
	f := inst.future
	k := inst.settled

	// 节点编号：0=S, 1=T, 2..2+f-1=日节点, 之后为 u[i][j]（未来第 j 天，对应第 k+1+j 天）
	dayNode := func(j int) int { return 2 + j }
	uNode := func(i, j int) int { return 2 + f + i*f + j }
	numBase := 2 + f + m*f
	ss := numBase
	tt := numBase + 1
	mf := newMaxFlow(numBase + 2)

	const S, T = 0, 1
	demand := make([]int64, numBase+2) // 下界产生的节点需求：入下界-出下界
	// addLower 登记带下界 [lo,hi] 的边，返回变换后边（容量 hi-lo）的索引。
	addLower := func(from, to int, lo, hi int64) int {
		demand[to] += lo
		demand[from] -= lo
		return mf.addEdge(from, to, hi-lo)
	}

	// 注入边 D_d -> u[i][j]，用于读出计划支出。
	injectEdge := make([][]int, m)
	for i := range injectEdge {
		injectEdge[i] = make([]int, f)
		for j := range injectEdge[i] {
			injectEdge[i][j] = -1
		}
	}

	for i := 0; i < m; i++ {
		a := inst.actual[i]
		b := inst.budget[i]
		// S -> u[i][0] 固定注入历史累计 A_i
		addLower(S, uNode(i, 0), a, a)
		for j := 0; j < f; j++ {
			d := k + 1 + j // 1-based 日
			lo := max64(a, inst.target[i][d]-dev)
			hi := min64(b, inst.target[i][d]+dev)
			if lo > hi {
				return nil
			}
			// 承载 S_i(d) 的出边
			if j+1 < f {
				addLower(uNode(i, j), uNode(i, j+1), lo, hi)
			} else {
				// 末日累计恒等于总预算
				if b < lo || b > hi {
					return nil
				}
				addLower(uNode(i, j), T, b, b)
			}
			// 当日支出注入边（暂停日不加）
			if !inst.paused[i][j] {
				injectEdge[i][j] = addLower(dayNode(j), uNode(i, j), 0, infCap)
			}
		}
	}
	for j := 0; j < f; j++ {
		addLower(S, dayNode(j), 0, inst.caps[j])
	}
	mf.addEdge(T, S, infCap)

	// 超级源汇归约
	total := int64(0)
	for v := 0; v < numBase; v++ {
		if demand[v] > 0 {
			mf.addEdge(ss, v, demand[v])
			total += demand[v]
		} else if demand[v] < 0 {
			mf.addEdge(v, tt, -demand[v])
		}
	}
	if mf.maxFlow(ss, tt) != total {
		return nil
	}

	spend := make([][]int64, m)
	for i := 0; i < m; i++ {
		spend[i] = make([]int64, f)
		for j := 0; j < f; j++ {
			if ei := injectEdge[i][j]; ei >= 0 {
				e := mf.g[dayNode(j)][ei]
				spend[i][j] = mf.g[e.to][e.rev].cap // 反向边残量即正向流量
			}
		}
	}
	return spend
}
