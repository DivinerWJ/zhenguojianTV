package core

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"math"
	"math/bits"
	"strings"
	"time"
)

// 红果 App 的 x-medusa 签名，移植自 jackotom/honggou-drama（GPL-3.0）。
// 缺这个头时接口会随机返回 200 + 空响应，是"时好时坏"的根因。

var (
	hongguoMedusaSignKey = hongguoHexBytes("8ebdfa3806ecc5cee79423e6029ed82540bc2218bb7eaef71cb691f7aa8aa2f5")
	hongguoMedusaAESKey  = hongguoHexBytes("f1593376766ea98d34f31b057a9d5be4")
	hongguoMedusaAESIV   = hongguoHexBytes("1fe109a4125283f418de9e051a969e12")
	hongguoMedusaVersion = hongguoHexBytes("03000000f7e85ffad7d7dc3bd62ac87057cf6118")
)

const hongguoMedusaEnv = `{"cmr":16777216,"cmr2":16777216,"un_h":1879194040,"vpn":0,"kd":0,"fkd":3672518972,"pd":-1872573247,"dyn":"","do":0,"tk":true}`

// 进程级常量：原 JS 在模块加载时各取一次，签名里的字段 16 与 25~28 依赖它们。
var (
	hongguoProcessNonce   = newHongguoProcessNonce()
	hongguoProcessStarted = time.Now().UnixMilli()
)

func hongguoHexBytes(value string) []byte {
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return nil
	}
	return decoded
}

func newHongguoProcessNonce() string {
	var buffer [18]byte
	for index := range buffer {
		buffer[index] = byte(hongguoRandomUint32())
	}
	return base64.RawURLEncoding.EncodeToString(buffer[:])
}

func hongguoRandomUint32() uint32 {
	var buffer [4]byte
	if _, err := rand.Read(buffer[:]); err != nil {
		return uint32(time.Now().UnixNano())
	}
	return binary.LittleEndian.Uint32(buffer[:])
}

func hongguoMD5Sum(data []byte) [16]byte {
	return md5.Sum(data)
}

// ---- protobuf 风格编码 ----

type hongguoProtoValue struct {
	tag  int
	kind byte
	text string
	raw  []byte
	num  int64
	flt  float32
}

func hongguoProtoStr(tag int, text string) hongguoProtoValue {
	return hongguoProtoValue{tag: tag, kind: 's', text: text}
}

func hongguoProtoBytes(tag int, raw []byte) hongguoProtoValue {
	return hongguoProtoValue{tag: tag, kind: 'b', raw: raw}
}

func hongguoProtoMessage(tag int, raw []byte) hongguoProtoValue {
	return hongguoProtoValue{tag: tag, kind: 'm', raw: raw}
}

func hongguoProtoSint(tag int, num int64) hongguoProtoValue {
	return hongguoProtoValue{tag: tag, kind: 'i', num: num}
}

func hongguoProtoFloat(tag int, flt float32) hongguoProtoValue {
	return hongguoProtoValue{tag: tag, kind: 'f', flt: flt}
}

func hongguoVarint(value uint64) []byte {
	out := make([]byte, 0, 10)
	for value > 0x7f {
		out = append(out, byte(value&0x7f)|0x80)
		value >>= 7
	}
	return append(out, byte(value))
}

func hongguoZigzag(value int64) uint64 {
	if value < 0 {
		return uint64(-value)*2 - 1
	}
	return uint64(value) * 2
}

// 空字符串字段整段省略；bytes/message 即使长度为 0 也写入 tag 与长度。
func hongguoProtoEncode(fields ...hongguoProtoValue) []byte {
	var out []byte
	for _, field := range fields {
		switch field.kind {
		case 's':
			if field.text == "" {
				continue
			}
			body := []byte(field.text)
			out = append(out, hongguoVarint(uint64(field.tag)<<3|2)...)
			out = append(out, hongguoVarint(uint64(len(body)))...)
			out = append(out, body...)
		case 'b', 'm':
			out = append(out, hongguoVarint(uint64(field.tag)<<3|2)...)
			out = append(out, hongguoVarint(uint64(len(field.raw)))...)
			out = append(out, field.raw...)
		case 'f':
			out = append(out, hongguoVarint(uint64(field.tag)<<3|5)...)
			var encoded [4]byte
			binary.LittleEndian.PutUint32(encoded[:], math.Float32bits(field.flt))
			out = append(out, encoded[:]...)
		default:
			out = append(out, hongguoVarint(uint64(field.tag)<<3)...)
			out = append(out, hongguoVarint(hongguoZigzag(field.num))...)
		}
	}
	return out
}

