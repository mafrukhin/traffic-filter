package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type FPData struct {
	Webdriver           bool     `json:"webdriver"`
	UA                  string   `json:"ua"`
	Lang                string   `json:"lang"`
	Languages           []string `json:"languages"`
	Platform            string   `json:"platform"`
	Vendor              string   `json:"vendor"`
	HardwareConcurrency int      `json:"hardwareConcurrency"`
	MaxTouchPoints      int      `json:"maxTouchPoints"`
	Plugins             int      `json:"plugins"`
	Screen              string   `json:"screen"`
	ColorDepth          int      `json:"colorDepth"`
	PixelDepth          int      `json:"pixelDepth"`
	TZ                  string   `json:"tz"`
	TimezoneOffset      int      `json:"timezoneOffset"`
	InteractionCount    int      `json:"interactionCount"`
	InteractionTypes    []string `json:"interactionTypes"`
	HadMouse            bool     `json:"hadMouse"`
	HadTouch            bool     `json:"hadTouch"`
	HadKeyboard         bool     `json:"hadKeyboard"`
	HadFocus            bool     `json:"hadFocus"`
	VisibilityState     string   `json:"visibilityState"`
	TimeOnPage          int      `json:"timeOnPage"`
}

// ===== VERIFICATION CACHE =====
var verified = make(map[string]time.Time)

func isVerified(ip string) bool {
	mu.Lock()
	defer mu.Unlock()
	if t, ok := verified[ip]; ok {
		if time.Now().Before(t) {
			return true
		}
		delete(verified, ip)
	}
	return false
}

func markVerified(ip string) {
	mu.Lock()
	defer mu.Unlock()
	verified[ip] = time.Now().Add(10 * time.Minute)
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
var rateLimitWindow = 10 * time.Second
var rateLimitMax = 5

func allow(ip string) bool {
	mu.Lock()
	defer mu.Unlock()

	rate[ip]++
	if rate[ip] == 1 {
		go func() {
			time.Sleep(rateLimitWindow)
			mu.Lock()
			delete(rate, ip)
			mu.Unlock()
		}()
	}

	return rate[ip] <= rateLimitMax
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

func isPlatformUAInconsistent(ua, platform string) bool {
	if strings.Contains(ua, "windows") && strings.Contains(platform, "mac") {
		return true
	}
	if strings.Contains(ua, "macintosh") && strings.Contains(platform, "win") {
		return true
	}
	if strings.Contains(ua, "android") && !strings.Contains(platform, "android") {
		return true
	}
	if strings.Contains(ua, "iphone") && !strings.Contains(platform, "iphone") && !strings.Contains(platform, "ipad") {
		return true
	}
	return false
}

func buildRedirect(target string, action string) string {
	if target == "" {
		target = "https://google.com"
	}

	if strings.Contains(target, "?") {
		return target + "&r=" + fmt.Sprint(time.Now().UnixNano())
	}
	return target + "?r=" + fmt.Sprint(time.Now().UnixNano())
}

func checkHTTPHeaders(r *http.Request) int {
	score := 0

	accept := r.Header.Get("Accept")
	if accept == "" {
		score += 10
	}
	if !strings.Contains(accept, "text/html") && accept != "" {
		score += 15
	}

	acceptLang := r.Header.Get("Accept-Language")
	if acceptLang == "" {
		score += 15
	}

	userAgent := r.Header.Get("User-Agent")
	if userAgent == "" {
		score += 30
	}

	connection := r.Header.Get("Connection")
	if connection == "" || strings.ToLower(connection) == "close" {
		score += 5
	}

	return score
}

func logDecision(ip string, action string, score int, ua string) {
	logEntry := fmt.Sprintf("%s [%s] IP=%s Score=%d UA=%s", time.Now().Format("2006-01-02 15:04:05"), action, ip, score, ua)
	log.Println(logEntry)
}

// ===== HANDLER =====
func getClientIP(r *http.Request) string {
	if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
		return ip
	}
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func fpHandler(w http.ResponseWriter, r *http.Request) {

	ip := getClientIP(r)

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
		FP       FPData `json:"fp"`
		DL       string `json:"dl"`
		BL       string `json:"bl"`
		Verified bool   `json:"verified"`
	}

	json.NewDecoder(r.Body).Decode(&body)

	ua := strings.ToLower(body.FP.UA)
	platform := strings.ToLower(body.FP.Platform)

	response := map[string]interface{}{
		"action": "allow",
		"score":  0,
	}

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

	if len(body.FP.Languages) == 0 {
		score += 20
	}

	if body.FP.HardwareConcurrency == 0 {
		score += 10
	}

	if body.FP.MaxTouchPoints == 0 && strings.Contains(ua, "mobile") {
		score += 15
	}

	if body.FP.Plugins == 0 && strings.Contains(ua, "chrome") {
		score += 10
	}

	if isPlatformUAInconsistent(ua, platform) {
		score += 30
	}

	if body.FP.InteractionCount == 0 {
		score += 40
	}

	if body.FP.InteractionCount == 0 && body.FP.TimeOnPage < 1000 {
		score += 20
	}

	if !body.FP.HadFocus {
		score += 15
	}

	if body.FP.VisibilityState == "hidden" {
		score += 20
	}

	if !body.FP.HadMouse && !body.FP.HadTouch && !strings.Contains(ua, "mobile") {
		score += 20
	}

	updatedScore := score + checkHTTPHeaders(r)
	response["score"] = updatedScore

	mode := os.Getenv("MODE")
	allowLimit := 40
	challengeLimit := 70

	if mode == "light" {
		allowLimit = 25
		challengeLimit = 55
	}
	if mode == "paranoid" {
		allowLimit = 60
		challengeLimit = 85
	}

	if isVerified(ip) {
		response["action"] = "allow"
		response["redirect"] = buildRedirect(body.DL, "allow")
		logDecision(ip, "VERIFIED", updatedScore, ua)
		json.NewEncoder(w).Encode(response)
		return
	}

	if updatedScore <= allowLimit {
		response["action"] = "allow"
		response["redirect"] = buildRedirect(body.DL, "allow")
		markVerified(ip)
		logDecision(ip, "ALLOW", updatedScore, ua)
	} else if updatedScore <= challengeLimit {
		response["action"] = "challenge"
		response["message"] = "Please wait..."
		response["challengeID"] = fmt.Sprintf("%x", time.Now().UnixNano())
		logDecision(ip, "CHALLENGE", updatedScore, ua)
	} else {
		response["action"] = "block"
		response["redirect"] = buildRedirect(body.BL, "block")
		logDecision(ip, "BLOCK", updatedScore, ua)
	}

	json.NewEncoder(w).Encode(response)

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
