package utils

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// GenerateRandomString generates a random string of specified length
func GenerateRandomString(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b)[:length], nil
}

// FormatDuration formats a duration in a human-readable format
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.2fm", d.Minutes())
	}
	return fmt.Sprintf("%.2fh", d.Hours())
}

// PrettyJSON returns a pretty-printed JSON string
func PrettyJSON(v interface{}) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ContainsString checks if a string slice contains a specific string
func ContainsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// RemoveString removes a string from a string slice
func RemoveString(slice []string, str string) []string {
	for i, s := range slice {
		if s == str {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

// TruncateString truncates a string to the specified length
func TruncateString(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length] + "..."
}

// IsValidEmail performs basic email validation
func IsValidEmail(email string) bool {
	// Basic email validation regex
	// This is a simple validation and might need to be enhanced based on requirements
	at := false
	dot := false
	for i := 0; i < len(email); i++ {
		if email[i] == '@' {
			at = true
		}
		if at && email[i] == '.' {
			dot = true
		}
	}
	return at && dot
}