// ---- 魔改 MD5（hashF13 的 branch 2 专用）----

func hongguoLe32(value uint32) []byte {
	encoded := make([]byte, 4)
	binary.LittleEndian.PutUint32(encoded, value)
	return encoded
}

func hongguoSumMd5(data []byte) uint32 {
	check := uint32(0x20220420)
	for index := 0; index < 12; index++ {
		var temp uint32
		if index&1 == 0 {
			temp = (check >> 3) ^ check
			check = uint32(data[index]) ^ (check << 7)
		} else {
			temp = (check >> 5) ^ check
			check = uint32(data[index]) | (check << 11)
			check = ^check
		}
		check ^= temp
	}
	return (check | 4) ^ 0x1000000
}

func hongguoMd5V3Step(kind int, a, b, c, d, m, shift, constant uint32) uint32 {
	var mixed uint32
	switch kind {
	case 0:
		mixed = (b & c) | (^b & d)
	case 1:
		mixed = (b & d) | (c & ^d)
	case 2:
		mixed = b ^ c ^ d
	default:
		mixed = c ^ (b | ^d)
	}
	return b + bits.RotateLeft32(a+mixed+m+constant, int(shift&31))
}

var hongguoMd5V3Shifts = [4][16]uint32{
	{7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22},
	{5, 9, 14, 20, 5, 9, 14, 20, 5, 9, 14, 20, 5, 9, 14, 20},
	{4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23},
	{6, 10, 15, 21, 6, 10, 15, 21, 6, 10, 15, 21, 6, 10, 15, 21},
}

// 返回 16 字节摘要 + 4 字节校验。
func hongguoMd5SumV3(message []byte, countV2 uint32, orders []byte, countV1 uint32) []byte {
	if len(message) < 64 || len(orders) < 64 {
		return nil
	}
	var constants [64]uint32
	for index := range constants {
		constants[index] = bits.RotateLeft32(hongguoMedusaSV[index], -int(countV1&31))
	}
	start := [4]uint32{
		bits.RotateLeft32(0x79e0f2fb, -int(countV2&31)),
		bits.RotateLeft32(0xc8b52570, -int(countV2&31)),
		bits.RotateLeft32(0xebc2f8cd, -int(countV2&31)),
		bits.RotateLeft32(0x7c104d93, -int(countV2&31)),
	}
	endCount := (countV2 + 6) & 0xff
	end := [4]uint32{
		bits.RotateLeft32(0x19be4866, -int(endCount&31)),
		bits.RotateLeft32(0xe85986b4, -int(endCount&31)),
		bits.RotateLeft32(0xe19b326e, -int(endCount&31)),
		bits.RotateLeft32(0x71d1d7d4, -int(endCount&31)),
	}
	var words [16]uint32
	for index := 0; index < 16; index++ {
		words[index] = binary.LittleEndian.Uint32(message[index*4:])
	}
	a, b, c, d := start[0], start[1], start[2], start[3]
	for kind := 0; kind < 4; kind++ {
		shifts := hongguoMd5V3Shifts[kind]
		offset := kind * 16
		for index := 0; index < 16; index++ {
			word := words[orders[offset+index]&0x0f]
			shift := shifts[index]
			constant := constants[offset+index]
			switch index & 3 {
			case 0:
				a = hongguoMd5V3Step(kind, a, b, c, d, word, shift, constant)
			case 1:
				d = hongguoMd5V3Step(kind, d, a, b, c, word, shift, constant)
			case 2:
				c = hongguoMd5V3Step(kind, c, d, a, b, word, shift, constant)
			default:
				b = hongguoMd5V3Step(kind, b, c, d, a, word, shift, constant)
			}
		}
	}
	digest := make([]byte, 20)
	binary.LittleEndian.PutUint32(digest[0:], (start[0]+a)^end[0])
	binary.LittleEndian.PutUint32(digest[4:], (start[1]+b)^end[1])
	binary.LittleEndian.PutUint32(digest[8:], (start[2]+c)^end[2])
	binary.LittleEndian.PutUint32(digest[12:], (start[3]+d)^end[3])
	copy(digest[16:], hongguoLe32(hongguoSumMd5(digest[:16])))
	return digest
}

var hongguoBranch2Seeds = [5]uint32{0x8980f29b, 0xeb549c7f, 0xb08726db, 0xd40cb5e6, 0xe8f559e4}

