package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"log/slog"
	"os"

	"golang.org/x/crypto/argon2"
)

const (
	KeySize   = 32 // AES-256
	NonceSize = 12 // GCM standard
	SaltSize  = 16
)

// EncryptionService provides AES-256-GCM encryption/decryption.
type EncryptionService struct {
	key       []byte
	backupKey []byte
}

// NewEncryptionService creates a new encryption service with the given master password and salt.
// Uses Argon2id for key derivation. Supports an optional backup key for recovery.
func NewEncryptionService(masterPassword string, salt []byte) *EncryptionService {
	key := argon2.IDKey(
		[]byte(masterPassword),
		salt,
		1,       // time
		64*1024, // memory (64 MB)
		4,       // parallelism
		KeySize,
	)

	// Issue #10: Derive backup key if env var is set
	var backupKey []byte
	if backup := os.Getenv("ENCRYPTION_MASTER_KEY_BACKUP"); backup != "" {
		// Derive unique backup salt from primary salt to avoid halving brute-force work factor
		backupSaltHMAC := hmac.New(sha256.New, salt)
		backupSaltHMAC.Write([]byte("backup-key-salt"))
		backupSalt := backupSaltHMAC.Sum(nil)[:SaltSize]

		backupKey = argon2.IDKey(
			[]byte(backup),
			backupSalt,
			1, 64*1024, 4, KeySize,
		)
	}

	return &EncryptionService{key: key, backupKey: backupKey}
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
// Tries the primary key first, then the backup key if available.
func (s *EncryptionService) Decrypt(ciphertext, nonce []byte) (string, error) {
	if len(ciphertext) == 0 || len(nonce) == 0 {
		return "", nil
	}

	// Try primary key
	plaintext, err := s.decryptWithKey(s.key, ciphertext, nonce)
	if err == nil {
		return plaintext, nil
	}

	// Try backup key if primary fails
	if len(s.backupKey) > 0 {
		plaintext, err = s.decryptWithKey(s.backupKey, ciphertext, nonce)
		if err == nil {
			slog.Warn("decryption succeeded with backup key — consider re-encrypting with primary key")
			return plaintext, nil
		}
	}

	return "", errors.New("decryption failed: invalid key or corrupted data")
}

func (s *EncryptionService) decryptWithKey(key, ciphertext, nonce []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
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
