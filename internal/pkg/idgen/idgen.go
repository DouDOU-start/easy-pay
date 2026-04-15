package idgen

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// OrderNo returns an order number of the form <prefix><yyyymmddHHMMSS><hex>,
// clamped to 32 characters total. 32 is the WeChat Pay out_trade_no hard limit
// and also fits our VARCHAR(40) index comfortably. With a 2-char prefix the
// output is exactly 32 chars (2 + 14 + 16). Longer prefixes truncate the tail
// randomness, but timestamp + leading entropy still makes collisions
// astronomically unlikely.
func OrderNo(prefix string) string {
	const maxLen = 32
	b := make([]byte, 8) // 16 hex chars
	_, _ = rand.Read(b)
	s := fmt.Sprintf("%s%s%s", prefix, time.Now().Format("20060102150405"), hex.EncodeToString(b))
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	return s
}

func Nonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