func hongguoBranch2F13(iv uint32, querySm3, bodyMd5, tsBytes []byte, khronos uint32) []byte {
	value := (iv & 13) * 86
	group := int(((value >> 15) & 0xff) + ((value >> 8) & 0xff))
	if group >= len(hongguoBranch2Seeds) {
		return nil
	}
	seed := hongguoBranch2Seeds[group]
	countV1 := (seed + khronos + 1) & 0xff
	countV2 := seed + khronos
	shift := (countV2 + 5) & 7
	paddingSeed := [8]byte{0x84, 0x96, 0x77, 0x9d, 0xd4, 0x15, 0x0b, 0xf8}
	padding := make([]byte, len(paddingSeed))
	for index, value := range paddingSeed {
		padding[index] = byte(((uint32(value) | uint32(value)<<8) >> shift) & 0xff)
	}
	input := make([]byte, 0, 64)
	input = append(input, querySm3...)
	input = append(input, bodyMd5...)
	input = append(input, tsBytes...)
	input = append(input, padding...)
	input = append(input, 0xa0, 0x01, 0x00, 0x00)
	orders := hongguoMedusaOrders[group*64 : (group+1)*64]
	return hongguoMd5SumV3(input, countV2, orders, countV1)
}

// ---- hashF13 主流程 ----

func hongguoGetIv(iv uint32, data []byte) uint32 {
	value := iv
	for index := 0; index < len(data); index++ {
		if index&1 == 0 {
			value = (value >> 4) ^ value ^ (value << 6) ^ uint32(data[index])
		} else {
			value = ^((value >> 7) ^ value ^ (uint32(data[index]) | (value << 12)))
		}
	}
	return value
}

var (
	hongguoF13Seeds = [6]uint32{0xc4a78580, 0xb3c0fd39, 0xc58c5686, 0xc9aa3ba7, 0xf5a7adf2, 0x963c2ed1}
	hongguoF13TT    = [64]uint32{
		0xebb64faf, 0x07aadcc2, 0xcf3187bf, 0xe01138ff, 0x6d0bfcff, 0x5a30a3be, 0xb41ad638, 0x34180eb8,
		0xf233eb6f, 0xb1a584cc, 0xccc30dc7, 0x47d1db51, 0xd55653de, 0x70a84fa1, 0x57473c12, 0xf76f0288,
		0x2c077f0a, 0xda0dcad0, 0xfbb86f6c, 0xfdc4cf00, 0x688a020d, 0xe676c6a6, 0x8cd6338b, 0x1a3c8d0e,
		0xcce8b06b, 0x6ad0ed0b, 0xa0522717, 0xdc71ac83, 0x2285db71, 0xd5b4dda6, 0x736f8650, 0x6560306c,
		0x617ce2a6, 0xe423417e, 0x0a40e143, 0x544e4032, 0x88dffb2a, 0x716c1ae0, 0x4c467a88, 0x05b23bb3,
		0xe1d0b866, 0xbaa3dcb8, 0xae3374d3, 0xc3381a50, 0x1702f75b, 0xfe6da368, 0xf0b4cf48, 0x4e0ffbb8,
		0x72aad10d, 0x26c53a3d, 0xf2bce0f6, 0xb4557581, 0x4a257fdd, 0x8c3182a2, 0xab0b3b86, 0x3d5dfb14,
		0x4f103634, 0xd37b52d7, 0x444eff16, 0xeb0a33d1, 0x6ca86f6e, 0x00284ba7, 0x08387cfa, 0x5fb37586,
	}
	// 每行格式：[迭代次数, d1, d2, d3, d4, d5, d6, d7, d8, d9, d10]
	hongguoF13Chosen = [6][11]int{
		{101, 5, 7, 6, 3, 2, 1, 0, 5, 4, 3},
		{96, 0, 6, 7, 5, 3, 2, 1, 5, 4, 4},
		{96, 7, 6, 2, 1, 4, 0, 5, 4, 3, 5},
		{99, 3, 6, 2, 4, 5, 1, 0, 0, 7, 6},
		{96, 0, 5, 6, 7, 3, 1, 2, 5, 4, 4},
		{100, 2, 0, 3, 5, 4, 6, 7, 2, 1, 5},
	}
)

func hongguoF13Xor(left, right []byte) []byte {
	length := len(left)
	if len(right) < length {
		length = len(right)
	}
	out := make([]byte, length)
	for index := 0; index < length; index++ {
		out[index] = left[index] ^ right[index]
	}
	return out
}

