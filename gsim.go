package gsim

import (
	"math/big"
	"runtime"
	"sync"
)

type node struct {
	n         *big.Int
	depth     int
	value     interface{}
	generator OptionGenerator
	cumuOpts  *big.Int
}

type rootNode struct {
	*node
}

type permN struct {
	perm []interface{}
	n    *big.Int
}

// The OptionGenerator is responsible for generating the next
// available possible paths from each permutation prefix. Two
// implementations of OptionGenerator are provided: simplePermutation
// and graphPermutation. If neither are sufficient for your needs then
// you'll want to implement OptionGenerator yourself.
type OptionGenerator interface {
	// Generate is provided with the previously-chosen option, and is
	// required to return the set of options now available as the next
	// element in the permutation. OptionGenerators are expected to be
	// stateful. Generate must return an empty list for permutation
	// generation to terminate.
	Generate(interface{}) []interface{}
	// Clone is used during permutation generation. If the
	// OptionGenerator is stateful then Clone must return a fresh
	// OptionGenerator which shares no mutable state with the receiver
	// of Clone.
	Clone() OptionGenerator
}

// Permutations allows you to interate through the available
// permutations, and extract specific permutations.
type Permutations interface {
	// Iterate through every permutation in the current go-routine. No
	// parallelism occurs. The function passed is invoked once for
	// every permutation. It is supplied with the permutation number,
	// and the permutation itself. These arguments should be
	// considered read-only. If you mutate the permutation number or
	// permutation then behaviour is undefined.
	ForEach(func(*big.Int, []interface{}))
	// Iterate through every permutation and use concurrency. A number
	// of go-routines will be spawned appropriate for the current
	// value of GOMAXPROCS. These go-routines will be fed batches of
	// permutations and then invoke the function passed for each
	// permutation. It's your responsibility to make sure the function
	// passed is safe to be run concurrently from multiple
	// go-routines.
	//
	// The first argument is the batchsize. If the batchsize is very
	// low, then the generation of permutations will thrash CPU due to
	// contention for locks on channels. If your processing of each
	// permutation is very quick, then high numbers (e.g. 8192) can
	// help to keep your CPU busy. If your processing of each
	// permutation is less quick then lower numbers will avoid memory
	// ballooning. Some trial and error may be worthwhile to find a
	// good number for your computer, but 2048 is a sensible place to
	// start.
	ForEachPar(int, func(*big.Int, []interface{}))
	// Every permutation has a unique number, which is supplied to the
	// function passed in both of the other functions in
	// Permutations. If you need to generate specific permutations,
	// those numbers can be provided to Permutation, which will
	// generate the exact same permutation. Note that iterating
	// through a range of permutation numbers and repeatedly calling
	// Permutation is slower than using either of the iterator
	// functions.
	Permutation(*big.Int) []interface{}
}

var (
	bigIntZero = big.NewInt(0)
	bigIntOne  = big.NewInt(1)
	bigIntTwo  = big.NewInt(2)
)

// Construct a Permutations from an OptionGenerator.
func BuildPermutations(gen OptionGenerator) Permutations {
	cur := &node{
		n:         bigIntZero,
		depth:     0,
		generator: gen,
		cumuOpts:  bigIntOne,
	}
	root := &rootNode{
		node: cur,
	}
	return root
}

func (n *rootNode) ForEachPar(batchSize int, f func(*big.Int, []interface{})) {
	par := runtime.GOMAXPROCS(0) // 0 gets the current count
	var wg sync.WaitGroup
	wg.Add(par)
	ch := make(chan []*permN, par*par)
	for idx := 0; idx < par; idx += 1 {
		go func() {
			defer wg.Done()
			for {
				perms, ok := <-ch
				if !ok {
					return
				}
				for _, perm := range perms {
					f(perm.n, perm.perm)
				}
			}
		}()
	}

	batch := make([]*permN, batchSize)
	batchIdx := 0
	n.ForEach(func(n *big.Int, perm []interface{}) {
		permCopy := make([]interface{}, len(perm))
		copy(permCopy, perm)
		batch[batchIdx] = &permN{n: n, perm: permCopy}
		batchIdx += 1
		if batchIdx == batchSize {
			ch <- batch
			batch = make([]*permN, batchSize)
			batchIdx = 0
		}
	})
	ch <- batch[:batchIdx]
	close(ch)
	wg.Wait()
}

func (r *rootNode) ForEach(f func(*big.Int, []interface{})) {
	perm := []interface{}{}

	worklist := []*node{&node{
		n:         r.node.n,
		depth:     r.node.depth,
		generator: r.node.generator.Clone(),
		cumuOpts:  r.node.cumuOpts,
	}}
	l := len(worklist)

	for l != 0 {
		l -= 1
		cur := worklist[l]
		worklist = worklist[:l]

		perm = append(perm[:cur.depth], cur.value)

		options := cur.generator.Generate(cur.value)
		optionCount := len(options)

		if optionCount == 0 {
			f(cur.n, perm[1:])

		} else {
			cumuOpts := big.NewInt(int64(optionCount))
			cumuOpts.Mul(cur.cumuOpts, cumuOpts)
			for idx, option := range options {
				var childN *big.Int
				if optionCount == 1 {
					childN = cur.n
				} else {
					childN = big.NewInt(int64(idx))
					childN.Mul(childN, cur.cumuOpts)
					childN.Add(childN, cur.n)
				}
				var gen OptionGenerator
				if idx == 0 {
					gen = cur.generator
				} else {
					gen = cur.generator.Clone()
				}
				child := &node{
					n:         childN,
					depth:     cur.depth + 1,
					value:     option,
					generator: gen,
					cumuOpts:  cumuOpts,
				}
				worklist = append(worklist, child)
			}
			l += optionCount
		}
	}
}

func (r *rootNode) Permutation(permNum *big.Int) []interface{} {
	n := big.NewInt(0).Set(permNum)
	perm := []interface{}{}
	choiceBig := big.NewInt(0)

	gen := r.node.generator.Clone()
	val := r.node.value
	for {
		options := gen.Generate(val)
		optionCount := len(options)
		if optionCount == 0 {
			return perm
		}
		choiceBig.SetInt64(int64(optionCount))
		n.QuoRem(n, choiceBig, choiceBig)
		val = options[int(choiceBig.Int64())]
		perm = append(perm, val)
		gen = gen.Clone()
	}
	return perm
}
