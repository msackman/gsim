package main

import (
	"fmt"
	"github.com/msackman/gsim"
	"math/big"
	"runtime"
)

func main() {
	runtime.GOMAXPROCS(2 * runtime.NumCPU())

	simplePerms()
	graphPerms()
}

func graphPerms() {
	// no deps, so exactly the same as a SimplePermutation
	c1 := gsim.NewGraphNode("a")
	c2 := gsim.NewGraphNode("b")
	c3 := gsim.NewGraphNode("c")
	c4 := gsim.NewGraphNode("d")
	runPerms(gsim.NewGraphPermutation(c1, c2, c3, c4))

	// simple dependency
	b1 := gsim.NewGraphNode("B1")
	b2 := gsim.NewGraphNode("B2")
	b1.AddEdgeTo(b2)
	runPerms(gsim.NewGraphPermutation(b1))

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
	a3.SetJoins(true)
	a4.SetJoins(true)
	a5.SetJoins(true)
	runPerms(gsim.NewGraphPermutation(a1, a2))

	// by not setting d3 to a join, it can appear after any enabling
	// node is visited.
	d1 := gsim.NewGraphNode("D1")
	d2 := gsim.NewGraphNode("D2")
	d3 := gsim.NewGraphNode("D3")
	d1.AddEdgeTo(d3)
	d2.AddEdgeTo(d3)
	runPerms(gsim.NewGraphPermutation(d1, d2))
}

func simplePerms() {
	runPerms(gsim.NewSimplePermutation([]interface{}{"a", "b", "c", "d", "e"}))
}

func runPerms(og gsim.OptionGenerator) {
	perms := gsim.BuildPermutations(og)
	perms.ForEachPar(8192, func(n *big.Int, perm []interface{}) {
		fmt.Println(n, perm)
	})
}
