package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashCard returns a SHA256 hash of the full card number for consistent lookup.
func HashCard(cardNumber string) string {
	h := sha256.New()
	h.Write([]byte(cardNumber))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// Last4 returns the last 4 digits of a card number.
func Last4(cardNumber string) string {
	if len(cardNumber) <= 4 {
		return cardNumber
	}
	return cardNumber[len(cardNumber)-4:]
}
