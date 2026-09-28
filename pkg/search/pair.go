package search

// rank is a byte-frequency rank table used to pick a "packed pair" of rare
// bytes from a needle for the AVX2 coarse pre-filter: a lower rank means the
// byte occurs less frequently in typical text/log data, so finding it in the
// haystack is more predictive of a genuine match - fewer false-positive
// candidates reach the expensive full-needle memcmp confirmation.
//
// Ported directly from BurntSushi/memchr's default frequency table
// (https://github.com/BurntSushi/memchr/blob/master/src/arch/all/packedpair/default_rank.rs,
// dual MIT/Unlicense), which the "rare bytes" TODO this implements
// originally pointed at under an older path (src/memmem/rarebytes.rs) that
// no longer exists upstream - the technique and table have since moved to
// src/arch/all/packedpair, generalized from "first and last byte" to
// "whichever two positions are rarest," which is what SelectPair below
// mirrors.
var rank = [256]uint8{
	55, 52, 51, 50, 49, 48, 47, 46, 45, 103, 242, 66, 67, 229, 44, 43,
	42, 41, 40, 39, 38, 37, 36, 35, 34, 33, 56, 32, 31, 30, 29, 28,
	255, 148, 164, 149, 136, 160, 155, 173, 221, 222, 134, 122, 232, 202, 215, 224,
	208, 220, 204, 187, 183, 179, 177, 168, 178, 200, 226, 195, 154, 184, 174, 126,
	120, 191, 157, 194, 170, 189, 162, 161, 150, 193, 142, 137, 171, 176, 185, 167,
	186, 112, 175, 192, 188, 156, 140, 143, 123, 133, 128, 147, 138, 146, 114, 223,
	151, 249, 216, 238, 236, 253, 227, 218, 230, 247, 135, 180, 241, 233, 246, 244,
	231, 139, 245, 243, 251, 235, 201, 196, 240, 214, 152, 182, 205, 181, 127, 27,
	212, 211, 210, 213, 228, 197, 169, 159, 131, 172, 105, 80, 98, 96, 97, 81,
	207, 145, 116, 115, 144, 130, 153, 121, 107, 132, 109, 110, 124, 111, 82, 108,
	118, 141, 113, 129, 119, 125, 165, 117, 92, 106, 83, 72, 99, 93, 65, 79,
	166, 237, 163, 199, 190, 225, 209, 203, 198, 217, 219, 206, 234, 248, 158, 239,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
}

// SelectPair picks the two byte positions within needle believed to be the
// most predictive of a genuine match, by choosing the two rarest bytes
// (lowest rank) it can find. Mirrors the algorithm in
// BurntSushi/memchr's Pair::with_ranker.
//
// needle must have length >= 2; the returned indices are always distinct.
func SelectPair(needle []byte) (idx1, idx2 int) {
	rare1, i1 := needle[0], 0
	rare2, i2 := needle[1], 1
	if rank[rare2] < rank[rare1] {
		rare1, rare2 = rare2, rare1
		i1, i2 = i2, i1
	}

	// Match memchr's cap: bytes beyond the first 255 aren't considered, since
	// it's rarely useful to pick a pivot that far into the needle anyway.
	max := len(needle)
	if max > 255 {
		max = 255
	}

	for i := 2; i < max; i++ {
		b := needle[i]
		switch {
		case rank[b] < rank[rare1]:
			rare2, i2 = rare1, i1
			rare1, i1 = b, i
		case b != rare1 && rank[b] < rank[rare2]:
			rare2, i2 = b, i
		}
	}

	return i1, i2
}
