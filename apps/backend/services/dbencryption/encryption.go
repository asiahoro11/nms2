// Made by YTSworks
// YTS工作室製作
package dbencryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/crypto/pbkdf2"
)

// Service handles database and backup encryption
type Service struct {
	encryptionKey []byte
}

// NewService creates a new encryption service
func NewService(password string) (*Service, error) {
	if password == "" {
		return nil, fmt.Errorf("encryption password cannot be empty")
	}

	// Derive 256-bit key from password using PBKDF2
	key := pbkdf2.Key([]byte(password), []byte("management-server-salt-v1"), 100000, 32, sha256.New)

	return &Service{
		encryptionKey: key,
	}, nil
}

// EncryptBackup encrypts a database backup file
func (s *Service) EncryptBackup(sourcePath, destPath string) error {
	// Read source file
	plaintext, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read source file: %v", err)
	}

	// Create AES cipher
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %v", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %v", err)
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("failed to generate nonce: %v", err)
	}

	// Encrypt
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// Write to destination
	if err := os.WriteFile(destPath, ciphertext, 0600); err != nil {
		return fmt.Errorf("failed to write encrypted file: %v", err)
	}

	return nil
}

// DecryptBackup decrypts an encrypted backup file
func (s *Service) DecryptBackup(sourcePath, destPath string) error {
	// Read encrypted file
	ciphertext, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read encrypted file: %v", err)
	}

	// Create AES cipher
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %v", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %v", err)
	}

	// Extract nonce
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return fmt.Errorf("decryption failed (wrong password?): %v", err)
	}

	// Write to destination
	if err := os.WriteFile(destPath, plaintext, 0600); err != nil {
		return fmt.Errorf("failed to write decrypted file: %v", err)
	}

	return nil
}

// EncryptDatabase encrypts an existing SQLite database using SQLCipher commands
// This converts a plain database to an encrypted one
func (s *Service) EncryptDatabase(dbPath string, password string) error {
	// Open the plain database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}
	defer db.Close()

	// Create a temporary encrypted database
	tempPath := dbPath + ".encrypted.tmp"
	defer os.Remove(tempPath)

	// Attach encrypted database
	attachSQL := fmt.Sprintf("ATTACH DATABASE '%s' AS encrypted KEY '%s'", tempPath, password)
	if _, err := db.Exec(attachSQL); err != nil {
		return fmt.Errorf("failed to attach encrypted database: %v", err)
	}

	// Export schema and data
	if _, err := db.Exec("SELECT sqlcipher_export('encrypted')"); err != nil {
		return fmt.Errorf("failed to export to encrypted database: %v", err)
	}

	// Detach
	if _, err := db.Exec("DETACH DATABASE encrypted"); err != nil {
		return fmt.Errorf("failed to detach database: %v", err)
	}

	db.Close()

	// Replace original with encrypted
	backupPath := dbPath + ".plain.backup"
	if err := os.Rename(dbPath, backupPath); err != nil {
		return fmt.Errorf("failed to backup original database: %v", err)
	}

	if err := os.Rename(tempPath, dbPath); err != nil {
		// Restore backup on failure
		os.Rename(backupPath, dbPath)
		return fmt.Errorf("failed to replace database: %v", err)
	}

	return nil
}

// CreateEncryptedBackup creates an encrypted backup with metadata
func (s *Service) CreateEncryptedBackup(dbPath, outputPath string) error {
	// Create temp directory
	tempDir := filepath.Join(os.TempDir(), "nms-backup-temp")
	os.MkdirAll(tempDir, 0700)
	defer os.RemoveAll(tempDir)

	// Copy database to temp
	tempDB := filepath.Join(tempDir, "nms.db")
	dbData, err := os.ReadFile(dbPath)
	if err != nil {
		return fmt.Errorf("failed to read database: %v", err)
	}

	if err := os.WriteFile(tempDB, dbData, 0600); err != nil {
		return fmt.Errorf("failed to write temp database: %v", err)
	}

	// Encrypt the backup
	if err := s.EncryptBackup(tempDB, outputPath); err != nil {
		return fmt.Errorf("failed to encrypt backup: %v", err)
	}

	return nil
}

// ValidatePassword checks if a password can decrypt a backup
func (s *Service) ValidatePassword(encryptedPath string) error {
	// Try to decrypt a small portion
	data, err := os.ReadFile(encryptedPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}

	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return fmt.Errorf("invalid key: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %v", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return fmt.Errorf("invalid encrypted file")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	_, err = gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return fmt.Errorf("wrong password")
	}

	return nil
}