// 按 IV 分成三条分支：2 走魔改 MD5，0 走 112 轮扩展 + 表驱动压缩，1 走简易拼接。
func hongguoHashF13(querySm3, bodyMd5, tsBytes []byte, khronos uint32) []byte {
	iv := hongguoGetIv(hongguoGetIv(hongguoGetIv(0x20230928, querySm3), bodyMd5), tsBytes)
	low := iv & 15
	group := int((low * 171) >> 9)
	branch := int(low) - group*3
	if branch == 2 {
		return hongguoBranch2F13(iv, querySm3, bodyMd5, tsBytes, khronos)
	}
	if branch != 0 {
		combined := make([]byte, 0, len(querySm3)+len(bodyMd5))
		combined = append(combined, querySm3...)
		combined = append(combined, bodyMd5...)
		out := make([]byte, 0, 40)
		out = append(out, querySm3[:16]...)
		out = append(out, bodyMd5...)
		out = append(out, hongguoLe32(hongguoSumMd5(combined))...)
		return out
	}
	if group >= len(hongguoF13Seeds) {
		return nil
	}
	seed := hongguoF13Seeds[group]
	countV1 := (seed + khronos + 1) & 0xff
	countV2 := seed + khronos
	constants := make([]uint32, len(hongguoF13TT))
	for index, value := range hongguoF13TT {
		constants[index] = bits.RotateLeft32(value, -int(countV1&31))
	}
	shift := (countV2 + 2) & 7
	paddingSeed := [4]byte{0xfa, 0x45, 0x61, 0xd7}
	padding := make([]byte, len(paddingSeed))
	for index, value := range paddingSeed {
		padding[index] = byte(((uint32(value) | uint32(value)<<8) >> shift) & 0xff)
	}
	data := make([]byte, 0, 64)
	data = append(data, querySm3...)
	data = append(data, bodyMd5...)
	data = append(data, tsBytes...)
	data = append(data, padding...)
	data = append(data, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0xa0)
	expanded := make([]uint32, len(data)/4, len(data)/4+112)
	for index := 0; index < len(expanded); index++ {
		expanded[index] = binary.BigEndian.Uint32(data[index*4:])
	}
	previous := expanded[0]
	for index := 0; index < 112; index++ {
		current := expanded[index+1]
		first := bits.RotateLeft32(current, 14) ^ bits.RotateLeft32(current, 25) ^ (current >> 3)
		second := bits.RotateLeft32(expanded[index+14], 13) ^ bits.RotateLeft32(expanded[index+14], 15) ^ (expanded[index+14] >> 10)
		expanded = append(expanded, previous+expanded[index+9]+first+second)
		previous = current
	}
	initValues := [8]uint32{0x7aba4fc8, 0x67166507, 0x6403fa00, 0x340f512f, 984304912, 3005047866, 2874125293, 2152413264}
	var initial [8]uint32
	for index, value := range initValues {
		initial[index] = bits.RotateLeft32(value, -int(countV2&31))
	}
	state := initial
	chosen := hongguoF13Chosen[group]
	for index := 0; index < chosen[0]; index++ {
		base := seed + uint32(index)
		mixed := expanded[base&127]
		first := ((state[chosen[3]] ^ state[chosen[4]]) & state[chosen[1]]) ^ state[chosen[3]]
		second := bits.RotateLeft32(state[chosen[1]], 26) ^ bits.RotateLeft32(state[chosen[1]], 21) ^ bits.RotateLeft32(state[chosen[1]], 7)
		combined := mixed + first + second + constants[base&63] + state[chosen[5]]
		third := bits.RotateLeft32(state[chosen[2]], 30) ^ bits.RotateLeft32(state[chosen[2]], 19) ^ bits.RotateLeft32(state[chosen[2]], 10)
		fourth := (state[chosen[2]] & state[chosen[6]]) | ((state[chosen[2]] | state[chosen[6]]) & state[chosen[7]])
		updated := third + fourth
		saved := state[chosen[9]]
		last := state[7]
		copy(state[1:], state[0:7])
		state[0] = last
		state[chosen[10]] = updated + combined
		state[chosen[8]] = saved + combined
	}
	digest := make([]byte, 32)
	for index := 0; index < 8; index++ {
		binary.BigEndian.PutUint32(digest[index*4:], state[index]+initial[index])
	}
	folded := hongguoF13Xor(digest[:16], digest[16:])
	out := make([]byte, 20)
	copy(out[:16], folded)
	copy(out[16:], hongguoLe32(hongguoSumMd5(folded)))
	return out
}

// ---- 字节混淆与密钥派生 ----

