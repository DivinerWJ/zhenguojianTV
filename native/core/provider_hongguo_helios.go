package core

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/binary"
	"strconv"
)

// 红果 App 的 x-helios 签名，与 x-medusa 同属必需头。

func hongguoRotateRight64(value uint64, count uint) uint64 {
	shift := count & 63
	if shift == 0 {
		return value
	}
	return value>>shift | value<<(64-shift)
}

// 34 轮 64 位 ARX，初始状态取自 MD5(随机数||"8662") 的十六进制文本。
func hongguoHelios(khronos, random uint32) string {
	seed := make([]byte, 0, 8)
	seed = append(seed, hongguoLe32(random)...)
	seed = append(seed, '8', '6', '6', '2')
	digest := md5.Sum(seed)
	text := make([]byte, 0, 32)
	for _, value := range digest {
		text = append(text, "0123456789abcdef"[value>>4], "0123456789abcdef"[value&0x0f])
	}
	var words [4]uint64
	for index := 0; index < 4; index++ {
		words[index] = binary.LittleEndian.Uint64(text[index*8:])
	}
	table := make([]uint64, 0, 35)
	table = append(table, words[0])
	current, pending := words[0], words[1]
	queue := make([]uint64, 0, 40)
	queue = append(queue, words[2], words[3])
	for index := 0; index < 34; index++ {
		value := hongguoRotateRight64(pending, 8) + current
		value ^= uint64(index)
		queue = append(queue, value)
		value ^= hongguoRotateRight64(current, 61)
		table = append(table, value)
		current = value
		pending = queue[0]
		queue = queue[1:]
	}
	message := []byte(strconv.FormatUint(uint64(khronos), 10) + "-1588093228-8662")
	length := (len(message) + 1 + 15) / 16 * 16
	padding := make([]byte, length)
	for index := range padding {
		padding[index] = byte(16 - len(message)%16)
	}
	copy(padding, message)
	out := make([]byte, 0, 4+length)
	out = append(out, hongguoLe32(random)...)
	for offset := 0; offset < len(padding); offset += 16 {
		left := binary.LittleEndian.Uint64(padding[offset:])
		right := binary.LittleEndian.Uint64(padding[offset+8:])
		for index := 0; index < 34; index++ {
			right = table[index] ^ (left + hongguoRotateRight64(right, 8))
			left = right ^ hongguoRotateRight64(left, 61)
		}
		var block [16]byte
		binary.LittleEndian.PutUint64(block[0:], left)
		binary.LittleEndian.PutUint64(block[8:], right)
		out = append(out, block[:]...)
	}
	return base64.StdEncoding.EncodeToString(out)
}
