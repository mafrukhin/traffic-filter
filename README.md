# Traffic Filter - PDR (Product Design Record)

## Ringkasan Sistem

Traffic Filter adalah sistem deteksi bot dan human verification berbasis fingerprinting JavaScript dan scoring behavior untuk aplikasi affiliate traffic. Sistem ini dirancang untuk mengurangi bot traffic dan fake user tanpa memerlukan CAPTCHA external seperti Cloudflare, meskipun dapat diintegrasikan sebagai layer tambahan.

**Status**: Production Ready (Fase 1-5 Complete)

---

## Arsitektur Sistem

### Komponen Utama

```
┌─────────────────────────────────────┐
│         User Browser                │
│  (fp.html - JavaScript Fingerprint) │
└────────────┬────────────────────────┘
             │ POST /fp.html
             │ (redirect → /fp)
             ▼
┌─────────────────────────────────────┐
│      Nginx Gateway (Lua)            │
│   - Route all traffic → /fp.html    │
│   - Forward /fp → botengine:8080    │
└────────────┬────────────────────────┘
             │ POST /fp (JSON)
             ▼
┌─────────────────────────────────────┐
│   Botengine (Go Service)            │
│  - Fingerprint Analysis             │
│  - Behavior Detection               │
│  - HTTP Header Validation           │
│  - Scoring & Decision Logic         │
│  - Verification Caching             │
└────────────┬────────────────────────┘
             │ JSON Response
             ▼
┌─────────────────────────────────────┐
│      Frontend (fp.html)             │
│  - Handle response actions          │
│  - Show challenge UI if needed      │
│  - Auto-redirect or show verification
└─────────────────────────────────────┘
```

---

## Fase Implementasi

### Fase 1: IP Client Extraction ✅
**Tujuan**: Memastikan IP klien diidentifikasi dengan benar di backend.

**Perubahan**:
- Extract real IP dari `X-Real-IP` dan `X-Forwarded-For` header
- Fallback ke `r.RemoteAddr` jika header tidak ada
- Normalisasi IP parsing menggunakan `net.SplitHostPort`

**Benefit**:
- Rate limit berdasarkan IP asli, bukan proxy
- Cache verification per IP yang akurat

---

### Fase 2: Extended Fingerprinting ✅
**Tujuan**: Kumpulkan data browser lebih lengkap untuk deteksi bot yang lebih akurat.

**Data yang dikumpulkan**:
```javascript
{
  ua: navigator.userAgent,
  webdriver: navigator.webdriver,
  lang: navigator.language,
  languages: navigator.languages,
  platform: navigator.platform,
  vendor: navigator.vendor,
  hardwareConcurrency: navigator.hardwareConcurrency,
  maxTouchPoints: navigator.maxTouchPoints,
  plugins: navigator.plugins.length,
  screen: "width x height",
  colorDepth: screen.colorDepth,
  pixelDepth: screen.pixelDepth,
  tz: timezone string,
  timezoneOffset: new Date().getTimezoneOffset()
}
```

**Deteksi**:
- `navigator.webdriver` → +60 poin (automated browser)
- Empty timezone → +30 poin
- Bot UA pattern (bot, crawler, spider, curl, wget, python) → +40 poin
- Known crawler (googlebot, bingbot, yandex) → +50 poin
- Fake mobile (iPhone + Windows) → +40 poin
- Missing languages → +20 poin
- Zero hardware concurrency → +10 poin
- Zero touch points pada mobile → +15 poin
- Zero plugins pada Chrome → +10 poin
- UA vs Platform inconsistent → +30 poin

---

### Fase 3: Behavior Detection ✅
**Tujuan**: Deteksi apakah pengguna adalah manusia berdasarkan interaksi.

**Event yang dikumpulkan**:
- `mousemove`, `mousedown` → `hadMouse`
- `touchstart` → `hadTouch`
- `keydown` → `hadKeyboard`
- `scroll`, `focus`, `blur`, `visibilitychange`
- `timeOnPage` → durasi dari page load hingga fingerprint dikirim
- `interactionCount` → jumlah interaksi
- `interactionTypes` → array tipe interaksi yang terjadi
- `hadFocus` → apakah tab pernah fokus
- `visibilityState` → "visible" atau "hidden"

