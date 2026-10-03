package pacing

// lbEdge is a directed edge with a lower and upper bound on its flow.
type lbEdge struct {
	from, to int
	lo, hi   int64
	id       int
}

// dinic is a standard Dinic max-flow over int64 capacities. Edges are
// stored in flat slices; head holds the first edge index per node (-1 for
// none) and next chains the adjacency list.
type dinic struct {
	head []int
	next []int
	to   []int
	cap  []int64
}

func newDinic(numNodes int) *dinic {
	head := make([]int, numNodes)
	for i := range head {
		head[i] = -1
	}
	return &dinic{head: head}
}

// addEdge appends a directed edge and returns the index of its forward
// arc in the capacity slice. The reverse arc occupies index^1.
func (d *dinic) addEdge(from, to int, capacity int64) int {
	idx := len(d.to)
	d.to = append(d.to, to)
	d.cap = append(d.cap, capacity)
	d.next = append(d.next, d.head[from])
	d.head[from] = idx
	d.to = append(d.to, from)
	d.cap = append(d.cap, 0)
	d.next = append(d.next, d.head[to])
	d.head[to] = idx + 1
	return idx
}

func (d *dinic) bfsLevels(s, t int, level []int) bool {
	for i := range level {
		level[i] = -1
	}
	queue := make([]int, 0, len(level))
	level[s] = 0
	queue = append(queue, s)
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		for e := d.head[u]; e >= 0; e = d.next[e] {
			if d.cap[e] > 0 && level[d.to[e]] < 0 {
				level[d.to[e]] = level[u] + 1
				queue = append(queue, d.to[e])
			}
		}
	}
	return level[t] >= 0
}

func (d *dinic) sendFlow(u, t int, f int64, level, iter []int) int64 {
	if u == t {
		return f
	}
	for ; iter[u] >= 0; iter[u] = d.next[iter[u]] {
		e := iter[u]
		v := d.to[e]
		if d.cap[e] > 0 && level[v] == level[u]+1 {
			pushed := d.sendFlow(v, t, min64(f, d.cap[e]), level, iter)
			if pushed > 0 {
				d.cap[e] -= pushed
				d.cap[e^1] += pushed
				return pushed
			}
		}
	}
	return 0
}

// maxFlow computes the max flow from s to t, mutating capacities.
func (d *dinic) maxFlow(s, t int) int64 {
	n := len(d.head)
	level := make([]int, n)
	iter := make([]int, n)
	var total int64
	for d.bfsLevels(s, t, level) {
		for i := range iter {
			iter[i] = d.head[i]
		}
		for {
			pushed := d.sendFlow(s, t, 1<<62, level, iter)
			if pushed == 0 {
				break
			}
			total += pushed
		}
	}
	return total
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// feasibleFlow decides whether a feasible flow exists on the given graph
// with per-edge lower/upper bounds and, if so, returns the flow value of
// every edge that was given a non-negative id.
//
// It uses the classic lower-bound reduction: each edge lo<=f<=hi is
// replaced by capacity hi-lo, node imbalances are recorded, and a super
// source/sink pair checks that all demands can be satisfied.
func feasibleFlow(numNodes int, edges []lbEdge) ([]int64, bool) {
	ss := numNodes
	tt := numNodes + 1
	d := newDinic(numNodes + 2)
	demand := make([]int64, numNodes)
	edgeIdx := make([]int, len(edges))
	maxID := -1
	for i, e := range edges {
		if e.lo > e.hi {
			return nil, false
		}
		edgeIdx[i] = d.addEdge(e.from, e.to, e.hi-e.lo)
		demand[e.from] -= e.lo
		demand[e.to] += e.lo
		if e.id > maxID {
			maxID = e.id
		}
	}
	var total int64
	for v := 0; v < numNodes; v++ {
		if demand[v] > 0 {
			d.addEdge(ss, v, demand[v])
			total += demand[v]
		} else if demand[v] < 0 {
			d.addEdge(v, tt, -demand[v])
		}
	}
	if d.maxFlow(ss, tt) != total {
		return nil, false
	}
	flows := make([]int64, maxID+1)
	for i, e := range edges {
		if e.id < 0 {
			continue
		}
		used := (e.hi - e.lo) - d.cap[edgeIdx[i]]
		flows[e.id] = e.lo + used
	}
	return flows, true
}
