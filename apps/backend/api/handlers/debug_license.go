package handlers

import (
	"encoding/base64"
	"management-server/services/license"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type DebugLicenseInput struct {
	LicenseKey string `json:"license_key"`
}

// DebugLicense 測試並�??��?權�???
func (h *Handler) DebugLicense(c *gin.Context) {
	var input DebugLicenseInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	key := strings.TrimSpace(input.LicenseKey)
	key = strings.ReplaceAll(key, "\n", "")
	key = strings.ReplaceAll(key, "\r", "")
	key = strings.ReplaceAll(key, " ", "")

	machineID := getSystemUUID() // uses ToLower as per admin.go update

	passphrase := "NMS-LICENSE-" + machineID
	formalSecretKey := h.getFormalSecretKey()
	pocSecretKey := h.getPOCSecretKey()

	// Decode Base64
	ciphertext, err := base64.StdEncoding.DecodeString(key)
	decodeErr := ""
	if err != nil {
		decodeErr = err.Error()
	}

	// Try Decrypt
	payload, decryptErr := license.ValidateLicenseKey(key, machineID, formalSecretKey, pocSecretKey)

	decryptErrStr := ""
	if decryptErr != nil {
		decryptErrStr = decryptErr.Error()
	}

	success := (decryptErr == nil)

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"analysis": map[string]interface{}{
				"input_key_length":      len(key),
				"machine_id_used":       machineID,
				"passphrase_derived":    passphrase,
				"poc_key_path_enabled":  true,
				"base64_decode_success": (decodeErr == ""),
				"base64_decode_error":   decodeErr,
				"ciphertext_bytes_len":  len(ciphertext),
				"decryption_success":    success,
				"decryption_error":      decryptErrStr,
			},
			"payload": payload,
		},
		Message: "Debug analysis complete",
	})
}
