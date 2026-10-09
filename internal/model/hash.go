package model

import (
	"crypto/sha256"
	"encoding/hex"
)

func Hex(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
