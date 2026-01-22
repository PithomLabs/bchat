package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"

	"golang.org/x/crypto/argon2"
)

const (
	KeySize   = 32 // AES-256
	NonceSize = 12 // GCM standard
	SaltSize  = 16
)

// EncryptionService provides AES-256-GCM encryption/decryption.
type EncryptionService struct {
	key []byte
}

// NewEncryptionService creates a new encryption service with the given master password and salt.
// Uses Argon2id for key derivation.
func NewEncryptionService(masterPassword string, salt []byte) *EncryptionService {
	key := argon2.IDKey(
		[]byte(masterPassword),
		salt,
		1,       // time
		64*1024, // memory (64 MB)
		4,       // parallelism
		KeySize,
	)
	return &EncryptionService{key: key}
}

// Encrypt encrypts plaintext using AES-256-GCM.
// Returns the ciphertext and nonce.
func (s *EncryptionService) Encrypt(plaintext string) (ciphertext, nonce []byte, err error) {
	if plaintext == "" {
		return nil, nil, nil
	}

	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	nonce = make([]byte, NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}

	ciphertext = gcm.Seal(nil, nonce, []byte(plaintext), nil)
	return ciphertext, nonce, nil
}

// Decrypt decrypts ciphertext using AES-256-GCM.
func (s *EncryptionService) Decrypt(ciphertext, nonce []byte) (string, error) {
	if len(ciphertext) == 0 || len(nonce) == 0 {
		return "", nil
	}

	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", errors.New("decryption failed: invalid key or corrupted data")
	}

	return string(plaintext), nil
}

// GenerateSalt generates a random salt for key derivation.
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, SaltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	return salt, nil
}
