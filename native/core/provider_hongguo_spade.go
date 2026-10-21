package core

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"
)

// 红果 App 片源接口返回的 main_url / backup_url 并不是明文地址，而是
// "spade" 密文：base64 解码后以 a8 00 01 00 开头，其后是 AES-128-CBC 密文。
// 解密种子由响应里的 video_info.data.key_seed（base64）提供，工作密钥由
// SHA-512(key_seed) 与一组固定常量再做一次 SHA-512 派生得到。
//
// 该算法在公开项目 jackotom/hongguo-drama（GPL-3.0）的
// src/native/app-source.js:decryptSpadeUrl 中实现，此处按相同语义移植。
var hongguoSpadeURLConstants = func() []byte {
	decoded, err := hex.DecodeString("4dd4c2e6b83162090e52b3c7a6733ba41cb2462b829ab58a196b39db57177524f49baf7f08e8d68d26a72e37c1a95a2f1f05a51892aef2949732b62a38aadd58")
	if err != nil {
		return nil
	}
	return decoded
}()

func hongguoDecryptSpadeURL(value string, seed []byte) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("红果片源地址为空")
	}
	// 部分响应直接给明文地址，保持兼容。
	if isProviderHTTPMediaURL(value) {
		return value, nil
	}
	raw, err := decodeHongguoBase64(value)
	if err != nil || len(raw) < 5 {
		return "", errors.New("红果片源地址编码无效")
	}
	if raw[0] != 0xa8 || raw[2] != 0x01 || raw[3] != 0x00 {
		return "", errors.New("红果片源地址头部格式异常")
	}
	cipherLength := (len(raw) - 4) / aes.BlockSize * aes.BlockSize
	if cipherLength <= 0 {
		return "", errors.New("红果片源地址密文不完整")
	}
	if len(seed) == 0 || len(hongguoSpadeURLConstants) == 0 {
		return "", errors.New("红果片源地址缺少解密种子")
	}
	first := sha512.Sum512(seed)
	hasher := sha512.New()
	hasher.Write(first[:])
	hasher.Write(hongguoSpadeURLConstants)
	derived := hasher.Sum(nil)
	block, err := aes.NewCipher(derived[:aes.BlockSize])
	if err != nil {
		return "", err
	}
	plain := make([]byte, cipherLength)
	cipher.NewCBCDecrypter(block, derived[aes.BlockSize:2*aes.BlockSize]).CryptBlocks(plain, raw[4:4+cipherLength])
	// 与 JS 版保持一致：不做严格 PKCS#7 校验，末字节落在 1..16 即视为填充长度，
	// 随后再去掉尾部零字节。地址本身是 ASCII，不会与零填充混淆。
	if count := len(plain); count > 0 {
		if padding := int(plain[count-1]); padding >= 1 && padding <= aes.BlockSize && padding <= count {
			plain = plain[:count-padding]
		}
	}
	for len(plain) > 0 && plain[len(plain)-1] == 0 {
		plain = plain[:len(plain)-1]
	}
	address := strings.TrimSpace(string(plain))
	if !isProviderHTTPMediaURL(address) {
		return "", errors.New("红果片源地址解密失败")
	}
	return address, nil
}

// hongguoSpadeHostAllowed 限定片源只允许落到字节系视频 CDN，
// 避免被中间人把播放地址改写到任意主机。
func hongguoSpadeHostAllowed(address string) bool {
	parsed, err := url.Parse(address)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return false
	}
	for _, domain := range []string{"qznovelvod.com", "fqnovelvod.com", "douyinvod.com", "snssdk.com", "fqnovel.com"} {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}
