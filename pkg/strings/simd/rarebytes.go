package simd 

type RareNeedleBytes struct {
	rare1i uint8 
	rare2i uint8 
}

func NewRaraNeedleBytes(needle []byte) RareNeedleBytes {
	if len(needle) <= 1 {
		return RareNeedleBytes{0, 0}
	}

	rare1 := needle[0]
	rare1i := uint8(0)
	rare2 := needle[1]
	rare2i := uint8(1)
	if rank(rare2) < rank(rare1) {
		rare1, rare2 = rare2, rare1
		rare1i, rare2i = rare2i, rare1i
	}

	for i, b := range needle[2:] {
		if rank(b) < rank(rare1) {
			rare2, rare2i = rare1, rare1i
			rare1, rare1i = b, uint8(i)
		} else if b!= rare1 && rank(b) < rank(rare2) {
			rare2, rare2i = b, uint8(i)
		}
	}

	return RareNeedleBytes{rare1i, rare2i}
}

func rank(b byte) uint8 {
	return byteFrequencies[int(b)]
}
