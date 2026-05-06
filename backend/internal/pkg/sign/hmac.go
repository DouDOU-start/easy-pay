package sign

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Merchant request signing scheme:
//   signature = hex( HMAC-SHA256(app_secret, method + "\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + body) )
// Headers required on downstream requests:
//   X-App-Id, X-Timestamp, X-Nonce, X-Signature
// Replay protection: reject if |now - timestamp| > 5 minutes.

const MaxClockSkew = 5 * time.Minute

var (
	ErrExpiredTimestamp = errors.New("sign: timestamp expired")
	ErrBadSignature     = errors.New("sign: bad signature")
)

func Compute(appSecret, method, path, timestamp, nonce string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write([]byte(strings.ToUpper(method)))
	mac.Write([]byte("\n"))
	mac.Write([]byte(path))
	mac.Write([]byte("\n"))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("\n"))
	mac.Write([]byte(nonce))
	mac.Write([]byte("\n"))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func Verify(appSecret, method, path, timestamp, nonce, signature string, body []byte) error {
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return ErrBadSignature
	}
	if d := time.Since(time.Unix(ts, 0)); d > MaxClockSkew || d < -MaxClockSkew {
		return ErrExpiredTimestamp
	}
	want := Compute(appSecret, method, path, timestamp, nonce, body)
	if !hmac.Equal([]byte(want), []byte(signature)) {
		return ErrBadSignature
	}
	return nil
}

// SortedQuery builds a canonical form "k1=v1&k2=v2" sorted by key.
// Useful when signing query-string callbacks (e.g. alipay sync return).
func SortedQuery(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(params[k])
	}
	return b.String()
}
