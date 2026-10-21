package core

import (
	"encoding/hex"
	"testing"
)

// 向量由原版 JS 实现生成，锁定 Go 移植的逐字节一致性。

func hongguoTestBytes(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatalf("十六进制解码失败: %v", err)
	}
	return decoded
}

func hongguoTestHex(value []byte) string { return hex.EncodeToString(value) }

func TestHongguoSM3MatchesSource(t *testing.T) {
	for _, item := range []struct{ text, want string }{
		{``, `1ab21d8355cfa17f8e61194831e81a8f22bec8c728fefb747ed035eb5082aa2b`},
		{`abc`, `66c7f0f462eeedd9d1f2d46bdc10e4e24167c4875cf2f7a2297da02b8f4ba8e0`},
		{`hongguo-medusa-向量`, `9d290d6eb5024c409756cf87f98a4dfd34952507bda2582221ad4c186e973af1`},
	} {
		digest := hongguoSM3([]byte(item.text))
		if got := hongguoTestHex(digest[:]); got != item.want {
			t.Fatalf("SM3(%q) = %s", item.text, got)
		}
	}
}

func TestHongguoMd5SumV3MatchesSource(t *testing.T) {
	for _, item := range []struct {
		message, orders string
		countV2         uint32
		countV1         uint32
		want            string
	}{
		{`0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef`, `03080d02070c01060b00050a0f04090e03080d02070c01060b00050a0f04090e03080d02070c01060b00050a0f04090e03080d02070c01060b00050a0f04090e`, 305419896, 7, `e1299448278d95df11f8080e24b8002e0451368c`},
		{`0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef`, `03080d02070c01060b00050a0f04090e03080d02070c01060b00050a0f04090e03080d02070c01060b00050a0f04090e03080d02070c01060b00050a0f04090e`, 3735928559, 200, `ad05e2b8f3f21871f26ac7649dede74d0e8cc7b5`},
		{`0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef`, `03080d02070c01060b00050a0f04090e03080d02070c01060b00050a0f04090e03080d02070c01060b00050a0f04090e03080d02070c01060b00050a0f04090e`, 1, 0, `51ffc78da613eb010d48c0d4fca3c9a4b753359b`},
	} {
		got := hongguoMd5SumV3(hongguoTestBytes(t, item.message), item.countV2, hongguoTestBytes(t, item.orders), item.countV1)
		if hongguoTestHex(got) != item.want {
			t.Fatalf("md5SumV3(countV2=%d, countV1=%d) = %s", item.countV2, item.countV1, hongguoTestHex(got))
		}
	}
}

func TestHongguoHashF13MatchesSource(t *testing.T) {
	for _, item := range []struct {
		query   string
		body    string
		khronos uint32
		branch  int
		want    string
	}{
		{`seed=0&device_id=7512345678901234567&ts=1760000000`, `7b2269223a307d`, 1760000000, 2, `6cf3febdbe6ccf000f4501e357be26fa97c927d1`},
		{`seed=3&device_id=7512345678901234567&ts=1760000003`, `7b2269223a337d`, 1760000003, 0, `c9e07816a4b01b2fa988542cf53a031c1cd27d4a`},
		{`seed=6&device_id=7512345678901234567&ts=1760000006`, `7b2269223a367d`, 1760000006, 1, `c5f7f2bb638dd98d53c791a105ce7aeb484f84c86f038f235d0deaab32e5def7455a7858`},
	} {
		queryDigest := hongguoSM3([]byte(item.query))
		bodyMd5 := hongguoMD5Sum(hongguoTestBytes(t, item.body))
		got := hongguoHashF13(queryDigest[:], bodyMd5[:], hongguoLe32(item.khronos), item.khronos)
		if hongguoTestHex(got) != item.want {
			t.Fatalf("hashF13(branch=%d) = %s", item.branch, hongguoTestHex(got))
		}
	}
}

