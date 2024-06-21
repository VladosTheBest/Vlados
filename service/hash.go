package service

import (
	"crypto/sha1"
	"fmt"
)

// MakeHash - returns a string who represents a hash generated based of s parameter
func MakeHash(s string) string {
	h := sha1.New()
	_, _ = h.Write([]byte(s))
	bs := h.Sum(nil)
	return fmt.Sprintf("%x", bs)
}
