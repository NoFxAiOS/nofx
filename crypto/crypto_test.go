package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func newTestCryptoService(t *testing.T) *CryptoService {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}

	return &CryptoService{
		privateKey: privateKey,
		publicKey:  &privateKey.PublicKey,
		dataKey:    make([]byte, 32),
	}
}

func mustMarshalAAD(t *testing.T, data *AADData) []byte {
	t.Helper()

	payload, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal AAD: %v", err)
	}
	return payload
}

func encryptPayload(
	t *testing.T,
	publicKey *rsa.PublicKey,
	plaintext []byte,
	aad []byte,
	payloadTS int64,
) *EncryptedPayload {
	t.Helper()

	aesKey := make([]byte, 32)
	if _, err := rand.Read(aesKey); err != nil {
		t.Fatalf("generate AES key: %v", err)
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("new GCM: %v", err)
	}

	iv := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(iv); err != nil {
		t.Fatalf("generate IV: %v", err)
	}

	ciphertext := gcm.Seal(nil, iv, plaintext, aad)

	wrappedKey, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, publicKey, aesKey, nil)
	if err != nil {
		t.Fatalf("wrap AES key: %v", err)
	}

	payload := &EncryptedPayload{
		WrappedKey: base64.RawURLEncoding.EncodeToString(wrappedKey),
		IV:         base64.RawURLEncoding.EncodeToString(iv),
		Ciphertext: base64.RawURLEncoding.EncodeToString(ciphertext),
		TS:         payloadTS,
	}

	if len(aad) > 0 {
		payload.AAD = base64.RawURLEncoding.EncodeToString(aad)
	}

	return payload
}

func TestDecryptPayloadValidAAD(t *testing.T) {
	cs := newTestCryptoService(t)
	ts := time.Now().Unix()

	aadData := &AADData{
		UserID:    "user-1",
		SessionID: "session-1",
		TS:        ts,
		Purpose:   AADPurposeSensitiveData,
	}
	payload := encryptPayload(t, cs.publicKey, []byte("secret"), mustMarshalAAD(t, aadData), ts)

	expected := &AADData{
		UserID:    "user-1",
		SessionID: "session-1",
		Purpose:   AADPurposeSensitiveData,
	}
	plaintext, err := cs.DecryptPayload(payload, expected)
	if err != nil {
		t.Fatalf("decrypt payload: %v", err)
	}
	if string(plaintext) != "secret" {
		t.Fatalf("unexpected plaintext: %s", plaintext)
	}
}

func TestDecryptPayloadMissingAAD(t *testing.T) {
	cs := newTestCryptoService(t)
	ts := time.Now().Unix()
	payload := encryptPayload(t, cs.publicKey, []byte("secret"), nil, ts)

	expected := &AADData{
		UserID:  "user-1",
		Purpose: AADPurposeSensitiveData,
	}
	_, err := cs.DecryptPayload(payload, expected)
	if err == nil || !strings.Contains(err.Error(), "missing AAD") {
		t.Fatalf("expected missing AAD error, got: %v", err)
	}
}

func TestDecryptPayloadUserMismatch(t *testing.T) {
	cs := newTestCryptoService(t)
	ts := time.Now().Unix()

	aadData := &AADData{
		UserID:  "user-1",
		TS:      ts,
		Purpose: AADPurposeSensitiveData,
	}
	payload := encryptPayload(t, cs.publicKey, []byte("secret"), mustMarshalAAD(t, aadData), ts)

	expected := &AADData{
		UserID:  "user-2",
		Purpose: AADPurposeSensitiveData,
	}
	_, err := cs.DecryptPayload(payload, expected)
	if err == nil || !strings.Contains(err.Error(), "userId") {
		t.Fatalf("expected user mismatch error, got: %v", err)
	}
}

func TestDecryptPayloadInvalidAADJSON(t *testing.T) {
	cs := newTestCryptoService(t)
	ts := time.Now().Unix()

	payload := encryptPayload(t, cs.publicKey, []byte("secret"), []byte("not-json"), ts)

	_, err := cs.DecryptPayload(payload, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid AAD") {
		t.Fatalf("expected invalid AAD error, got: %v", err)
	}
}
