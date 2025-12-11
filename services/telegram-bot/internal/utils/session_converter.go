package utils

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// ConvertToTData converts session string and cookies to TData format
func ConvertToTData(sessionString string, cookies []byte, phone string) ([]byte, error) {
	// Create a ZIP archive
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	// Create TData structure
	folderName := fmt.Sprintf("telegram_%s/tdata/", phone)

	// Create key_datas file (simplified)
	keyDatasPath := folderName + "key_datas"
	f, err := w.Create(keyDatasPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create key_datas: %w", err)
	}

	// Write simplified key data
	keyData := []byte{0x01, 0x02, 0x03, 0x04} // Placeholder
	f.Write(keyData)

	// Create session file
	sessionPath := folderName + "D877F783D5D3EF8C"
	f, err = w.Create(sessionPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create session file: %w", err)
	}

	// Decode session string if it's base64
	sessionData, err := base64.StdEncoding.DecodeString(sessionString)
	if err != nil {
		// If not base64, use as is
		sessionData = []byte(sessionString)
	}
	f.Write(sessionData)

	// Create maps file
	mapsPath := folderName + "maps"
	f, err = w.Create(mapsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create maps: %w", err)
	}
	f.Write([]byte("0"))

	// Create settings file
	settingsPath := folderName + "settings"
	f, err = w.Create(settingsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create settings: %w", err)
	}

	settings := map[string]interface{}{
		"version": 1,
		"cookies": string(cookies),
	}
	settingsJSON, _ := json.Marshal(settings)
	f.Write(settingsJSON)

	w.Close()
	return buf.Bytes(), nil
}

// ConvertToTelethonSession converts session data to Telethon .session format
func ConvertToTelethonSession(sessionString string, phone string, dcID int, serverAddress string, port int) ([]byte, error) {
	// This is a simplified version
	// Real implementation would create a proper SQLite database

	sessionData := map[string]interface{}{
		"format":         "telethon",
		"phone":          phone,
		"dc_id":          dcID,
		"server_address": serverAddress,
		"port":           port,
		"auth_key":       sessionString,
	}

	// For now, return JSON representation
	// Real implementation would create SQLite file
	data, err := json.MarshalIndent(sessionData, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal session: %w", err)
	}

	return data, nil
}

// ConvertToPyrogramSession converts session data to Pyrogram .session format
func ConvertToPyrogramSession(sessionString string, phone string, dcID int) ([]byte, error) {
	// This is a simplified version
	// Real implementation would create a proper SQLite database

	sessionData := map[string]interface{}{
		"format":    "pyrogram",
		"phone":     phone,
		"dc_id":     dcID,
		"auth_key":  sessionString,
		"test_mode": false,
	}

	// For now, return JSON representation
	// Real implementation would create SQLite file
	data, err := json.MarshalIndent(sessionData, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal session: %w", err)
	}

	return data, nil
}

// ValidateSessionString checks if session string is valid
func ValidateSessionString(sessionString string) bool {
	if sessionString == "" {
		return false
	}

	// Try to decode as base64
	_, err := base64.StdEncoding.DecodeString(sessionString)
	if err == nil {
		return true
	}

	// Check if it looks like a valid session string
	// This is a basic check
	return len(sessionString) >= 100
}

// ExtractAuthKey extracts auth key from session string
func ExtractAuthKey(sessionString string) ([]byte, error) {
	// Try to decode as base64
	authKey, err := base64.StdEncoding.DecodeString(sessionString)
	if err != nil {
		// If not base64, use first 256 bytes as auth key
		if len(sessionString) >= 256 {
			return []byte(sessionString[:256]), nil
		}
		return []byte(sessionString), nil
	}

	// Ensure auth key is 256 bytes
	if len(authKey) < 256 {
		// Pad with zeros
		padded := make([]byte, 256)
		copy(padded, authKey)
		return padded, nil
	}

	return authKey[:256], nil
}