**Scoring Behavior**:
- Zero interaction → +40 poin
- Zero interaction + time < 1s → +20 poin
- No focus → +15 poin
- Visibility = hidden → +20 poin
- Desktop tanpa mouse/touch → +20 poin

**Delay**:
- Minimal 1200ms sebelum submit fingerprint
- Memberi waktu browser nyata untuk interaksi

---

### Fase 4: Tiered Challenge Flow ✅
**Tujuan**: Implementasi sistem 3-tier (allow/challenge/block) dengan verification flow.

**Decision Logic**:
```
MODE=normal (default):
  Score ≤ 40       → ALLOW (mark verified, cache 10 min)
  Score 41-70      → CHALLENGE (show spinner, wait 2s, reverify)
  Score > 70       → BLOCK (redirect ke blacklink)

MODE=light (permissive):
  Score ≤ 25       → ALLOW
  Score 26-55      → CHALLENGE
  Score > 55       → BLOCK

MODE=paranoid (strict):
  Score ≤ 60       → ALLOW
  Score 61-85      → CHALLENGE
  Score > 85       → BLOCK
```

**Challenge Flow**:
1. User kena score medium → response action: "challenge"
2. Frontend tampil "Verifying..." UI dengan spinner
3. Tunggu 2 detik (atau custom duration)
4. Auto-submit ulang dengan `verified: true`
5. Backend re-verify: jika `verified=true` dan `isVerified(ip)`, langsung ALLOW
6. Jika tidak approved, trigger BLOCK

**Verification Cache**:
- `isVerified(ip)` → cek apakah IP sudah diverifikasi
- `markVerified(ip)` → tandai IP sebagai verified selama 10 menit
- Cache reset otomatis setelah TTL

---

### Fase 5: HTTP Headers Validation & Monitoring ✅
**Tujuan**: Tambahan deteksi dari HTTP request headers dan logging untuk monitoring.

**HTTP Header Scoring**:
- Missing `Accept` → +10 poin
- `Accept` bukan text/html → +15 poin
- Missing `Accept-Language` → +15 poin
- Missing `User-Agent` → +30 poin
- Missing/close `Connection` → +5 poin

**Decision Logging**:
```
Format: [TIMESTAMP] [ACTION] IP=... Score=... UA=...

Contoh output:
2026-05-10 09:35:42 [ALLOW] IP=192.168.1.1 Score=15 UA=Mozilla/5.0...
2026-05-10 09:35:43 [CHALLENGE] IP=192.168.1.2 Score=55 UA=bot-like...
2026-05-10 09:35:44 [BLOCK] IP=192.168.1.3 Score=85 UA=curl/7.0...
2026-05-10 09:35:45 [VERIFIED] IP=192.168.1.1 Score=18 UA=Mozilla/5.0...
```

**Log Actions**:
- `ALLOW`: Traffic diizinkan langsung
- `CHALLENGE`: Challenge flow ditampilkan
- `BLOCK`: Traffic diblok, redirect ke blacklink
- `VERIFIED`: IP sudah diverifikasi sebelumnya, cache hit

---

## Struktur Project

```
traffic-filter/
├── go.mod                    # Go module definition
├── docker-compose.yml        # Container orchestration
├── install.sh              # Installation script
├── README.md               # Dokumentasi ini
│
├── botengine/              # Backend service (Go)
│   ├── main.go            # Main logic (327 lines)
│   ├── Dockerfile         # Container image
│   └── botengine          # Compiled binary
│
├── gateway/               # Nginx + Lua
│   ├── nginx.conf         # Nginx configuration
│   └── filter.lua         # Lua routing logic
│
└── js/                    # Frontend
    └── fp.html           # Fingerprint collector & UI
```

---

## Cara Run di Server

