package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

var (
	ErrConfigMissing    = errors.New("missing SSO config")
	ErrInvalidConfig    = errors.New("invalid SSO config")
	ErrInvalidBase64    = errors.New("invalid base64 encoding")
	ErrDecryptFailed    = errors.New("decryption failed")
	ErrInvalidPayload   = errors.New("invalid payload structure")
	ErrInvalidHMAC      = errors.New("HMAC verification failed")
	ErrTimestampExpired = errors.New("timestamp outside allowed window")
)

type BratConfig struct {
	HashSecret        string
	EncryptionKey     string
	EncryptionIV      string
	Cipher            string
	MaxSkewSeconds    int
	HashLength        int
	HashBytes         int
	MaxPseudonimBytes int
}

type BratUser struct {
	Pseudonim string
	Punktacja int64
	Timestamp int64
}

type BratDecoder struct {
	cfg BratConfig
	key []byte
	iv  []byte
}

func NewBratDecoder(cfg BratConfig) (*BratDecoder, error) {
	disabled := cfg.HashSecret == "" && cfg.EncryptionKey == "" && cfg.EncryptionIV == "" && cfg.Cipher == ""
	if disabled {
		return nil, nil
	}
	if cfg.HashSecret == "" || cfg.EncryptionKey == "" || cfg.EncryptionIV == "" || cfg.Cipher == "" {
		return nil, ErrConfigMissing
	}
	if cfg.HashLength <= 0 || cfg.HashBytes <= 0 || cfg.HashLength != cfg.HashBytes*2 {
		return nil, ErrInvalidConfig
	}
	if cfg.HashBytes > sha256.Size {
		return nil, ErrInvalidConfig
	}

	if !strings.EqualFold(cfg.Cipher, "AES-256-CBC") {
		return nil, ErrInvalidConfig
	}
	if len(cfg.EncryptionIV) != aes.BlockSize {
		return nil, ErrInvalidConfig
	}

	key := sha256.Sum256([]byte(cfg.EncryptionKey))
	return &BratDecoder{
		cfg: cfg,
		key: key[:],
		iv:  []byte(cfg.EncryptionIV),
	}, nil
}

func (d *BratDecoder) Decode(data string) (*BratUser, error) {
	if d == nil {
		return nil, ErrConfigMissing
	}

	decoded, err := d.decodeBase64(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidBase64, err)
	}

	decrypted, err := d.decrypt(decoded)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptFailed, err)
	}

	parsed, err := parseBinaryPayload(decrypted, d.cfg.HashBytes, d.cfg.MaxPseudonimBytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
	}

	if !d.verifyHMAC(parsed) {
		return nil, ErrInvalidHMAC
	}

	if !d.verifyTimestamp(parsed.Timestamp) {
		return nil, ErrTimestampExpired
	}

	return &BratUser{
		Pseudonim: parsed.Pseudonim,
		Punktacja: parsed.Punktacja,
		Timestamp: parsed.Timestamp,
	}, nil
}

func (d *BratDecoder) decodeBase64(data string) ([]byte, error) {
	raw, err := url.PathUnescape(data)
	if err != nil {
		return nil, err
	}
	normalized := normalizeBase64(raw)
	return base64.StdEncoding.DecodeString(normalized)
}

func normalizeBase64(s string) string {
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")
	padLen := (4 - len(s)%4) % 4
	return s + strings.Repeat("=", padLen)
}

func (d *BratDecoder) decrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(d.key)
	if err != nil {
		return nil, err
	}

	if len(data) < aes.BlockSize || len(data)%aes.BlockSize != 0 {
		return nil, errors.New("invalid ciphertext length")
	}

	mode := cipher.NewCBCDecrypter(block, d.iv)
	decrypted := make([]byte, len(data))
	mode.CryptBlocks(decrypted, data)

	return pkcs7Unpad(decrypted, aes.BlockSize)
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, errors.New("invalid padding size")
	}
	padLen := int(data[len(data)-1])
	if padLen == 0 || padLen > blockSize {
		return nil, errors.New("invalid padding")
	}
	for _, b := range data[len(data)-padLen:] {
		if int(b) != padLen {
			return nil, errors.New("invalid padding")
		}
	}
	return data[:len(data)-padLen], nil
}

type parsedPayload struct {
	Timestamp int64
	Punktacja int64
	Pseudonim string
	Hash      string
}

func parseBinaryPayload(data []byte, hashBytes, maxPseudonimBytes int) (*parsedPayload, error) {
	if len(data) < 4+8+1+hashBytes {
		return nil, errors.New("payload too short")
	}

	timestamp := int64(binary.BigEndian.Uint32(data[0:4]))

	high := int64(binary.BigEndian.Uint32(data[4:8]))
	low := int64(binary.BigEndian.Uint32(data[8:12]))
	punktacja := (high << 32) | low

	pseudonimLen := int(data[12])
	if pseudonimLen <= 0 {
		return nil, errors.New("empty pseudonim")
	}
	if maxPseudonimBytes > 0 && pseudonimLen > maxPseudonimBytes {
		return nil, errors.New("pseudonim too long")
	}

	expectedLen := 4 + 8 + 1 + pseudonimLen + hashBytes
	if len(data) != expectedLen {
		return nil, errors.New("payload length mismatch")
	}

	pseudonim := string(data[13 : 13+pseudonimLen])
	hashStart := 13 + pseudonimLen
	hash := hex.EncodeToString(data[hashStart : hashStart+hashBytes])

	return &parsedPayload{
		Timestamp: timestamp,
		Punktacja: punktacja,
		Pseudonim: pseudonim,
		Hash:      hash,
	}, nil
}

func (d *BratDecoder) verifyHMAC(p *parsedPayload) bool {
	source := fmt.Sprintf("%d|%d|%s", p.Timestamp, p.Punktacja, p.Pseudonim)
	mac := hmac.New(sha256.New, []byte(d.cfg.HashSecret))
	mac.Write([]byte(source))
	expected := hex.EncodeToString(mac.Sum(nil))[:d.cfg.HashLength]
	return subtle.ConstantTimeCompare([]byte(expected), []byte(p.Hash)) == 1
}

func (d *BratDecoder) verifyTimestamp(ts int64) bool {
	if d.cfg.MaxSkewSeconds <= 0 {
		return true
	}
	now := time.Now().Unix()
	maxSkew := int64(d.cfg.MaxSkewSeconds)
	return ts >= now-maxSkew && ts <= now+maxSkew
}
