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
	// The callback is invoked when the node is not inhibited and an
	// additional incoming edge is reached. The callback controls when
	// the node becomes eligible for selection in the permutation, and
	// when it is excluded from selection.
	Callback GraphNodeCallback
}

type GraphNodeCallback interface {
	IncomingEdgesReached(*GraphNode, []*GraphNode) GraphNodeStateChange
}

type availableAnyCallback struct{}

func (aac *availableAnyCallback) IncomingEdgesReached(*GraphNode, []*GraphNode) GraphNodeStateChange {
	return MakeAvailable
}

// The AvailableAnyCallback is the default callback and will always
// return MakeAvailable. This means as soon as at least one incoming
// edge has been reached, the node becomes available for selection.
var AvailableAnyCallback = &availableAnyCallback{}

type allCallback struct {
	result   GraphNodeStateChange
	required []*GraphNode
}

func newAllCallback(result GraphNodeStateChange, required ...*GraphNode) GraphNodeCallback {
	return &allCallback{
		result:   result,
		required: required,
	}
}
func (ac *allCallback) IncomingEdgesReached(node *GraphNode, reached []*GraphNode) GraphNodeStateChange {
	if len(reached) >= len(ac.required) {
		// we required that everything in ac.required is in reached. Reached can be bigger.
		for _, reqNode := range ac.required {
			found := false
			for _, reachedNode := range reached {
				if found = reqNode == reachedNode; found {
					break
				}
			}
			if !found {
				return NoChange
			}
		}
		return ac.result
	}
	return NoChange
}

// The AvailableAllCallback returns MakeAvailable only when all of the
// nodes supplied to the constructor are found in the reached incoming
// edges. It never returns Inhibit.
type AvailableAllCallback GraphNodeCallback

func NewAvailableAllCallback(required ...*GraphNode) AvailableAllCallback {
	return (AvailableAllCallback)(newAllCallback(MakeAvailable, required...))
}

type inhibitAnyCallback struct{}

func (iac *inhibitAnyCallback) IncomingEdgesReached(*GraphNode, []*GraphNode) GraphNodeStateChange {
	return Inhibit
}

// The InhibitAnyCallback is the inhibit equivalent to
// AvailableAnyCallback. It returns Inhibit as soon as at least one
// incoming edge has been reached.
var InhibitAnyCallback = &inhibitAnyCallback{}

// The InhibitAllCallback is the inhibit equivalent to
// AvailableAllCallback. It returns Inhibit only when all of the nodes
// supplied to the constructor are found in the reached incoming
// edges. It never returns MakeAvailable.
type InhibitAllCallback GraphNodeCallback

func NewInhibitAllCallback(required ...*GraphNode) InhibitAllCallback {
	return (InhibitAllCallback)(newAllCallback(Inhibit, required...))
}

// The CombinationCallback allows you to attach several callbacks to a
// graphnode. This is very useful for more complex and layer logic
// surrounding when a node becomes available for selection or
// inhibited. Each callback produces a GraphNodeStateChange answer,
// and it is then up to the combiner function to combine all those
// answers. The combiner is provided with the node, the reached
// incoming edges, the list of callbacks, and the corresponding list
// of their answers. It is in fact perfectly possible to have
// combinations of combinations, should you really need to!
type CombinationCallback struct {
	callbacks []GraphNodeCallback
	combiner  CombinationCallbackCombiner
}

type CombinationCallbackCombiner func(*GraphNode, []*GraphNode, []GraphNodeCallback, []GraphNodeStateChange) GraphNodeStateChange

func NewCombinationCallback(combiner CombinationCallbackCombiner) *CombinationCallback {
	return &CombinationCallback{
		combiner: combiner,
	}
}

func (cc *CombinationCallback) AddCallback(callback GraphNodeCallback) {
	cc.callbacks = append(cc.callbacks, callback)
}

func (cc *CombinationCallback) IncomingEdgesReached(node *GraphNode, reached []*GraphNode) GraphNodeStateChange {
	results := make([]GraphNodeStateChange, len(cc.callbacks))
	for idx, callback := range cc.callbacks {
		results[idx] = callback.IncomingEdgesReached(node, reached)
	}
	return cc.combiner(node, reached, cc.callbacks, results)
}

// InhibitThenAvailableCombiner is a CombinationCallbackCombiner. Its
// semantics are that if any callback returns Inhibit, then the result
// is Inhibit. If no callback returns Inhibit, and at least one
// callback returns MakeAvailable then the result in
// MakeAvailable. Otherwise the result is NoChange.
func InhibitThenAvailableCombiner(node *GraphNode, reached []*GraphNode, callbacks []GraphNodeCallback, results []GraphNodeStateChange) GraphNodeStateChange {
	finalResult := GraphNodeStateChange(NoChange)
	for _, result := range results {
		switch result {
		case Inhibit:
			return Inhibit
		case MakeAvailable:
			finalResult = MakeAvailable
		}
	}
	return finalResult
}