func hongguoXmxor(data, key []byte) []byte {
	if len(data) < 4 || len(key) < 32 {
		return nil
	}
	encoded := make([]byte, len(data))
	for index := 0; index < len(data); index++ {
		at := (index * 4) & 28
		first, second := uint32(key[at]), uint32(key[at+1])
		value := (((uint32(data[index]) << 4) | (uint32(data[index]) >> 4)) & 0xff) + first
		value = (^value ^ second) & 0xff
		value = (((value & 0xff) << 3) | ((value & 0xff) >> 5)) & 0xff
		value = (value + second) & 0xff
		value = (value ^ first) & 0xff
		encoded[len(data)-1-index] = byte(^value)
	}
	length := len(encoded)
	last := int32(encoded[length-1]) ^ int32(encoded[length-2])
	first := int32(encoded[0])
	encoded[0] = byte((^last + first) & 0xff)
	encoded[1] = byte(((int32(encoded[0]) ^ int32(encoded[length-1]) ^ 254) + int32(encoded[1])) & 0xff)
	rotate := int32(((uint32(encoded[1]) << 3) | (uint32(encoded[1]) >> 5)) & 0xff)
	encoded[2] = byte((int32(encoded[2]) + ((last - first) ^ rotate ^ 2)) & 0xff)
	for index := 0; index < length-4; index++ {
		mixed := int32(((uint32(encoded[index+2]) << 3) | (uint32(encoded[index+2]) >> 5)) & 0xff)
		temp := mixed ^ int32(encoded[index+1]) ^ int32(index+3)
		encoded[index+3] = byte((^temp + int32(encoded[index+3])) & 0xff)
	}
	encoded[length-1] ^= encoded[length-2]
	sum := int32(0)
	for index := 0; index < length-1; index++ {
		sum += int32(encoded[index+1])
	}
	encoded[0] = byte(((int32(encoded[0]) ^ int32(encoded[1])) + sum) & 0xff)
	return encoded
}

func hongguoKeyHash(signKey []byte, random uint32) ([]byte, []byte) {
	input := make([]byte, 0, len(signKey)*2+4)
	input = append(input, signKey...)
	input = append(input, hongguoLe32(random)...)
	input = append(input, signKey...)
	digest := hongguoSM3(input)
	low := (random >> 16) & 0xff
	seed := ^(((low << 11) | (random >> 24)) ^ (low >> 5) ^ low)
	return digest[:], hongguoLe32(seed)
}

// ---- 自制 AES 变体（x-medusa 的载荷加密）----

type hongguoAesV3 struct {
	wordSize int
	box      []byte
	con2     [4]byte
	order    [16]byte
	keys     [12][4]byte
}

var (
	hongguoAesInit = [4]uint32{0xca025ddc, 0x823dc546, 0xc9420583, 0xc298225f}
	hongguoAesCon2 = [4][4]byte{
		{1, 0, 2, 3}, {2, 0, 3, 1}, {0, 1, 3, 2}, {1, 0, 2, 3},
	}
	hongguoAesOrder = [4][16]byte{
		{0, 9, 14, 11, 4, 13, 2, 7, 8, 1, 6, 15, 12, 5, 10, 3},
		{0, 9, 14, 15, 4, 13, 2, 7, 8, 1, 6, 3, 12, 5, 10, 11},
		{0, 9, 14, 7, 4, 13, 2, 11, 8, 1, 6, 3, 12, 5, 10, 15},
		{0, 9, 14, 11, 4, 13, 2, 7, 8, 1, 6, 15, 12, 5, 10, 3},
	}
)

func newHongguoAesV3(key []byte, khronos uint32) *hongguoAesV3 {
	wordSize := int(khronos & 3)
	cipher := &hongguoAesV3{
		wordSize: wordSize,
		box:      hongguoSbox[wordSize*256 : (wordSize+1)*256],
		con2:     hongguoAesCon2[wordSize],
		order:    hongguoAesOrder[wordSize],
	}
	initial := make([]byte, 16)
	initValue := hongguoAesInit[wordSize]
	for index := 0; index < 4; index++ {
		binary.LittleEndian.PutUint32(initial[index*4:], initValue)
	}
	expanded := make([]byte, 48)
	for index := 0; index < 16; index++ {
		expanded[index] = initial[index] ^ key[index%len(key)]
	}
	rounds := uint32(8)
	for index := 4; index < 12; index++ {
		at := 4 * (index - 1)
		a, b, c, d := expanded[at], expanded[at+1], expanded[at+2], expanded[at+3]
		if index&3 == 0 {
			replaced := byte(((initValue >> (rounds & 24)) ^ uint32(cipher.box[b])) & 0xff)
			b, c, d, a = cipher.box[c], cipher.box[d], cipher.box[a], replaced
		}
		rounds += 2
		expanded[at+4] = a ^ expanded[at-12]
		expanded[at+5] = b ^ expanded[at-11]
		expanded[at+6] = c ^ expanded[at-10]
		expanded[at+7] = d ^ expanded[at-9]
	}
	for index := 0; index < 12; index++ {
		copy(cipher.keys[index][:], expanded[index*4:index*4+4])
	}
	return cipher
}

