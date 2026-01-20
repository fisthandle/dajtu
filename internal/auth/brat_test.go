package auth

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestNewBratDecoder_Disabled(t *testing.T) {
	decoder, err := NewBratDecoder(BratConfig{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if decoder != nil {
		t.Fatal("expected nil decoder for empty config")
	}
}

func TestNewBratDecoder_InvalidConfig(t *testing.T) {
	_, err := NewBratDecoder(BratConfig{
		HashSecret:    "secret",
		EncryptionKey: "key",
		EncryptionIV:  "1234567890123456",
		Cipher:        "AES-256-CBC",
		HashLength:    10,
		HashBytes:     6,
	})
	if err == nil {
		t.Fatal("expected error for hash config mismatch")
	}
}

func TestNewBratDecoder_InvalidIVLength(t *testing.T) {
	_, err := NewBratDecoder(BratConfig{
		HashSecret:    "secret",
		EncryptionKey: "key",
		EncryptionIV:  "short",
		Cipher:        "AES-256-CBC",
		HashLength:    10,
		HashBytes:     5,
	})
	if err == nil {
		t.Fatal("expected error for invalid IV length")
	}
}

func TestDecodeBratPayload_InvalidBase64(t *testing.T) {
	decoder := mustDecoder(t, testConfig())

	_, err := decoder.Decode("!!!invalid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestDecodeBratPayload_ValidPayload(t *testing.T) {
	cfg := testConfig()
	decoder := mustDecoder(t, cfg)

	ts := uint32(time.Now().Unix())
	payload := buildPayload(t, cfg, ts, 100, "test")

	user, err := decoder.Decode(payload)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if user.Pseudonim != "test" {
		t.Fatalf("Pseudonim = %q, want %q", user.Pseudonim, "test")
	}
	if user.Punktacja != 100 {
		t.Fatalf("Punktacja = %d, want 100", user.Punktacja)
	}
	if user.Timestamp != int64(ts) {
		t.Fatalf("Timestamp = %d, want %d", user.Timestamp, ts)
	}
}

func TestDecodeBratPayload_PseudonimTooLong(t *testing.T) {
	cfg := testConfig()
	cfg.MaxPseudonimBytes = 3
	decoder := mustDecoder(t, cfg)

	ts := uint32(time.Now().Unix())
	payload := buildPayload(t, cfg, ts, 100, "test")

	_, err := decoder.Decode(payload)
	if err == nil {
		t.Fatal("expected error for pseudonim too long")
	}
}

func testConfig() BratConfig {
	return BratConfig{
		HashSecret:        "70bea0d8db1666ebb2c0aef1a820be13d8274dca73843935336f2dbb0e2f4f7e",
		EncryptionKey:     "8d29fa415eda466a6e54d2843b5d219295de5fe9c5a52330c88a9f9b7107b0e4",
		EncryptionIV:      "eOnipVEZxUk52ezJ",
		Cipher:            "AES-256-CBC",
		MaxSkewSeconds:    600,
		HashLength:        10,
		HashBytes:         5,
		MaxPseudonimBytes: 255,
	}
}

func mustDecoder(t *testing.T, cfg BratConfig) *BratDecoder {
	t.Helper()
	decoder, err := NewBratDecoder(cfg)
	if err != nil {
		t.Fatalf("NewBratDecoder error: %v", err)
	}
	if decoder == nil {
		t.Fatal("NewBratDecoder returned nil")
	}
	return decoder
}

func buildPayload(t *testing.T, cfg BratConfig, ts uint32, punktacja uint64, pseudonim string) string {
	t.Helper()

	payload := make([]byte, 0, 4+8+1+len(pseudonim)+cfg.HashBytes)
	tsBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(tsBytes, ts)
	payload = append(payload, tsBytes...)

	scoreBytes := make([]byte, 8)
	binary.BigEndian.PutUint32(scoreBytes[0:4], uint32(punktacja>>32))
	binary.BigEndian.PutUint32(scoreBytes[4:8], uint32(punktacja))
	payload = append(payload, scoreBytes...)

	payload = append(payload, byte(len(pseudonim)))
	payload = append(payload, []byte(pseudonim)...)

	source := fmt.Sprintf("%d|%d|%s", ts, punktacja, pseudonim)
	mac := hmac.New(sha256.New, []byte(cfg.HashSecret))
	mac.Write([]byte(source))
	hashBytes := mac.Sum(nil)[:cfg.HashBytes]
	payload = append(payload, hashBytes...)

	key := sha256.Sum256([]byte(cfg.EncryptionKey))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		t.Fatalf("NewCipher error: %v", err)
	}

	padded := pkcs7Pad(payload, aes.BlockSize)
	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, []byte(cfg.EncryptionIV))
	mode.CryptBlocks(ciphertext, padded)

	encoded := base64.StdEncoding.EncodeToString(ciphertext)
	encoded = strings.TrimRight(encoded, "=")
	encoded = strings.NewReplacer("+", "-", "/", "_").Replace(encoded)
	return encoded
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padLen := blockSize - (len(data) % blockSize)
	padding := bytes.Repeat([]byte{byte(padLen)}, padLen)
	return append(data, padding...)
}
