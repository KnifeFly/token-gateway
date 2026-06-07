package configadmin

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"io"

	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// CredentialCodec encrypts provider credentials before persistence.
type CredentialCodec struct {
	key []byte
}

// NewCredentialCodec returns an AES-GCM credential codec.
func NewCredentialCodec(secret string) *CredentialCodec {
	sum := sha256.Sum256([]byte(secret))
	return &CredentialCodec{key: sum[:]}
}

// Encrypt encrypts plaintext credential material.
func (c *CredentialCodec) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
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
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return "gcm:" + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts credential material when a privileged component needs it.
func (c *CredentialCodec) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	if len(ciphertext) < 4 || ciphertext[:4] != "gcm:" {
		return "", apperr.InvalidArgument("credential ciphertext is invalid")
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext[4:])
	if err != nil {
		return "", apperr.InvalidArgument("credential ciphertext is invalid", apperr.WithCause(err))
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", apperr.InvalidArgument("credential ciphertext is invalid")
	}
	nonce := raw[:gcm.NonceSize()]
	payload := raw[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, payload, nil)
	if err != nil {
		return "", apperr.InvalidArgument("credential ciphertext is invalid", apperr.WithCause(err))
	}
	return string(plaintext), nil
}