func hongguoAesMix(rows *[4][4]byte) {
	double := func(value byte) byte {
		if value&0x80 != 0 {
			return (value << 1) ^ 0x1b
		}
		return value << 1
	}
	for column := 0; column < 4; column++ {
		total := rows[0][column] ^ rows[1][column] ^ rows[2][column] ^ rows[3][column]
		first := rows[0][column]
		rows[0][column] ^= total ^ double(rows[0][column]^rows[1][column])
		rows[1][column] ^= total ^ double(rows[1][column]^rows[2][column])
		rows[2][column] ^= total ^ double(rows[2][column]^rows[3][column])
		rows[3][column] ^= total ^ double(rows[3][column]^first)
	}
}

func (cipher *hongguoAesV3) addCon(rows *[4][4]byte, keys [][4]byte) {
	for row := 0; row < 4; row++ {
		for column := 0; column < 4; column++ {
			rows[row][column] ^= keys[row][cipher.con2[column]]
		}
	}
}

func (cipher *hongguoAesV3) sub(rows *[4][4]byte) {
	for row := 0; row < 4; row++ {
		for column := 0; column < 4; column++ {
			rows[row][column] = cipher.box[rows[row][column]]
		}
	}
	previous := *rows
	for row := 0; row < 4; row++ {
		rows[row] = previous[cipher.con2[row]]
	}
}

func (cipher *hongguoAesV3) shift(rows *[4][4]byte) {
	var flat [16]byte
	for row := 0; row < 4; row++ {
		copy(flat[row*4:], rows[row][:])
	}
	for index := 0; index < 16; index++ {
		rows[index/4][index%4] = flat[cipher.order[index]]
	}
}

func (cipher *hongguoAesV3) shiftCon(rows *[4][4]byte) {
	previous := *rows
	for row := 0; row < 4; row++ {
		for column := 0; column < 4; column++ {
			rows[row][column] = previous[row][cipher.con2[column]]
		}
	}
}

func (cipher *hongguoAesV3) block(value []byte) []byte {
	var rows [4][4]byte
	for row := 0; row < 4; row++ {
		copy(rows[row][:], value[row*4:row*4+4])
	}
	cipher.addCon(&rows, cipher.keys[0:4])
	for round := 1; round < 3; round++ {
		cipher.sub(&rows)
		cipher.shift(&rows)
		if round == 1 {
			cipher.shiftCon(&rows)
			hongguoAesMix(&rows)
		}
		cipher.addCon(&rows, cipher.keys[round*4:(round+1)*4])
	}
	for row := 0; row < 4; row++ {
		for column := 0; column < 4; column++ {
			rows[row][column] ^= cipher.keys[4+row][column]
		}
	}
	out := make([]byte, 16)
	for row := 0; row < 4; row++ {
		copy(out[row*4:], rows[row][:])
	}
	return out
}

// 越界读返回 0，对应 JS 里 undefined 参与位运算的结果。
func hongguoAesByte(data []byte, index int) byte {
	if index < 0 || index >= len(data) {
		return 0
	}
	return data[index]
}