type GraphNodeStateChange interface {
	graphNodeStateChangeWitness()
}

type noChange struct{}

func (nc *noChange) graphNodeStateChangeWitness() {}
func (nc *noChange) String() string               { return "NoChange" }

type makeAvailable struct{}

func (ma *makeAvailable) graphNodeStateChangeWitness() {}
func (ma *makeAvailable) String() string               { return "MakeAvailable" }

type inhibit struct{}

func (i *inhibit) graphNodeStateChangeWitness() {}
func (i *inhibit) String() string               { return "Inhibit" }

var (
	NoChange      = &noChange{}
	MakeAvailable = &makeAvailable{}
	Inhibit       = &inhibit{}
)

// Construct a new GraphNode. The node will start with no edges,
// and Callback is AvailableAnyCallback.
func NewGraphNode(value interface{}) *GraphNode {
	return &GraphNode{
		Value:    value,
		Out:      []*GraphNode{},
		In:       []*GraphNode{},
		Callback: AvailableAnyCallback,
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
	parent    *graphPermutation
	current   []interface{}
	nodeState map[interface{}]*graphNodeState
}

type graphNodeState struct {
	*GraphNode
	permutation     *graphPermutation
	inhibited       bool
	available       bool
	incomingVisited []*GraphNode
}

func (gns *graphNodeState) Clone(gp *graphPermutation) *graphNodeState {
	if gns.permutation == gp {
		return gns
	}
	gns2 := &graphNodeState{
		GraphNode:       gns.GraphNode,
		permutation:     gp,
		inhibited:       gns.inhibited,
		available:       gns.available,
		incomingVisited: make([]*GraphNode, len(gns.incomingVisited)),
	}
	copy(gns2.incomingVisited, gns.incomingVisited)
	gp.nodeState[gns2.GraphNode] = gns2
	return gns2
}

// Create a OptionGenerator for the given graphs. Note the starting
// nodes may both be from the same graph (useful if you don't know
// what the first event will be), or from multiple disjoint graphs, or
// any combination.
func NewGraphPermutation(startingNode ...*GraphNode) OptionGenerator {
	current := make([]interface{}, len(startingNode))
	nodeState := make(map[interface{}]*graphNodeState, len(startingNode))
	gp := &graphPermutation{
		current:   current,
		nodeState: nodeState,
	}
	for idx, gn := range startingNode {
		current[idx] = gn
		nodeState[gn] = &graphNodeState{
			GraphNode:       gn,
			permutation:     gp,
			inhibited:       false,
			available:       true,
			incomingVisited: make([]*GraphNode, 0, len(gn.In)),
		}
	}
	return gp
}

func (gp *graphPermutation) Clone() OptionGenerator {
	current := make([]interface{}, len(gp.current))
	copy(current, gp.current)
	return &graphPermutation{
		parent:    gp,
		current:   current,
		nodeState: make(map[interface{}]*graphNodeState, len(gp.nodeState)),
	}
}

func (gp *graphPermutation) getNodeState(node interface{}, cloneToLocal bool) (*graphNodeState, bool) {
	if gns, found := gp.nodeState[node]; found {
		return gns.Clone(gp), found
	} else if gp.parent == nil {
		return nil, found
	} else if gns, found = gp.parent.getNodeState(node, false); found && cloneToLocal {
		return gns.Clone(gp), found
	} else if found {
		gp.nodeState[node] = gns
		return gns, found
	} else {
		return nil, false
	}
}

func (gp *graphPermutation) Generate(lastChosen interface{}) []interface{} {
	if lastChosen != nil {
		lastChosenState, _ := gp.getNodeState(lastChosen, true)
		lastChosenState.inhibited = true
		for idx, node := range gp.current {
			if node == lastChosen {
				gp.current = append(gp.current[:idx], gp.current[idx+1:]...)
				break
			}
		}

		for _, gn := range lastChosenState.Out {
			nodeState, found := gp.getNodeState(gn, false)

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
					nodeState = nodeState.Clone(gp)
					nodeState.incomingVisited = append(nodeState.incomingVisited, lastChosenState.GraphNode)
				}

			default:
				dirty = true
				nodeState = &graphNodeState{
					GraphNode:       gn,
					permutation:     gp,
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

			switch nodeState.Callback.IncomingEdgesReached(nodeState.GraphNode, nodeState.incomingVisited) {
			case Inhibit:
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
			case MakeAvailable:
				if !nodeState.available {
					nodeState.available = true
					gp.current = append(gp.current, nodeState.GraphNode)
				}
			}
		}
	}
	return gp.current
}

func (gn *GraphNode) String() string {
	return fmt.Sprintf("GraphNode with value %v", gn.Value)
}
