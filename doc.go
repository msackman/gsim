// Package gsim provides the ability to exhaustively iterate through
// every permutation of a graph, to facilitate exhaustive simulation.
//
// The idea is that you might be trying to simulate some sort of
// concurrent system. Various events can occur within the system as a
// whole but there are constraints on the possible order in which
// things can occur. These constraints you can describe as a graph (or
// series of graphs). This package will then exhaustively produce
// every possible serial order of events.
//
// What you do with such permutations is entirely up to you. One idea
// is you might consider those events as instructions, and write an
// interpreter to process those instructions. Then at the end, you can
// check to see if goals have been met and invariants kept. If they
// have been, you have performed a model check that with any valid
// sequence of instructions as generated from the graph, your
// algorithm is correct.
//
// An alternative use is for testing: the permutation represents the
// order of events which you send to some black-box system to be
// tested. You then write your own simple interpreter, if necessary,
// and then perform some sort of equivalence checking on the final
// states of the black-box system and your simple interpreter.
//
// A fair amount of effort has been spent in trying to keep the
// permutation generator both fast, and with minimal memory use. That
// said, in all likelihood it will never compete with other tools such
// as TLA+, but it allows you to write your models in Go and thus
// there'll be less of a gap between your model code and your real
// implementation.
//
// See https://github.com/msackman/gsim/blob/master/main/main.go for
// examples.
package gsim