func TestHongguoXmxorMatchesSource(t *testing.T) {
	got := hongguoXmxor(hongguoTestBytes(t, `a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90`), hongguoTestBytes(t, `00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff`))
	if hongguoTestHex(got) != `773d1596a38d3aaf1b7bdbba8235129cc8dbd5c651c2b2ca6e59e7ebe0e77631` {
		t.Fatalf("xmxor = %s", hongguoTestHex(got))
	}
}

func TestHongguoKeyHashMatchesSource(t *testing.T) {
	for _, item := range []struct {
		random     uint32
		hash, seed string
	}{
		{0, `076be6044a3906f60a21a0d621a56d597431af27d7170acd8b816bb60629f001`, `ffffffff`},
		{1, `9ec3ea534629da4f42fe29167a6c7749d98d348ec0a5dc8ac4ebd05e8105594b`, `ffffffff`},
		{2309737967, `9e72712876094f368da81a840329aad052d2eb8aa9d91408374af6f97e98f67f`, `d8a7faff`},
	} {
		hash, seed := hongguoKeyHash(hongguoMedusaSignKey, item.random)
		if hongguoTestHex(hash) != item.hash || hongguoTestHex(seed) != item.seed {
			t.Fatalf("keyHash(%d) = %s/%s", item.random, hongguoTestHex(hash), hongguoTestHex(seed))
		}
	}
}

func TestHongguoAesV3MatchesSource(t *testing.T) {
	for _, item := range []struct {
		wordSize      uint32
		key, iv, data string
		want          string
	}{
		{0, `f1593376766ea98d34f31b057a9d5be4`, `1fe109a4125283f418de9e051a969e12`, `0b30557a9fc4e90e33587da2c7ec11365b80a5caef14395e83a8cdf2173c6186abd0f51a3f6489aed3f81d42678cb1d6fb20456a8fb4d9fe23486d92b7dc01264b7095badf04294e7398bde2072c51769bc0e50a2f54799ec3e80d32577ca1c6`, `b00b70517a97c6e88e33187db2cfee10b65bc0a1dae71638dea3a8c9f21f3e60868bd0f51a376688aed3b81952678cb0d6fb20457a8fb6d9fe03486d82b7dc00264b3091aadf06284e53d8bde20f2c5076bb80e51a2754799ec3e80d225f7ca0c6`},
		{1, `f1593376766ea98d34f31b057a9d5be4`, `1fe109a4125283f418de9e051a969e12`, `0b30557a9fc4e90e33587da2c7ec11365b80a5caef14395e83a8cdf2173c6186abd0f51a3f6489aed3f81d42678cb1d6fb20456a8fb4d9fe23486d92b7dc01264b7095badf04294e7398bde2072c51769bc0e50a2f54799ec3e80d32577ca1c6`, `f32b70557a9fc6e80e13587db2c7ee10b65bc0a5cae71438de83a8c9e21f3c61068b90f50a3f66882ef3b81d42678eb1d6fb20457a87b6d9fe03086d92bfdc00266b7091aadf06294e53d8bdf2072c51769bc0e51a2754789ee3a80932577ca146`},
		{2, `f1593376766ea98d34f31b057a9d5be4`, `1fe109a4125283f418de9e051a969e12`, `0b30557a9fc4e90e33587da2c7ec11365b80a5caef14395e83a8cdf2173c6186abd0f51a3f6489aed3f81d42678cb1d6fb20456a8fb4d9fe23486d92b7dc01264b7095badf04294e7398bde2072c51769bc0e50a2f54799ec3e80d32577ca1c6`, `450b30516a97c6e80e131879a2c7ec11367b80a1daef1639de83a8c9f2173e60868b90f51a3f66882ed3f819526f8eb156fb60457a8fb6d9fe23486992bfde01266b7091badf0628ce73d8bdf20f2c5176bb80e51a2f54781ee3e80d32577ca1c6`},
		{3, `f1593376766ea98d34f31b057a9d5be4`, `1fe109a4125283f418de9e051a969e12`, `0b30557a9fc4e90e33587da2c7ec11365b80a5caef14395e83a8cdf2173c6186abd0f51a3f6489aed3f81d42678cb1d6fb20456a8fb4d9fe23486d92b7dc01264b7095badf04294e7398bde2072c51769bc0e50a2f54799ec3e80d32577ca1c6`, `8c2b30517a97c4e98e135879a2cfee11367bc0a5dae716385ea3a8cdf2173c6186ab90f11a3f66882ed3b81d426f8eb056db60416a87b4d8fe03486992bfdc00264b7091aad706284e73d8b9e2072c50f6bb80e10a2f56791ee3e809225f7ca046`},
	} {
		cipher := newHongguoAesV3(hongguoTestBytes(t, item.key), item.wordSize)
		got := cipher.encrypt(hongguoTestBytes(t, item.data), hongguoTestBytes(t, item.iv))
		if hongguoTestHex(got) != item.want {
			t.Fatalf("AesV3(wordSize=%d) = %s", item.wordSize, hongguoTestHex(got))
		}
	}
}