### Prerequisite

- **Docker** (versi 20+)
- **Docker Compose** (versi 1.29+)
- **Go** 1.26+ (untuk development)
- **Nginx** (optional, jika tidak pakai container)

### Opsi 1: Docker Compose (Recommended)

#### 1. Clone / Setup Project

```bash
cd /path/to/traffic-filter
```

#### 2. Build Images

```bash
docker-compose build
```

#### 3. Run Services

```bash
docker-compose up -d
```

**Verifikasi running**:
```bash
docker-compose ps
```

Output:
```
NAME                COMMAND             SERVICE      STATUS
botengine           ./botengine         botengine    Up
nginx               nginx -g daemon off gateway      Up
```

#### 4. Akses

- **Gateway**: http://localhost:80
- **Fingerprint Page**: http://localhost:80/fp.html?dl=https://target.com&bl=https://blacklist.com

#### 5. Logs

```bash
# Semua services
docker-compose logs -f

# Botengine only
docker-compose logs -f botengine

# Nginx only
docker-compose logs -f gateway
```

#### 6. Stop Services

```bash
docker-compose down
```

---

### Opsi 2: Manual Run (Development)

#### 1. Build Botengine

```bash
cd botengine
go build -o botengine
```

#### 2. Run Botengine

```bash
./botengine
```

Default: listen pada `:8080`

#### 3. Run Nginx (separate terminal)

```bash
# Pastikan nginx.conf sudah di copy ke /etc/nginx/
sudo cp gateway/nginx.conf /etc/nginx/nginx.conf
sudo cp gateway/filter.lua /etc/nginx/
sudo cp js/fp.html /etc/nginx/js/

sudo nginx -s reload
```

#### 4. Access

```bash
curl http://localhost/fp.html
```

---

### Opsi 3: Production Deployment

#### Environment Setup

```bash
# Set mode (default: normal)
export MODE=paranoid  # atau light, normal

# Rebuild botengine
cd botengine
go build -o botengine
```

#### Systemd Service (Ubuntu/Debian)

**File: `/etc/systemd/system/botengine.service`**

```ini
[Unit]
Description=Traffic Filter Bot Engine
After=network.target

[Service]
Type=simple
User=www-data
WorkingDirectory=/opt/traffic-filter/botengine
ExecStart=/opt/traffic-filter/botengine/botengine
Restart=always
RestartSec=10
Environment="MODE=paranoid"
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

**Enable & Start**:
```bash
sudo systemctl daemon-reload
sudo systemctl enable botengine
sudo systemctl start botengine
sudo systemctl status botengine
```

**Logs**:
```bash
sudo journalctl -u botengine -f
```

---

## Configuration

### Environment Variables

| Variable | Default | Options | Fungsi |
|----------|---------|---------|--------|
| `MODE` | `normal` | `light`, `normal`, `paranoid` | Scoring strictness |

- **light**: Score limit allow=25, challenge=55 (permissive)
- **normal**: Score limit allow=40, challenge=70 (balanced)
- **paranoid**: Score limit allow=60, challenge=85 (strict)

### Query Parameters (Frontend)

```
http://localhost/fp.html?dl=<url>&bl=<url>
```

| Parameter | Fungsi |
|-----------|--------|
| `dl` | Destination Link (jika allow) |
| `bl` | Blacklist Link (jika block) |

**Contoh**:
```
http://localhost/fp.html?dl=https://offer.com&bl=https://google.com
```

---

## Monitoring & Debugging

### Check Real-time Logs

```bash
# Docker
docker-compose logs -f botengine

# Systemd
journalctl -u botengine -f

# Manual
# (stdout dari running process)
```

### Parse Log untuk Analytics

```bash
# Count by action
docker-compose logs botengine | grep -E "\[ALLOW\]|\[BLOCK\]|\[CHALLENGE\]" | cut -d'[' -f2 | cut -d']' -f1 | sort | uniq -c

# Filter by IP
docker-compose logs botengine | grep "IP=192.168"

