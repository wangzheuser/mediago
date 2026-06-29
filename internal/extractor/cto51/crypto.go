package cto51

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"sync"
)

const cto51RSAPublicKeyPEM = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQC3pDA7GTxOvNbXRGMi9QSIzQEI
+EMD1HcUPJSQSFuRkZkWo4VQECuPRg/xVjqwX1yUrHUvGQJsBwTS/6LIcQiSwYsO
qf+8TWxGQOJyW46gPPQVzTjNTiUoq435QB0v11lNxvKWBQIZLmacUZ2r1APta7i/
MY4Lx9XlZVMZNUdUywIDAQAB
-----END PUBLIC KEY-----`

var cto51RSAPublicKeyCache struct {
	once sync.Once
	key  *rsa.PublicKey
}

func rsaEncryptOverlay(overlay string) string {
	key := cto51PublicKey()
	if key == nil || overlay == "" {
		return ""
	}
	ciphertext, err := rsa.EncryptPKCS1v15(rand.Reader, key, []byte(overlay))
	if err != nil {
		return ""
	}
	return hex.EncodeToString(ciphertext)
}

func cto51PublicKey() *rsa.PublicKey {
	cto51RSAPublicKeyCache.once.Do(func() {
		block, _ := pem.Decode([]byte(cto51RSAPublicKeyPEM))
		if block == nil {
			return
		}
		pub, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return
		}
		if key, ok := pub.(*rsa.PublicKey); ok {
			cto51RSAPublicKeyCache.key = key
		}
	})
	return cto51RSAPublicKeyCache.key
}