func TestHongguoDeviceProtoMatchesSource(t *testing.T) {
	got := hongguoDeviceProto(`7512345678901234567`, `fixed-nonce-abcdefgh`, 1760987654321)
	if hongguoTestHex(got) != `080210041a04383636322213373531323334353637383930313233343536372a1466697865642d6e6f6e63652d61626364656667683208216e6f7065726d2138efc06c40efc06c480650efc06c5a08216e6f7473657421620f417369612f5368616e676861692c386a057a685f434e70088501000000008d01000000009501000000009d0100000000a50100000000ad0100000000b201023136b80152c00148c801e2a2d9b0c066d001e2a2d9b0c066d801e2a2d9b0c066e001e2a2d9b0c066e80101f2010a32353035335254343743fa01055265646d6982020a323530353352543437438a020a323530353352543437439202065869616f6d699a02055265646d69a202055265646d69b0023e` {
		t.Fatalf("deviceProto = %s", hongguoTestHex(got))
	}
}

func TestHongguoHeliosMatchesSource(t *testing.T) {
	for _, item := range []struct {
		khronos, random uint32
		want            string
	}{
		{1760000000, 305419896, `eFY0EhxUiWEJ5myK/ec6gVptoBFl2JqeXVHnmakVf60TcY/S`},
		{1760987654, 3735928559, `776t3ug6Xq5oxkHMRcW80wDyM0ppAeumNk2vl4wou4crWkgq`},
	} {
		if got := hongguoHelios(item.khronos, item.random); got != item.want {
			t.Fatalf("helios(%d, %d) = %s", item.khronos, item.random, got)
		}
	}
}

func TestHongguoXGorgonMatchesSource(t *testing.T) {
	for _, item := range []struct {
		query   string
		body    string
		khronos uint32
		random  uint32
		want    string
	}{
		{`aid=8662&device_id=7512345678901234567&ts=1760000000&_rticket=1760987654321`, `7b226d697865645f766964656f5f69645f6d6170223a7b2231303034223a5b2237353132333435363738393031323334353637225d7d7d`, 1760000000, 4660, `84043412400136f72ab9829d75af1364e71df6105af6342a8a86`},
		{`aid=8662&device_id=7512345678901234567&ts=1760000000&_rticket=1760987654321`, `7b226d697865645f766964656f5f69645f6d6170223a7b2231303034223a5b2237353132333435363738393031323334353637225d7d7d`, 1760987654, 48879, `8404efbe40017e72b2ec5962e0b888979b7509843fd0a768c685`},
	} {
		got := hongguoXGorgon(item.query, hongguoTestBytes(t, item.body), item.khronos, item.random)
		if got != item.want {
			t.Fatalf("xGorgon = %s", got)
		}
	}
	if got := hongguoXGorgon(`aid=8662&device_id=7512345678901234567&ts=1760000000&_rticket=1760987654321`, nil, 1760000000, 3855); got != `84040f0f40018fa9ddfa6ec32b6dc93fa3a3b0015ef9668b935e` {
		t.Fatalf("xGorgon(空 body) = %s", got)
	}
}

