package core

import (
	"encoding/binary"
	"math/bits"
)

// 标准 SM3 摘要，与 Node 的 crypto.createHash('sm3') 逐字节一致。
func hongguoSM3(input []byte) [32]byte {
	length := uint64(len(input)) * 8
	data := append(append([]byte(nil), input...), 0x80)
	for len(data)%64 != 56 {
		data = append(data, 0)
	}
	data = binary.BigEndian.AppendUint64(data, length)
	state := [8]uint32{0x7380166f, 0x4914b2b9, 0x172442d7, 0xda8a0600, 0xa96f30bc, 0x163138aa, 0xe38dee4d, 0xb0fb0e4e}
	for offset := 0; offset < len(data); offset += 64 {
		var words [68]uint32
		for index := 0; index < 16; index++ {
			words[index] = binary.BigEndian.Uint32(data[offset+index*4:])
		}
		for index := 16; index < len(words); index++ {
			mixed := words[index-16] ^ words[index-9] ^ bits.RotateLeft32(words[index-3], 15)
			words[index] = mixed ^ bits.RotateLeft32(mixed, 15) ^ bits.RotateLeft32(mixed, 23) ^
				bits.RotateLeft32(words[index-13], 7) ^ words[index-6]
		}
		a, b, c, d, e, f, g, h := state[0], state[1], state[2], state[3], state[4], state[5], state[6], state[7]
		for index := 0; index < 64; index++ {
			constant, first, second := uint32(0x79cc4519), a^b^c, e^f^g
			if index >= 16 {
				constant, first, second = 0x7a879d8a, (a&b)|(a&c)|(b&c), (e&f)|(^e&g)
			}
			ss1 := bits.RotateLeft32(bits.RotateLeft32(a, 12)+e+bits.RotateLeft32(constant, index), 7)
			tt1 := first + d + (ss1 ^ bits.RotateLeft32(a, 12)) + (words[index] ^ words[index+4])
			tt2 := second + h + ss1 + words[index]
			a, b, c, d = tt1, a, bits.RotateLeft32(b, 9), c
			e, f, g, h = tt2^bits.RotateLeft32(tt2, 9)^bits.RotateLeft32(tt2, 17), e, bits.RotateLeft32(f, 19), g
		}
		for index, value := range [8]uint32{a, b, c, d, e, f, g, h} {
			state[index] ^= value
		}
	}
	var result [32]byte
	for index, value := range state {
		binary.BigEndian.PutUint32(result[index*4:], value)
	}
	return result
}