# Extract scores
docker-compose logs botengine | grep -oE "Score=[0-9]+" | sort -u
```

### Debug Mode

Untuk development, set ulang hardcoded scores atau tambah verbose output:

Edit `botengine/main.go`:
```go
if os.Getenv("DEBUG") == "1" {
    fmt.Printf("DEBUG: IP=%s, Score=%d, Action=%s\n", ip, updatedScore, action)
}
```

Run dengan:
```bash
DEBUG=1 ./botengine
```

---

## Performance Considerations

### Rate Limiting

- **Window**: 10 detik
- **Max**: 5 request per IP per window
- **Action**: Return 429 Too Many Requests

### Caching

- **Fingerprint Cache**: 5 menit (unused di fase 4+)
- **Verification Cache**: 10 menit
- **Rate Limit Reset**: 10 detik

### Scoring Cost

- Fingerprint collection: ~1.2-1.6 detik (browser)
- Backend processing: <10ms
- Total latency: ~1.3 detik

### Scalability

Untuk production scale:
- Run multiple botengine instances
- Use load balancer (HAProxy, Nginx upstream)
- Implement Redis untuk shared verification cache
- Consider persistent logging (ELK stack, Datadog)

---

## Troubleshooting

### Issue: "rate limited" response

**Penyebab**: IP kena rate limit (>5 request per 10 detik)

**Solusi**:
```bash
# Tunggu 10 detik atau
# Ubah rateLimitMax di botengine/main.go
```

### Issue: Challenge loop tidak selesai

**Penyebab**: `isVerified(ip)` gagal atau score masih tinggi di reverify

**Solusi**:
1. Check log untuk lihat reverify score
2. Jika score masih tinggi, ubah `challengeLimit`
3. Atau extend delay di `fp.html` dari 2s menjadi lebih lama

### Issue: Nginx tidak forward ke botengine

**Penyebab**: DNS/service routing issue

**Solusi**:
```bash
# Check docker network
docker network inspect traffic-filter_default

# Verify botengine is running
docker ps | grep botengine

# Test connectivity
docker exec nginx curl http://botengine:8080/fp
```

### Issue: Log tidak muncul

**Penyebab**: Stdout buffering atau permission issue

**Solusi**:
```bash
# Set unbuffered output
docker-compose logs botengine --follow

# Atau check systemd
journalctl -u botengine -n 50
```

---

## Security Notes

1. **TLS/HTTPS**: Gunakan reverse proxy dengan SSL certificate (Let's Encrypt)
2. **Rate Limit**: Sudah built-in, tapi consider DDoS protection layer
3. **Header Validation**: Jangan percaya 100% user-supplied data
4. **Blacklist Management**: Validasi URL di `dl` dan `bl` parameter sebelum redirect
5. **Logging Privacy**: Jangan log sensitive data, hanya IP dan score

---

## Future Enhancements

- [ ] Redis integration untuk shared cache di multiple instances
- [ ] Machine learning model untuk behavior scoring
- [ ] CAPTCHA integration (Hcaptcha, reCAPTCHA)
- [ ] IP reputation API integration (abuse.ch, MaxMind)
- [ ] Custom scoring rules per domain
- [ ] Dashboard untuk monitoring real-time
- [ ] Database storage untuk long-term analytics

---

## Support & References

### Related Files
- [nginx.conf](gateway/nginx.conf)
- [filter.lua](gateway/filter.lua)
- [botengine/main.go](botengine/main.go)
- [fp.html](js/fp.html)
- [docker-compose.yml](docker-compose.yml)

### External Resources
- [Go net package](https://golang.org/pkg/net/)
- [Nginx Lua module](https://github.com/openresty/lua-nginx-module)
- [JavaScript Navigator API](https://developer.mozilla.org/en-US/docs/Web/API/Navigator)
- [HTTP Header specs](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers)

---

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2026-05-10 | Fase 1-5 complete, PDR dokumentasi |

---

**Last Updated**: 2026-05-10  
**Maintainer**: Traffic Filter Team
