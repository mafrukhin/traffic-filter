package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

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

// VPN check via API
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

func check(w http.ResponseWriter, r *http.Request) {

	ip := r.FormValue("ip")
	ua := strings.ToLower(r.FormValue("ua"))

	score := 0

	// UA bot
	if isBotUA(ua) {
		score += 40
	}

	// crawler
	if isCrawler(ua) {
		score += 50
	}

	// fake mobile
	if isFakeMobile(ua) {
		score += 40
	}

	// VPN
	if checkVPN(ip) {
		score += 30
	}

	// empty ip
	if ip == "" {
		score += 50
	}

	if score >= 60 {
		fmt.Fprintf(w, "block")
		return
	}

	fmt.Fprintf(w, "allow")
}

func main() {
	http.HandleFunc("/check", check)
	http.ListenAndServe(":8080", nil)
}