func (cipher *hongguoAesV3) encrypt(data, iv []byte) []byte {
	source := data
	plaintext := make([]byte, 32)
	for index := 0; index < 31; index++ {
		at := index * 8
		value := uint32(hongguoAesByte(source, at)>>4) & 2
		value |= uint32(hongguoAesByte(source, at+1)) & 64
		value |= uint32(hongguoAesByte(source, at+2)>>2) & 1
		value |= uint32(hongguoAesByte(source, at+3)<<3) & 128
		value |= uint32(hongguoAesByte(source, at+4)>>1) & 4
		value |= uint32(hongguoAesByte(source, at+5)<<3) & 16
		value |= uint32(hongguoAesByte(source, at+6)<<5) & 32
		value |= uint32(hongguoAesByte(source, at+7)>>4) & 8
		plaintext[index] = byte(value)
	}
	plaintext[31] = 1
	previous := make([]byte, 16)
	copy(previous, iv)
	key := make([]byte, 0, 32)
	for index := 0; index < 32; index += 16 {
		blockInput := make([]byte, 16)
		for offset := 0; offset < 16; offset++ {
			blockInput[offset] = plaintext[index+offset] ^ previous[offset]
		}
		previous = cipher.block(blockInput)
		key = append(key, previous...)
	}
	out := make([]byte, len(source))
	copy(out, source)
	for index := 0; index < 31 && index < len(key); index++ {
		at := index * 8
		if at+7 >= len(out) {
			break
		}
		mask := key[index]
		out[at] = (out[at] & 0xdf) | (mask << 4 & 32)
		out[at+1] = (out[at+1] & 0xbf) | (mask & 64)
		out[at+2] = (out[at+2] & 0xfb) | (mask << 2 & 4)
		out[at+3] = (out[at+3] & 0xef) | (mask >> 3 & 16)
		out[at+4] = (out[at+4] & 0xf7) | (mask+mask)&8
		out[at+5] = (out[at+5] & 0xfd) | (mask >> 3 & 2)
		out[at+6] = (out[at+6] & 0xfe) | (mask >> 5 & 1)
		out[at+7] = (out[at+7] & 0x7f) | (mask << 4 & 128)
	}
	result := make([]byte, 0, len(out)+1)
	result = append(result, key[len(key)-1])
	result = append(result, out...)
	return result
}

// ---- 设备指纹与消息组装 ----

func hongguoDeviceProto(deviceID, nonce string, started int64) []byte {
	zero := float32(0)
	return hongguoProtoEncode(
		hongguoProtoSint(1, 1), hongguoProtoSint(2, 2), hongguoProtoStr(3, "8662"),
		hongguoProtoStr(4, deviceID), hongguoProtoStr(5, nonce), hongguoProtoStr(6, "!noperm!"),
		hongguoProtoSint(7, -888888), hongguoProtoSint(8, -888888), hongguoProtoSint(9, 3),
		hongguoProtoSint(10, -888888), hongguoProtoStr(11, "!notset!"), hongguoProtoStr(12, "Asia/Shanghai,8"),
		hongguoProtoStr(13, "zh_CN"), hongguoProtoSint(14, 4),
		hongguoProtoFloat(16, zero), hongguoProtoFloat(17, zero), hongguoProtoFloat(18, zero),
		hongguoProtoFloat(19, zero), hongguoProtoFloat(20, zero), hongguoProtoFloat(21, zero),
		hongguoProtoStr(22, "16"), hongguoProtoSint(23, 41), hongguoProtoSint(24, 36),
		hongguoProtoSint(25, started), hongguoProtoSint(26, started), hongguoProtoSint(27, started),
		hongguoProtoSint(28, started), hongguoProtoSint(29, -1),
		hongguoProtoStr(30, "25053RT47C"), hongguoProtoStr(31, "Redmi"), hongguoProtoStr(32, "25053RT47C"),
		hongguoProtoStr(33, "25053RT47C"), hongguoProtoStr(34, "Xiaomi"), hongguoProtoStr(35, "Redmi"),
		hongguoProtoStr(36, "Redmi"), hongguoProtoSint(38, 31),
	)
}

// hongguoMedusaRandom 收集 x-medusa 需要的全部随机量，便于测试时注入固定值。
type hongguoMedusaRandom struct {
	MessageRand uint32
	EnvLaunch   int64
	EnvPid      int64
	Random      uint32
	XmRand      uint32
	Nonce       string
	Started     int64
	Now         int64
}

func newHongguoMedusaRandom() hongguoMedusaRandom {
	return hongguoMedusaRandom{
		MessageRand: hongguoRandomUint32(),
		EnvLaunch:   int64(hongguoRandomUint32()%21) + 100,
		EnvPid:      int64(hongguoRandomUint32()%2000) + 10001,
		Random:      hongguoRandomUint32(),
		XmRand:      hongguoRandomUint32(),
		Nonce:       hongguoProcessNonce,
		Started:     hongguoProcessStarted,
		Now:         time.Now().Unix(),
	}
}

