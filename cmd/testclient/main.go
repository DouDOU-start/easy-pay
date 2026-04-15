// testclient drives a full end-to-end wechat Native prepay test against a
// locally running easy-pay API. It reads real merchant credentials from a
// JSON file so nothing sensitive ever appears on argv or in shell history.
//
// Usage:
//	go run ./cmd/testclient -creds ./wechat_creds.json
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/easypay/easy-pay/internal/pkg/idgen"
	"github.com/easypay/easy-pay/internal/pkg/sign"
)

type creds struct {
	BaseURL    string `json:"base_url"`
	AdminUser  string `json:"admin_user"`
	AdminPass  string `json:"admin_pass"`
	MerchantID int64  `json:"merchant_id"`
	AppID      string `json:"app_id"`
	AppSecret  string `json:"app_secret"`
	NotifyURL  string `json:"notify_url"`
	Wechat     struct {
		MchID         string `json:"mch_id"`
		AppID         string `json:"app_id"`
		APIV3Key      string `json:"api_v3_key"`
		SerialNo      string `json:"serial_no"`
		PrivateKeyPEM string `json:"private_key_pem"`
	} `json:"wechat"`
}

func main() {
	credsPath := flag.String("creds", "wechat_creds.json", "path to credentials json")
	amount := flag.Int64("amount", 1, "amount in cents (1 = ¥0.01)")
	subject := flag.String("subject", "easy-pay 测试订单", "order subject")
	skipChannel := flag.Bool("skip-channel", false, "don't (re)upsert the wechat channel config, assume already set")
	flag.Parse()

	raw, err := os.ReadFile(*credsPath)
	if err != nil {
		log.Fatalf("read creds: %v", err)
	}
	var c creds
	if err := json.Unmarshal(raw, &c); err != nil {
		log.Fatalf("parse creds: %v", err)
	}
	if c.BaseURL == "" {
		c.BaseURL = "http://localhost:8080"
	}
	if c.AdminUser == "" {
		c.AdminUser = "admin"
	}
	if c.AdminPass == "" {
		c.AdminPass = "admin123"
	}

	// --- 1. admin login ---
	log.Printf("[1/4] admin login as %q", c.AdminUser)
	loginBody, _ := json.Marshal(map[string]string{
		"username": c.AdminUser,
		"password": c.AdminPass,
	})
	var loginResp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := doJSON(http.MethodPost, c.BaseURL+"/admin/login", "", loginBody, &loginResp); err != nil {
		log.Fatalf("    login failed: %v", err)
	}
	if loginResp.Data.Token == "" {
		log.Fatalf("    login failed: %s", loginResp.Msg)
	}
	token := "Bearer " + loginResp.Data.Token
	log.Printf("      → token ok")

	// --- 2. upsert wechat channel config (encrypted AES-256-GCM in DB) ---
	if !*skipChannel {
		log.Printf("[2/4] upsert wechat channel for merchant %d", c.MerchantID)
		upsertBody, _ := json.Marshal(map[string]any{
			"channel": "wechat",
			"config":  c.Wechat,
		})
		var upsertResp map[string]any
		url := fmt.Sprintf("%s/admin/merchants/%d/channels", c.BaseURL, c.MerchantID)
		if err := doJSON(http.MethodPut, url, token, upsertBody, &upsertResp); err != nil {
			log.Fatalf("    upsert failed: %v", err)
		}
		log.Printf("      → channel saved")
	} else {
		log.Printf("[2/4] skipped channel upsert")
	}

	// --- 3. sign a merchant request and create a native order ---
	orderNo := fmt.Sprintf("TEST%d", time.Now().Unix())
	log.Printf("[3/4] create native order merchant_order_no=%s amount=%d", orderNo, *amount)
	payload := map[string]any{
		"merchant_order_no": orderNo,
		"channel":           "wechat",
		"trade_type":        "native",
		"subject":           *subject,
		"amount":            *amount,
		"expire_seconds":    900,
	}
	body, _ := json.Marshal(payload)

	ts := fmt.Sprintf("%d", time.Now().Unix())
	nonce := idgen.Nonce()
	path := "/api/v1/pay/create"
	signature := sign.Compute(c.AppSecret, http.MethodPost, path, ts, nonce, body)

	req, _ := http.NewRequest(http.MethodPost, c.BaseURL+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-App-Id", c.AppID)
	req.Header.Set("X-Timestamp", ts)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Signature", signature)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("    create request failed: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("      → HTTP %d", resp.StatusCode)
	log.Printf("      → %s", respBody)

	if resp.StatusCode != http.StatusOK {
		os.Exit(1)
	}

	// --- 4. pretty-print code_url ---
	var parsed struct {
		Code string `json:"code"`
		Data struct {
			OrderNo string `json:"order_no"`
			CodeURL string `json:"code_url"`
			H5URL   string `json:"h5_url"`
		} `json:"data"`
	}
	_ = json.Unmarshal(respBody, &parsed)
	log.Printf("[4/4] ✓ DONE")
	fmt.Println()
	fmt.Println("  order_no:", parsed.Data.OrderNo)
	fmt.Println("  code_url:", parsed.Data.CodeURL)
	if parsed.Data.CodeURL != "" {
		fmt.Println()
		fmt.Println("  → 用微信扫上面的 code_url 即可支付（金额:", float64(*amount)/100, "CNY）")
	}
}

func doJSON(method, url, auth string, body []byte, out any) error {
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("http %d: %s", resp.StatusCode, raw)
	}
	return json.Unmarshal(raw, out)
}
