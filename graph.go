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
// Each generated permutation will contain each node no more than
// once. Cycles in the graph are eliminated. Edges between nodes can
// make the target node eligible for selection in the permutation, or
// excluded from selection.
//
// When generating the graph programmatically, you must also ensure
// even your generation of the graph is deterministic - i.e. the order
// in which edges are added to nodes should be the same for multiple
// iterations. If they are not, then you will find permutation numbers
// differ between iterations.
type GraphNode struct {
	// The value you've provided to this GraphNode.
	Value interface{}
	// The outgoing edges from this node. Treat this field as read-only
	// and use the AddEdgeTo method to add edges.
	Out []*GraphNode
	// The incoming edges from this node. Treat this field as read-only
	// and use the AddEdgeTo method to add edges.
	In []*GraphNode
	// The condition under which a node will never be eligible for
	// selection in the permutation. By default, this is
	// ConditionNever.
	InhibitOn Condition
	// The condition under which a node becomes eligible for selection
	// in the permutation. By default, this is ConditionAny.
	AvailableOn Condition
}

// Construct a new GraphNode. The node will start with no edges,
// AvailableOn is ConditionAny and InhibitOn is ConditionNever.
func NewGraphNode(value interface{}) *GraphNode {
	return &GraphNode{
		Value:       value,
		Out:         []*GraphNode{},
		In:          []*GraphNode{},
		InhibitOn:   ConditionNever,
		AvailableOn: ConditionAny,
	}
}

// Add an edge from the receiver to the argument. This is idempotent.
func (gn *GraphNode) AddEdgeTo(gn2 *GraphNode) {
	if !containsGraphNode(gn.Out, gn2) {
		gn.Out = append(gn.Out, gn2)
	}
	if !containsGraphNode(gn2.In, gn) {
		gn2.In = append(gn2.In, gn)
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
	current   []interface{}
	nodeState map[interface{}]*graphNodeState
}

type graphNodeState struct {
	*GraphNode
	inhibited       bool
	available       bool
	incomingVisited []*GraphNode
}

func (gns *graphNodeState) Clone() *graphNodeState {
	gns2 := &graphNodeState{
		GraphNode:       gns.GraphNode,
		inhibited:       gns.inhibited,
		available:       gns.available,
		incomingVisited: make([]*GraphNode, len(gns.incomingVisited)),
	}
	copy(gns2.incomingVisited, gns.incomingVisited)
	return gns2
}

// Create a OptionGenerator for the given graphs. Note the starting
// nodes may both be from the same graph (useful if you don't know
// what the first event will be), or from multiple disjoint graphs, or
// any combination.
func NewGraphPermutation(startingNode ...*GraphNode) OptionGenerator {
	current := make([]interface{}, len(startingNode))
	nodeState := make(map[interface{}]*graphNodeState)
	for idx, gn := range startingNode {
		current[idx] = gn
		nodeState[gn] = &graphNodeState{
			GraphNode:       gn,
			inhibited:       false,
			available:       true,
			incomingVisited: make([]*GraphNode, 0, len(gn.In)),
		}
	}
	return &graphPermutation{
		current:   current,
		nodeState: nodeState,
	}
}

func (gp *graphPermutation) Clone() OptionGenerator {
	current := make([]interface{}, len(gp.current))
	copy(current, gp.current)
	nodeState := make(map[interface{}]*graphNodeState)
	for gn, gns := range gp.nodeState {
		nodeState[gn] = gns.Clone()
	}
	return &graphPermutation{
		current:   current,
		nodeState: nodeState,
	}
}

func (gp *graphPermutation) Generate(lastChosen interface{}) []interface{} {
	if lastChosen != nil {
		lastChosenState := gp.nodeState[lastChosen]
		lastChosenState.inhibited = true
		for idx, node := range gp.current {
			if node == lastChosen {
				gp.current = append(gp.current[:idx], gp.current[idx+1:]...)
				break
			}
		}

		for _, gn := range lastChosenState.Out {
			nodeState, found := gp.nodeState[gn]

			dirty := false
			switch {
			case found && nodeState.inhibited:
				continue

			case found:
				found = false
				for _, node := range nodeState.incomingVisited {
					if found = node == lastChosenState.GraphNode; found {
						break
					}
				}
				if !found {
					dirty = true
					nodeState.incomingVisited = append(nodeState.incomingVisited, lastChosenState.GraphNode)
				}

			default:
				dirty = true
				nodeState = &graphNodeState{
					GraphNode:       gn,
					inhibited:       false,
					available:       false,
					incomingVisited: make([]*GraphNode, 1, len(gn.In)),
				}
				nodeState.incomingVisited[0] = lastChosenState.GraphNode
				gp.nodeState[gn] = nodeState
			}

			if !dirty {
				continue
			}

			if nodeState.InhibitOn(nodeState.GraphNode, nodeState.incomingVisited) {
				if nodeState.available {
					nodeState.available = false
					for idx, node := range gp.current {
						if node == nodeState.GraphNode {
							gp.current = append(gp.current[:idx], gp.current[idx+1:]...)
							break
						}
					}
				}
				nodeState.inhibited = true
				continue
			}

			if nodeState.available {
				continue
			}
			if nodeState.AvailableOn(nodeState.GraphNode, nodeState.incomingVisited) {
				nodeState.available = true
				gp.current = append(gp.current, nodeState.GraphNode)
			}
		}
	}
	return gp.current
}

func (gn *GraphNode) String() string {
	return fmt.Sprintf("GraphNode with value %v", gn.Value)
}

// Conditions are used to control the circumstances under which a node
// which has at least one incoming edge reached becomes either
// available to selection or inhibited from ever being selection. If
// you implement your own, make sure they are pure functions. The
// arguments are the node in question, and the slice of incoming nodes
// which have been visited. It is guaranteed this list does not
// contain duplicates. Note once a node is inhibited, it cannot be
// visited.
type Condition func(node *GraphNode, incomingVisited []*GraphNode) bool

var (
	// ConditionNever always returns false and is the default value for
	// InhibitOn. Thus by default nodes are never eliminated from
	// selection (until of course they've been visited and included in
	// the current permutation).
	ConditionNever = func(node *GraphNode, visited []*GraphNode) bool {
		return false
	}
	// ConditionAny returns true provided the list of visited nodes is
	// at least one item long. This is the default value for
	// AvailableOn. Thus by default nodes become available for
	// selection as soon as any of their incoming edges are reached.
	ConditionAny = func(node *GraphNode, visited []*GraphNode) bool {
		return len(visited) != 0
	}
	// ConditionAll returns true provided the list of visited nodes is
	// of the same length as (and thus setwise-equal to) the list of
	// incoming edges to the node. If used as the AvailableOn
	// condition, this will make the node available for inclusion in
	// the permutation only once all the incoming edges have been
	// visited.
	ConditionAll = func(node *GraphNode, visited []*GraphNode) bool {
		return len(visited) == len(node.In)
	}
)
