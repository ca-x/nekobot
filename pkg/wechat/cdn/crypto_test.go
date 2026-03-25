package cdn

import (
	"encoding/base64"
	"testing"
)

func TestParseAESKeySupportsRawAndHex(t *testing.T) {
	t.Parallel()

	raw := []byte("1234567890abcdef")

	got, err := ParseAESKey(base64.StdEncoding.EncodeToString(raw))
	if err != nil {
		t.Fatalf("ParseAESKey(raw) failed: %v", err)
	}
	if string(got) != string(raw) {
		t.Fatalf("unexpected raw key: %q", got)
	}

	hexEncoded := base64.StdEncoding.EncodeToString([]byte("31323334353637383930616263646566"))
	got, err = ParseAESKey(hexEncoded)
	if err != nil {
		t.Fatalf("ParseAESKey(hex) failed: %v", err)
	}
	if string(got) != string(raw) {
		t.Fatalf("unexpected hex key: %q", got)
	}
}

func TestAESECBRoundTrip(t *testing.T) {
	t.Parallel()

	key := []byte("1234567890abcdef")
	plain := []byte("hello wechat sdk")

	ciphertext, err := AESECBEncrypt(plain, key)
	if err != nil {
		t.Fatalf("AESECBEncrypt failed: %v", err)
	}

	got, err := AESECBDecrypt(ciphertext, key)
	if err != nil {
		t.Fatalf("AESECBDecrypt failed: %v", err)
	}

	if string(got) != string(plain) {
		t.Fatalf("unexpected plaintext: %q", got)
	}
}
