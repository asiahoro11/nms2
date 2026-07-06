// Made by YTSworks
// YTS工作室製作
package camera

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

type cryptoHelper struct {
	key []byte
}

func newCrypto(secret string) *cryptoHelper {
	hash := sha256.Sum256([]byte(secret))
	return &cryptoHelper{key: hash[:]}
}

func (c *cryptoHelper) encrypt(plaintext string) (string, error) {
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
	ct := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

func (c *cryptoHelper) decrypt(encrypted string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encrypted)
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
	ns := gcm.NonceSize()
	if len(data) < ns {
		return "", errors.New("ciphertext too short")
	}
	pt, err := gcm.Open(nil, data[:ns], data[ns:], nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

func EncryptPassword(secret, plaintext string) (string, error) {
	return newCrypto(secret).encrypt(plaintext)
}

func ResolvePassword(secret, passwordEncrypted, rawRTSPUrl string) string {
	if passwordEncrypted != "" {
		if plain, err := newCrypto(secret).decrypt(passwordEncrypted); err == nil && plain != "" {
			return plain
		}
	}
	if strings.Contains(rawRTSPUrl, "@") {
		schemeEnd := strings.Index(rawRTSPUrl, "://")
		if schemeEnd != -1 {
			creds := rawRTSPUrl[schemeEnd+3:]
			if atIdx := strings.Index(creds, "@"); atIdx != -1 {
				userpass := creds[:atIdx]
				if colonIdx := strings.Index(userpass, ":"); colonIdx != -1 {
					return userpass[colonIdx+1:]
				}
			}
		}
	}
	return ""
}