// rawURL 必须与实际发出的 URL（含 _rticket）完全一致：签名对 query 字节级敏感。
func hongguoBuildMedusa(rawURL string, body []byte, khronos uint32, deviceID, versionName string, rnd hongguoMedusaRandom) string {
	bodyMd5 := make([]byte, 16)
	if body != nil {
		digest := hongguoMD5Sum(body)
		copy(bodyMd5, digest[:])
	}
	query := ""
	if index := strings.IndexByte(rawURL, '?'); index >= 0 {
		query = rawURL[index+1:]
	}
	queryDigest := hongguoSM3([]byte(query))
	querySm3 := queryDigest[:]
	tsBytes := hongguoLe32(khronos)
	queryBodyTs := hongguoHashF13(querySm3, bodyMd5, tsBytes, khronos)
	if len(queryBodyTs) == 0 {
		// hashF13 只在分组索引越界时返回空，正常输入（32+16+4 字节）不会走到这里。
		return ""
	}
	nested := hongguoProtoEncode(
		hongguoProtoSint(1, 111), hongguoProtoSint(2, 10),
		hongguoProtoSint(3, 694367), hongguoProtoSint(5, 586952199),
	)
	launch := hongguoProtoEncode(
		hongguoProtoSint(1, rnd.Now), hongguoProtoSint(2, -2), hongguoProtoSint(4, 200),
	)
	environment := hongguoProtoEncode(
		hongguoProtoSint(1, rnd.EnvLaunch), hongguoProtoSint(2, 146331399), hongguoProtoSint(3, 146331396),
		hongguoProtoSint(5, 7), hongguoProtoStr(6, "v04.06.04.03-bugfix"), hongguoProtoSint(7, rnd.EnvPid),
		hongguoProtoMessage(12, hongguoDeviceProto(deviceID, rnd.Nonce, rnd.Started)),
		hongguoProtoMessage(13, launch), hongguoProtoStr(14, versionName),
	)
	hashInput := make([]byte, 0, len(query)+len(bodyMd5)+4)
	hashInput = append(hashInput, query...)
	hashInput = append(hashInput, bodyMd5...)
	hashInput = append(hashInput, 'n', 'o', 'n', 'e')
	queryHashDigest := hongguoSM3(hashInput)
	message := hongguoProtoEncode(
		hongguoProtoBytes(1, hongguoHexBytes("f7e85ffad7d7dc3bd62ac87057cf6118")),
		hongguoProtoSint(2, 3), hongguoProtoSint(3, int64(rnd.MessageRand)),
		hongguoProtoStr(4, "8662"), hongguoProtoStr(5, deviceID),
		hongguoProtoStr(6, "1588093228"), hongguoProtoStr(7, versionName),
		hongguoProtoStr(8, "v04.06.04-ml-android"), hongguoProtoSint(9, 67503104),
		hongguoProtoBytes(10, hongguoHexBytes("4001000000000000")),
		hongguoProtoSint(12, int64(khronos)), hongguoProtoBytes(13, queryBodyTs),
		hongguoProtoBytes(14, querySm3[:6]), hongguoProtoMessage(15, nested),
		hongguoProtoStr(16, rnd.Nonce), hongguoProtoSint(17, int64(khronos)),
		hongguoProtoBytes(19, queryHashDigest[:]), hongguoProtoStr(20, "none"),
		hongguoProtoSint(21, 312), hongguoProtoMessage(23, environment),
		hongguoProtoStr(24, hongguoMedusaEnv),
	)
	hash, seed := hongguoKeyHash(hongguoMedusaSignKey, rnd.Random)
	transformed := hongguoXmxor(message, hash)
	if transformed == nil {
		return ""
	}
	transformed = append(hongguoHexBytes("4001000000000000"), transformed...)
	for left, right := 0, len(transformed)-1; left < right; left, right = left+1, right-1 {
		transformed[left], transformed[right] = transformed[right], transformed[left]
	}
	for index := 0; index < len(transformed); index++ {
		transformed[index] ^= seed[^uint32(index)&3]
	}
	check := (uint32(querySm3[0])&63)<<14 | 0x18000001 | (uint32(queryBodyTs[0])&63)<<8
	packed := make([]byte, 0, len(transformed)+11)
	packed = append(packed, 0x35)
	packed = append(packed, hongguoLe32(rnd.XmRand)...)
	packed = append(packed, hongguoLe32(check)...)
	packed = append(packed, transformed...)
	packed = append(packed, byte(rnd.Random>>16), byte(rnd.Random>>24))
	encrypted := newHongguoAesV3(hongguoMedusaAESKey, khronos).encrypt(packed, hongguoMedusaAESIV)
	prefix := make([]byte, 20)
	for index := 0; index < 20; index += 4 {
		value := binary.LittleEndian.Uint32(hongguoMedusaVersion[index:])
		binary.LittleEndian.PutUint32(prefix[index:], value^khronos)
	}
	out := make([]byte, 0, 24+len(encrypted))
	out = append(out, prefix...)
	out = append(out, byte(rnd.Random), byte(rnd.Random>>8), 0, 1)
	out = append(out, encrypted...)
	return base64.StdEncoding.EncodeToString(out)
}
