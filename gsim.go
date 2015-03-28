package gsim

import (
	"math/big"
	"runtime"
	"sync"
)

// The OptionGenerator is responsible for generating the next
// available possible paths from each permutation prefix. Two
// implementations of OptionGenerator are provided: simplePermutation
// and graphPermutation. If neither are sufficient for your needs then
// you'll want to implement OptionGenerator yourself.
//
// If you do implement OptionGenerator yourself, you must ensure it is
// entirely deterministic. So do not rely on iteration order of maps
// and so forth.
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

type node struct {
	n         *big.Int
	depth     int
	value     interface{}
	generator OptionGenerator
	cumuOpts  *big.Int
}

// Instances of PermutationConsumer may be supplied to the
// Permutations iteration functions: ForEach and ForEachPar.
type PermutationConsumer interface {
	// Clone is used only by Permutations.ForEachPar and is called
	// once for each go-routine which will be supplying permutations
	// to the PermutationConsumer. Through this, state can be
	// duplicated so that the consumer can be stateful and safe to
	// drive from multiple go-routines.
	Clone() PermutationConsumer
	// This function called once for each permutation generated.
	Consume(*big.Int, []interface{})
}

// Permutations allows you to interate through the available
// permutations, and extract specific permutations.
type Permutations struct {
	*node
}

var (
	bigIntZero = big.NewInt(0)
	bigIntOne  = big.NewInt(1)
)

// Construct a Permutations from an OptionGenerator.
func BuildPermutations(gen OptionGenerator) *Permutations {
	cur := &node{
		n:         bigIntZero,
		depth:     0,
		generator: gen,
		cumuOpts:  bigIntOne,
	}
	return &Permutations{
		node: cur,
	}
}

type permN struct {
	perm []interface{}
	n    *big.Int
}

type parPermutationConsumer struct {
	ch        chan<- []*permN
	batch     []*permN
	batchIdx  int
	batchSize int
}

func (ppc *parPermutationConsumer) Clone() PermutationConsumer {
	return &parPermutationConsumer{
		ch:        ppc.ch,
		batch:     make([]*permN, ppc.batchSize),
		batchIdx:  0,
		batchSize: ppc.batchSize,
	}
}

func (ppc *parPermutationConsumer) Consume(n *big.Int, perm []interface{}) {
	permCopy := make([]interface{}, len(perm))
	copy(permCopy, perm)
	ppc.batch[ppc.batchIdx] = &permN{n: n, perm: permCopy}
	ppc.batchIdx += 1
	if ppc.batchIdx == ppc.batchSize {
		ppc.ch <- ppc.batch
		ppc.batch = make([]*permN, ppc.batchSize)
		ppc.batchIdx = 0
	}
}

func (ppc *parPermutationConsumer) flush() {
	if ppc.batchIdx > 0 {
		ppc.ch <- ppc.batch[:ppc.batchIdx]
		ppc.batch = make([]*permN, ppc.batchSize)
		ppc.batchIdx = 0
	}
}

// Iterate through every permutation and use concurrency. A number of
// go-routines will be spawned appropriate for the current value of
// GOMAXPROCS. These go-routines will be fed batches of permutations
// and then invoke f.Consume for each permutation. It's your
// responsibility to make sure f is safe to be run concurrently from
// multiple go-routines (see PermutationConsumer.Clone to see how
// stateful consumers can be built).
//
// If the batchsize is very low, then the generation of permutations
// will thrash CPU due to contention for locks on channels. If your
// processing of each permutation is very quick, then high numbers
// (e.g. 8192) can help to keep your CPU busy. If your processing of
// each permutation is less quick then lower numbers will avoid memory
// ballooning. Some trial and error may be worthwhile to find a good
// number for your computer, but 2048 is a sensible place to start.
func (p *Permutations) ForEachPar(batchSize int, f PermutationConsumer) {
	par := runtime.GOMAXPROCS(0) // 0 gets the current count
	var wg sync.WaitGroup
	wg.Add(par)
	ch := make(chan []*permN, par*par)

	for idx := 0; idx < par; idx += 1 {
		go func() {
			defer wg.Done()
			g := f.Clone()
			for {
				perms, ok := <-ch
				if !ok {
					return
				}
				for _, perm := range perms {
					g.Consume(perm.n, perm.perm)
				}
			}
		}()
	}

	ppc := &parPermutationConsumer{
		ch:        ch,
		batch:     make([]*permN, batchSize),
		batchIdx:  0,
		batchSize: batchSize,
	}
	p.ForEach(ppc)
	ppc.flush()
	close(ch)
	wg.Wait()
}

// Iterate through every permutation in the current go-routine. No
// parallelism occurs. The function f.Consume is invoked once for
// every permutation. It is supplied with the permutation number, and
// the permutation itself. These arguments should be considered
// read-only. If you mutate the permutation number or permutation then
// behaviour is undefined.
func (p *Permutations) ForEach(f PermutationConsumer) {
	perm := []interface{}{}

	worklist := []*node{&node{
		n:         p.node.n,
		depth:     p.node.depth,
		generator: p.node.generator.Clone(),
		cumuOpts:  p.node.cumuOpts,
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
			f.Consume(cur.n, perm[1:])

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

// Every permutation has a unique number, which is supplied to the
// function passed to both of the iteration functions in
// Permutations. If you need to generate specific permutations, those
// numbers can be provided to Permutation, which will generate the
// exact same permutation. Note that iterating through a range of
// permutation numbers and repeatedly calling Permutation is slower
// than using either of the iterator functions.
func (p *Permutations) Permutation(permNum *big.Int) []interface{} {
	n := big.NewInt(0).Set(permNum)
	perm := []interface{}{}
	choiceBig := big.NewInt(0)

	gen := p.node.generator.Clone()
	val := p.node.value
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
