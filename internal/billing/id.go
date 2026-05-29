package billing

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

func newID(prefix string) string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return prefix + "_" + time.Now().UTC().Format("20060102150405000000000")
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}
