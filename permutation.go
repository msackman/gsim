package gsim

type simplePermutation struct {
	remains []interface{}
}

// SimplePermutation is an example implementation of OptionGenerator
// which implements a plain permutation with no dependencies between
// any values. For example, with the elems a,b,c, every permutation
// will be found: a,b,c; a,c,b; b,a,c; b,c,a; c,a,b; c,b,a
func NewSimplePermutation(elems []interface{}) OptionGenerator {
	return &simplePermutation{
		remains: elems,
	}
}

func (sp *simplePermutation) Clone() OptionGenerator {
	nsp := &simplePermutation{
		remains: make([]interface{}, len(sp.remains)),
	}
	copy(nsp.remains, sp.remains)
	return nsp
}

func (sp *simplePermutation) Generate(lastChosen interface{}) []interface{} {
	for idx, elem := range sp.remains {
		if elem == lastChosen {
			sp.remains = append(sp.remains[:idx], sp.remains[idx+1:]...)
			break
		}
	}
	return sp.remains
}
