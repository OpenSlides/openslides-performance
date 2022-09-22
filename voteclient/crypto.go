package voteclient

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

func encryptVote(vote string, mainKey, pollKey, keySig []byte) (string, error) {
	// Check that the poll Key was signed with the main key.
	if !verify(mainKey, pollKey, keySig) {
		return "", fmt.Errorf("poll key is invalid. It was not signed with the main key")
	}

	encrypted, err := encrypt(rand.Reader, pollKey, []byte(vote))
	if err != nil {
		return "", fmt.Errorf("encrypt vote: %w", err)
	}

	return string(encrypted), nil

}

const (
	pubKeySize = 32
	nonceSize  = 12
)

func createVoteToken() string {
	token := make([]byte, 8)
	rand.Reader.Read(token)
	return base64.StdEncoding.EncodeToString(token)
}

func verify(pubKey, message, signature []byte) bool {
	return ed25519.Verify(pubKey, message, signature)
}

func encrypt(random io.Reader, publicKey []byte, plaintext []byte) ([]byte, error) {
	cipherPrefix := make([]byte, pubKeySize+nonceSize)

	ephemeralPrivateKey := make([]byte, curve25519.ScalarSize)
	if _, err := io.ReadFull(random, ephemeralPrivateKey); err != nil {
		return nil, fmt.Errorf("reading from random source: %w", err)
	}

	ephemeralPublicKey, err := curve25519.X25519(ephemeralPrivateKey, curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("creating ephemeral public key: %w", err)
	}
	copy(cipherPrefix[:pubKeySize], ephemeralPublicKey)

	sharedSecred, err := curve25519.X25519(ephemeralPrivateKey, publicKey)
	if err != nil {
		return nil, fmt.Errorf("creating shared secred: %w", err)
	}

	hkdf := hkdf.New(sha256.New, sharedSecred, nil, nil)
	key := make([]byte, 16)
	if _, err := io.ReadFull(hkdf, key); err != nil {
		return nil, fmt.Errorf("generate key with hkdf: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating aes chipher: %w", err)
	}

	nonce := cipherPrefix[pubKeySize:]
	if _, err := random.Read(nonce); err != nil {
		return nil, fmt.Errorf("read random for nonce: %w", err)
	}

	mode, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm mode: %w", err)
	}

	encrypted := mode.Seal(nil, nonce, plaintext, nil)

	return append(cipherPrefix, encrypted...), nil
}
