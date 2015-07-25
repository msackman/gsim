package main

import (
	"fmt"
	"github.com/msackman/gsim"
	"math/big"
	"runtime"
)

type simpleConsumer struct{}

func (sc *simpleConsumer) Clone() gsim.PermutationConsumer {
	return sc
}

func (sc *simpleConsumer) Consume(n *big.Int, perm []interface{}) {
	fmt.Println(n, perm)
}

func main() {
	runtime.GOMAXPROCS(2 * runtime.NumCPU())

	consumer := &simpleConsumer{}

	simplePerms(consumer)
	graphPerms(consumer)
}

func graphPerms(consumer gsim.PermutationConsumer) {
	// no deps, so exactly the same as a SimplePermutation
	c1 := gsim.NewGraphNode("a")
	c2 := gsim.NewGraphNode("b")
	c3 := gsim.NewGraphNode("c")
	c4 := gsim.NewGraphNode("d")
	runPerms(consumer, gsim.NewGraphPermutation(c1, c2, c3, c4))

	// simple dependency
	b1 := gsim.NewGraphNode("B1")
	b2 := gsim.NewGraphNode("B2")
	b1.AddEdgeTo(b2)
	runPerms(consumer, gsim.NewGraphPermutation(b1))

	// more complex dependencies:
	//
	// A1----A3
	//    \ /   \
	//     X     A5
	//    / \   /
	// A2----A4
	//
	a1 := gsim.NewGraphNode("A1")
	a2 := gsim.NewGraphNode("A2")
	a3 := gsim.NewGraphNode("A3")
	a4 := gsim.NewGraphNode("A4")
	a5 := gsim.NewGraphNode("A5")
	a1.AddEdgeTo(a3)
	a1.AddEdgeTo(a4)
	a2.AddEdgeTo(a3)
	a2.AddEdgeTo(a4)
	a3.AddEdgeTo(a5)
	a4.AddEdgeTo(a5)
	a3.AvailableOn = gsim.ConditionAll
	a4.AvailableOn = gsim.ConditionAll
	a5.AvailableOn = gsim.ConditionAll
	runPerms(consumer, gsim.NewGraphPermutation(a1, a2))

	// by not setting d3 to a join, it can appear after any enabling
	// node is visited.
	d1 := gsim.NewGraphNode("D1")
	d2 := gsim.NewGraphNode("D2")
	d3 := gsim.NewGraphNode("D3")
	d1.AddEdgeTo(d3)
	d2.AddEdgeTo(d3)
	runPerms(consumer, gsim.NewGraphPermutation(d1, d2))

	// e3 can only be reached once e1 and e2 have been reached
	// once e3 has been reached, e4 must not be reached
	e1 := gsim.NewGraphNode("E1")
	e2 := gsim.NewGraphNode("E2")
	e3 := gsim.NewGraphNode("E3")
	e4 := gsim.NewGraphNode("E4")
	e1.AddEdgeTo(e3)
	e2.AddEdgeTo(e3)
	e3.AvailableOn = gsim.ConditionAll
	e1.AddEdgeTo(e4)
	e2.AddEdgeTo(e4)
	e3.AddEdgeTo(e4)
	e4.InhibitOn = gsim.Condition(
		func(node *gsim.GraphNode, incomingVisited []*gsim.GraphNode) bool {
			for _, gn := range incomingVisited {
				if gn == e3 {
					return true
				}
			}
			return false
		})
	runPerms(consumer, gsim.NewGraphPermutation(e1, e2))
}

func simplePerms(consumer gsim.PermutationConsumer) {
	runPerms(consumer, gsim.NewSimplePermutation([]interface{}{"a", "b", "c", "d", "e"}))
}

func runPerms(consumer gsim.PermutationConsumer, og gsim.OptionGenerator) {
	gsim.BuildPermutations(og).ForEachPar(8192, consumer)
}
