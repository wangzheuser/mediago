package kuke

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strings"

	_ "crypto/sha1"
	_ "crypto/sha256"
)

const (
	kukePrivateKeyPEM = `-----BEGIN PRIVATE KEY-----
MIICdwIBADANBgkqhkiG9w0BAQEFAASCAmEwggJdAgEAAoGBAPbm41E9lg04G9XL
WcV5LJDDejb+rkwnPJr2IygmQIYLuLKHU0MAijEmaamL070bF7+iJeqDt5N/mZ7T
85i3Ykky9SaMuC9TbsdjokYink6ngscCev76ReF8NLK5y1vzMI5M5RpEXK339D9w
t9Q/DEYjVa605EQ5Xv3it5ONBVk7AgMBAAECgYBWY7U4ENd26qH6rXtMuDhasrsJ
kRVFehkfk237t16uSF2oweblM8QmrG0eMNm2ektV9xNTOiE6j9QdmcXLMqdFi4lu
XKTRFe9Qx97+EWmu3paxX6ZXpok0aSVIzGp6u+u5D+KJnJ3rFMx5EzlCqX18IcLN
wdxtGSJ+kJPPhiPeWQJBAPzzJB75NiQTEfcPfDoM13fYEVYZBOpghnBNOPaA0qYQ
Q91mYuLW/Rm0sTI5H6ZC8JW1bUyUitQqD9Mjtt/lpKcCQQD54RO4hR0f27TO2hrD
puLQiGGJxxinEXngmkzPHy9kyd7518PzJlT+AOH89X2sT37NGtRiYZ7UgSY/guRH
ifVNAkEAteTF6bv9kc1g0s+A3mGTo+ts9APDxCKrKiBtwNz8HUx+8LuKimJc2NpV
va7UMoPaa11ufm4mstCYVpVNEQ4a6wJAXM2vGVS24GIk4L44Onn8ux4ru5PqIAJp
lXU5GaOnYnNnELuF1wRhhISnad9y8VAE9AAG6RMAfkQJBIWEat1d8QJBAPb/oHbl
onkrpwIrUX8yVvCDSr9JJF4ei8xfMSGXtElJuSKLrqh5cSGWphpu+T8+6MJfgSrz
8c7aTkk7cvRs/8g=
-----END PRIVATE KEY-----`
	kukePolyvIVHex = "01020305070B0D1113171D0705030201"
)

func kukeDecodeSecurePayload(kkAes, kkSDKString string) map[string]any {
	kkAes = strings.TrimSpace(kkAes)
	kkSDKString = strings.TrimSpace(kkSDKString)
	if kkAes == "" || kkSDKString == "" {
		return nil
	}
	priv, err := parseKukePrivateKey()
	if err != nil {
		return nil
	}
	encryptedKey, err := base64.StdEncoding.DecodeString(kkAes)
	if err != nil {
		return nil
	}
	keyMaterial, err := priv.Decrypt(rand.Reader, encryptedKey, &rsa.OAEPOptions{Hash: crypto.SHA256, MGFHash: crypto.SHA1})
	if err != nil || len(keyMaterial) < 32 {
		return nil
	}
	key, iv := keyMaterial[:16], keyMaterial[16:32]
	encryptedSDK, err := base64.StdEncoding.DecodeString(kkSDKString)
	if err != nil || len(encryptedSDK) == 0 || len(encryptedSDK)%aes.BlockSize != 0 {
		return nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil
	}
	plain := make([]byte, len(encryptedSDK))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, encryptedSDK)
	plain = pkcs7Unpad(plain, aes.BlockSize)
	var out map[string]any
	if err := json.Unmarshal(plain, &out); err != nil {
		return nil
	}
	return out
}

func kukeDecodePolyvSecureInfo(videoID string, info map[string]any) map[string]any {
	if len(info) == 0 {
		return map[string]any{}
	}
	if data := mapAny(info["data"]); len(data) > 0 {
		if len(polyvHLSList(data)) > 0 || data["seed_const"] != nil || mapAny(data["playsafe"])["token"] != nil {
			return data
		}
	}
	if len(polyvHLSList(info)) > 0 || info["seed_const"] != nil {
		return info
	}
	bodyHex := firstText(info["body"])
	secureVID := kukeSecureVID(videoID)
	if bodyHex == "" || secureVID == "" {
		return map[string]any{}
	}
	ciphertext, err := hex.DecodeString(bodyHex)
	if err != nil || len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return map[string]any{}
	}
	sum := md5.Sum([]byte(secureVID))
	keyHex := hex.EncodeToString(sum[:])
	block, err := aes.NewCipher([]byte(keyHex[:16]))
	if err != nil {
		return map[string]any{}
	}
	plain := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, []byte(keyHex[16:32])).CryptBlocks(plain, ciphertext)
	for _, candidate := range [][]byte{pkcs7Unpad(plain, aes.BlockSize), []byte(strings.TrimRight(string(plain), "\x00"))} {
		if decoded := kukeDecodePolyvSecurePayload(candidate); len(decoded) > 0 {
			return decoded
		}
	}
	return map[string]any{}
}

func kukeDecodePolyvSecurePayload(raw []byte) map[string]any {
	raw = []byte(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return nil
	}
	decoded := raw
	if !json.Valid(decoded) {
		b, err := base64.StdEncoding.DecodeString(string(raw))
		if err != nil {
			return nil
		}
		decoded = b
	}
	var out map[string]any
	if err := json.Unmarshal(decoded, &out); err != nil {
		return nil
	}
	return out
}

func parseKukePrivateKey() (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(kukePrivateKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("kuke private key PEM decode failed")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	priv, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("kuke private key is not RSA")
	}
	return priv, nil
}

func pkcs7Unpad(data []byte, blockSize int) []byte {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return data
	}
	pad := int(data[len(data)-1])
	if pad == 0 || pad > blockSize || pad > len(data) {
		return data
	}
	for _, b := range data[len(data)-pad:] {
		if int(b) != pad {
			return data
		}
	}
	return data[:len(data)-pad]
}

func kukeDecryptPolyvKey(keyBytes []byte, seedConst any) []byte {
	if len(keyBytes) != 32 {
		return nil
	}
	trySeed := func(seed any) []byte {
		sum := md5.Sum([]byte(firstText(seed)))
		key := []byte(hex.EncodeToString(sum[:])[:16])
		iv, err := hex.DecodeString(kukePolyvIVHex)
		if err != nil {
			return nil
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil
		}
		out := make([]byte, len(keyBytes))
		cipher.NewCBCDecrypter(block, iv).CryptBlocks(out, keyBytes)
		out = pkcs7Unpad(out, aes.BlockSize)
		if len(out) == 16 {
			return out
		}
		return nil
	}
	if key := trySeed(seedConst); len(key) == 16 {
		return key
	}
	seedText := firstText(seedConst)
	for i := 0; i < 1000; i++ {
		if fmt.Sprint(i) == seedText {
			continue
		}
		if key := trySeed(i); len(key) == 16 {
			return key
		}
	}
	return nil
}
