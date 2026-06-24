package util

import (
	"crypto/aes"
	"crypto/cipher"
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello world", "hello world"},
		{"file<name>:test", "file_name__test"},
		{"", "untitled"},
		{"  spaces  ", "spaces"},
		{"a/b\\c", "a_b_c"},
	}
	for _, tt := range tests {
		got := SanitizeFilename(tt.input)
		if got != tt.want {
			t.Errorf("SanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestAESDecryptCBC(t *testing.T) {
	key := []byte("1234567890123456")
	iv := []byte("1234567890123456")
	plain := []byte("hello medigo!!!")

	// pad manually
	padLen := aes.BlockSize - len(plain)%aes.BlockSize
	padded := make([]byte, len(plain)+padLen)
	copy(padded, plain)
	for i := len(plain); i < len(padded); i++ {
		padded[i] = byte(padLen)
	}

	// encrypt
	block, _ := aes.NewCipher(key)
	encrypted := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(encrypted, padded)

	// decrypt
	decrypted, err := AESDecryptCBC(encrypted, key, iv)
	if err != nil {
		t.Fatalf("AESDecryptCBC error: %v", err)
	}
	if string(decrypted) != string(plain) {
		t.Errorf("got %q, want %q", decrypted, plain)
	}
}

func TestAESDecryptECB(t *testing.T) {
	key := []byte("1234567890123456")
	plain := []byte("0123456789abcdef") // exactly one block, no padding needed

	// pad (already block-aligned, padding = full block)
	padLen := aes.BlockSize
	padded := make([]byte, len(plain)+padLen)
	copy(padded, plain)
	for i := len(plain); i < len(padded); i++ {
		padded[i] = byte(padLen)
	}

	// encrypt ECB
	block, _ := aes.NewCipher(key)
	encrypted := make([]byte, len(padded))
	bs := block.BlockSize()
	for i := 0; i < len(padded); i += bs {
		block.Encrypt(encrypted[i:i+bs], padded[i:i+bs])
	}

	decrypted, err := AESDecryptECB(encrypted, key)
	if err != nil {
		t.Fatalf("AESDecryptECB error: %v", err)
	}
	if string(decrypted) != string(plain) {
		t.Errorf("got %q, want %q", decrypted, plain)
	}
}

func TestRandomUA(t *testing.T) {
	ua := RandomUA()
	if len(ua) < 20 {
		t.Error("RandomUA returned suspiciously short string")
	}
}

func TestBase64Decode(t *testing.T) {
	decoded, err := Base64Decode("aGVsbG8=")
	if err != nil {
		t.Fatalf("Base64Decode error: %v", err)
	}
	if string(decoded) != "hello" {
		t.Errorf("got %q, want %q", decoded, "hello")
	}
}

func TestParseProxyURL(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantScheme string
		wantHost   string
		wantErr    bool
	}{
		{name: "empty", input: ""},
		{name: "http default", input: "127.0.0.1:7890", wantScheme: "http", wantHost: "127.0.0.1:7890"},
		{name: "socks5h normalized", input: "socks5h://127.0.0.1:7897", wantScheme: "socks5", wantHost: "127.0.0.1:7897"},
		{name: "unsupported", input: "ftp://127.0.0.1:21", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseProxyURL(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseProxyURL(%q) expected error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseProxyURL(%q) error: %v", tt.input, err)
			}
			if tt.input == "" {
				if got != nil {
					t.Fatalf("ParseProxyURL(empty) = %v, want nil", got)
				}
				return
			}
			if got.Scheme != tt.wantScheme || got.Host != tt.wantHost {
				t.Fatalf("ParseProxyURL(%q) = %s://%s, want %s://%s", tt.input, got.Scheme, got.Host, tt.wantScheme, tt.wantHost)
			}
		})
	}
}
