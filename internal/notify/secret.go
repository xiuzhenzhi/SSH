package notify

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
)

// Bind ciphertext to its channel ID so database rows cannot swap credentials.
func Seal(key, id string, plaintext []byte) (string, error) {
	a, err := box(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, a.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(a.Seal(nonce, nonce, plaintext, []byte(id))), nil
}
func Open(key, id, encoded string) ([]byte, error) {
	a, err := box(key)
	if err != nil {
		return nil, err
	}
	b, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(b) < a.NonceSize() {
		return nil, errors.New("invalid encrypted channel configuration")
	}
	return a.Open(nil, b[:a.NonceSize()], b[a.NonceSize():], []byte(id))
}
func box(key string) (cipher.AEAD, error) {
	b, err := hex.DecodeString(key)
	if err != nil || len(b) != 32 {
		return nil, errors.New("ENCRYPTION_KEY must be 64 hex characters")
	}
	c, err := aes.NewCipher(b)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(c)
}