func TestHongguoBuildMedusaMatchesSource(t *testing.T) {
	rnd := hongguoMedusaRandom{
		MessageRand: 287454020, EnvLaunch: 105, EnvPid: 12345,
		Random: 1432778632, XmRand: 2578103244,
		Nonce: `fixed-nonce-abcdefgh`, Started: 1760987654321, Now: 1760987654,
	}
	got := hongguoBuildMedusa(`https://api5-normal-sinfonlineb.fqnovel.com/novel/player/multi_video_model/v1/?aid=8662&device_id=7512345678901234567&ts=1760000000&_rticket=1760987654321`, hongguoTestBytes(t, `7b2262697a5f706172616d223a7b22766964656f5f706c6174666f726d223a313032347d2c226d697865645f766964656f5f69645f6d6170223a7b2231303034223a5b2237353132333435363738393031323334353637225d7d7d`), 1760000000, `7512345678901234567`, `7.3.5.32`, rnd)
	if got != `A3jnaPeQuJLXrztT1lIvGFe3hnCIdwABszXMv6qZAWoJGNnbylUVG6FGjl7EHJhyOLXgDDaLIc/p05QMw2n4ZlNvUgHBPZ0xGUbaYdW2/wT0IAW6vU/RfJKx+ZXtOnHpv8DgJYuScgVT6mJC6RnbAecR3nLEqmdpyrJowJJNetaCBzOFu5h2S7O41zmg+45ZbIyhqEJkChdUex3uo7okgOQFkD/vw9tIdbrBY6I6RQ6ziAWescayCOjx4pyo5xh8r5u5uZf56EWUGAUfcmMEm2ZJVFeLdfhvNmv38aQhWBVbAGOmm1s35Qlbf700kiGqRlIOl3imjIGJIDEL+iird4UeoQxLPWuyfIqXNXKvPKf+P65hAZ4NNAG3TO4Nl2KgBYJcgaxAnbWwqsv+6wOYtyE4Gr0LWOghVJanmwTs/pbkW+fOimGcQ5Jmf/bavTs6+5QVyxWo16k95sxJRIPGH+Dp6GZaW4OzQ7nlJ9CAFXiA1F3T1nteEANzDRGf9mZlEkvSEGSdKV8pcWOdh/2CEqId9fDAVQKdg8SeX+qrxtru+5iTO2LADLBfC7D2MXnLzQU3AuxX2zAzxkQhg7hGK88yh12rwDQYSF7r5CIv6ozhnOyWvsrK5Q1avgWIooNq3c7jjOGlgbICA4QcWkf5l1sUjz1AvkBFjBw24N8/r61E2uCYwhfnjwKmJiF4W6tflye1k/ZKImIEuH5MPBpa+q6NWYDdzrPAy+2MCxtkEPvLq778FfxX6KavF2iO59h06DInIFfzS/xRa1IYddJEn3uQj5g+pT7pulrI6+EioYb7eEU638DfIZ2ACX501BE6jm5357Wp3oOW9FPj66YeRH1S1/bNa2CkPZ55aO0zjG4KOX50Bof/jPbBfWOgSPvMXLXALw74IOpAdcxVlYlitGqOXVYWEiKv8Lacm6Lm7+45hbqObawAqbUMooG5vwu7tHrwJLRqRXpLza2JgmYPR0eBMJIzzGb5Gc///M/P//2PZlU=` {
		t.Fatalf("buildMedusa = %s", got)
	}
}
