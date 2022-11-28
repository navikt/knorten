package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"io"
)

type EncrypterDecrypter struct {
	key []byte
}

func New(key string) *EncrypterDecrypter {
	return &EncrypterDecrypter{
		key: []byte(key),
	}
}

func (ed *EncrypterDecrypter) EncryptValue(value string) (string, error) {
	aesBlock, err := aes.NewCipher(ed.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(aesBlock)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	_, err = io.ReadFull(rand.Reader, nonce)
	if err != nil {
		return "", err
	}

	encrypted := gcm.Seal(nonce, nonce, []byte(value), nil)
	return hex.EncodeToString(encrypted), nil
}

func (ed *EncrypterDecrypter) DecryptValue(encValue string) (string, error) {
	encBytes, err := hex.DecodeString(encValue)
	if err != nil {
		return "", err
	}

	aesBlock, err := aes.NewCipher(ed.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(aesBlock)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	nonce, cipheredText := encBytes[:nonceSize], encBytes[nonceSize:]

	value, err := gcm.Open(nil, nonce, cipheredText, nil)
	if err != nil {
		return "", err
	}
	return string(value), nil
}
