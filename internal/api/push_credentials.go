package api

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
)

// pushCredentialCipher keeps SSH private keys out of plaintext SQLite files.
// Production instances must set EXPRESS233_PUSH_CREDENTIAL_KEY to a base64
// encoded 32-byte key. Refusing to save a key without it is intentional.
type pushCredentialCipher struct{ key []byte }

func newPushCredentialCipher() (*pushCredentialCipher, error) {
	raw := os.Getenv("EXPRESS233_PUSH_CREDENTIAL_KEY")
	if raw == "" {
		return nil, fmt.Errorf("EXPRESS233_PUSH_CREDENTIAL_KEY must be configured before adding SSH private keys")
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("EXPRESS233_PUSH_CREDENTIAL_KEY must be base64 encoded 32-byte key")
	}
	return &pushCredentialCipher{key: key}, nil
}

func (c *pushCredentialCipher) encrypt(plain string) (string, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(append(nonce, gcm.Seal(nil, nonce, []byte(plain), nil)...)), nil
}

func (c *pushCredentialCipher) decrypt(encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", fmt.Errorf("invalid encrypted credential")
	}
	plain, err := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
