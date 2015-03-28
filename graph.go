package gsim

import (
	"fmt"
)

// GraphNodes allow you to construct arbitrary graphs which can be
// used to describe the dependencies between different events
// occurring, for example. As usual with graph libraries, create all
// the nodes you need first, and then link them together with edges.
//
// Permutations generated will be lists of GraphNodes. Access the
// value you supplied through the Value field.
//
// It is your responsibility to ensure that the graphs constructed
// contain no cycles. If they contain cycles, permutation generation
// may never terminate. In this way, it is not really suitable for
// modelling infinite state machines (tools like TLA+ are better
// suited for this). It's more suited for permutating finite sets of
// external inputs to a system, with dependencies between those
// inputs.
//
// You must also ensure even your generation of the graph is
// deterministic - i.e. the order in which edges are added to nodes
// should be the same for multiple iterations. If they are not, then
// you will find permutation numbers differ between iterations.
type GraphNode struct {
	// The value you've provided to this GraphNode.
	Value interface{}
	out   []*GraphNode
	in    []*GraphNode
	join  bool
}

// Construct a new GraphNode. The node will start with no edges, and
// will not be marked as a join node.
func NewGraphNode(value interface{}) *GraphNode {
	return &GraphNode{
		Value: value,
		out:   []*GraphNode{},
		in:    []*GraphNode{},
		join:  false,
	}
}

// A join-node is a node which may not be visited until all of its
// incoming edges have been visited. Joins indicates whether or not
// the current node is a join node.
func (gn *GraphNode) Joins() bool {
	return gn.join
}

// A join-node is a node which may not be visited until all of its
// incoming edges have been visited. SetJoins allows you to configure
// whether or not the current node is a join node.
//
// Consider you have three events, A B and C. A and B can be chosen in
// any order, but C can only occur once A and B have occurred. To
// model this, create three nodes, A B and C, add edges from A to C
// and B to C, and set C to be a join-node.
//
// If a node has multiple incoming edges and it's not set as a join
// node then it will still only appear once in any permutation, but
// can appear straight after any node which has an edge to it is
// reached. For example, if you have three nodes, A B and C, and edges
// A to C and B to C and you don't set C as a join-node, then C will
// still never appear first, but it can appear as soon as A or B have
// been chosen, and it will only appear once. So the permutations will
// be A B C; B A C; A C B; B C A.
func (gn *GraphNode) SetJoins(join bool) {
	gn.join = join
}

// Add an edge from the receiver to the argument. This is idempotent.
func (gn *GraphNode) AddEdgeTo(gn2 *GraphNode) {
	if !containsGraphNode(gn.out, gn2) {
		gn.out = append(gn.out, gn2)
	}
	if !containsGraphNode(gn2.in, gn) {
		gn2.in = append(gn2.in, gn)
	}
}

func containsGraphNode(gns []*GraphNode, gn *GraphNode) bool {
	for _, elem := range gns {
		if elem == gn {
			return true
		}
	}
	return false
}

type graphPermutation struct {
	current       []interface{}
	currentMap    map[*GraphNode]bool
	joinNodeState map[*GraphNode]map[*GraphNode]bool
}

// Create a OptionGenerator for the given graphs. Note the starting
// nodes may both be from the same graph (useful if you don't know
// what the first event will be), or from multiple disjoint graphs, or
// any combination.
func NewGraphPermutation(startingNode ...*GraphNode) OptionGenerator {
	current := make([]interface{}, len(startingNode))
	currentMap := make(map[*GraphNode]bool)
	for idx, gn := range startingNode {
		current[idx] = gn
		currentMap[gn] = true
	}
	return &graphPermutation{
		current:       current,
		currentMap:    currentMap,
		joinNodeState: make(map[*GraphNode]map[*GraphNode]bool),
	}
}

func (gp *graphPermutation) Clone() OptionGenerator {
	current := make([]interface{}, len(gp.current))
	copy(current, gp.current)
	currentMap := make(map[*GraphNode]bool)
	for gn, _ := range gp.currentMap {
		currentMap[gn] = true
	}
	jns := make(map[*GraphNode]map[*GraphNode]bool)
	for gn, gns := range gp.joinNodeState {
		gns2 := make(map[*GraphNode]bool)
		for arrived, _ := range gns {
			gns2[arrived] = true
		}
		jns[gn] = gns2
	}
	return &graphPermutation{
		current:       current,
		currentMap:    currentMap,
		joinNodeState: jns,
	}
}

var nonJoinVisitedToken = make(map[*GraphNode]bool)

func (gp *graphPermutation) Generate(lastChosen interface{}) []interface{} {
	if lastChosen != nil {
		lastChosenGraphNode := lastChosen.(*GraphNode)
		for idx, gn := range gp.current {
			if gn == lastChosenGraphNode {
				gp.current = append(gp.current[:idx], gp.current[idx+1:]...)
				delete(gp.currentMap, lastChosenGraphNode)
				break
			}
		}

		for _, gn := range lastChosenGraphNode.out {
			if _, found := gp.currentMap[gn]; !found {
				if gn.Joins() {
					var gns map[*GraphNode]bool
					if gns, found = gp.joinNodeState[gn]; !found {
						gns = make(map[*GraphNode]bool)
						gp.joinNodeState[gn] = gns
					}

					gns[lastChosenGraphNode] = true

					if len(gns) == len(gn.in) {
						gp.currentMap[gn] = true
						gp.current = append(gp.current, gn)
						delete(gp.joinNodeState, gn)
					}

				} else {
					if _, found = gp.joinNodeState[gn]; !found {
						gp.currentMap[gn] = true
						gp.current = append(gp.current, gn)
						gp.joinNodeState[gn] = nonJoinVisitedToken
					}
				}
			}
		}
	}
	return gp.current
}

func (gn *GraphNode) String() string {
	return fmt.Sprintf("GraphNode with value %v", gn.Value)
}
