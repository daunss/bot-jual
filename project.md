# WA Sales Bot — Project.md

> **Stack**: Go (WhatsMeow) · Gemini (Flash Lite) · Atlantic H2H (PPOB) · Supabase (Postgres + Auth + Storage) · Optional Node/TS worker · Redis (cache) · Docker · Prometheus/Grafana

## Ringkasan
Bot WhatsApp penjualan dengan pengalaman chat seperti manusia. Bot memanfaatkan **Gemini (Flash Lite)** sebagai otak percakapan (NLU/NLG + multimodal: teks, **gambar**, **voice note**), terintegrasi penuh dengan **Atlantic H2H** untuk produk/top‑up, tagihan pascabayar, deposit & transfer. **Supabase** menyimpan profil, preferensi, histori transaksi, dan log percakapan. Tersedia **rotasi multi API‑key Gemini** (failover + cooldown 24 jam bila quota tercapai).

---

## Tujuan & Non‑Tujuan
**Tujuan**
- Percakapan natural (bahasa Indonesia santai) untuk discovery produk, cek harga, dan transaksi tanpa tombol.
- Pencarian produk by **kategori/kata kunci** (contoh: “viu berapa?”) + **filter budget** (contoh: “saya cuma punya 5000”).
- Dukungan **multimodal**: user kirim **gambar** (mis. screenshot paket, kartu game) atau **VN** (voice note) → dipahami Gemini.
- Integrasi penuh **Atlantic H2H** (prabayar, pascabayar, deposit, transfer) + **webhook** status.
- **Failover Gemini** otomatis saat key limit/429 → switch ke key pengganti; key yang limit **diistirahatkan 24 jam**.

**Non‑Tujuan**
- Tidak membangun panel admin UI penuh (hanya endpoint & tabel data). Bisa ditambah di fase selanjutnya.
- Tidak memproses pembayaran on‑premise; mengandalkan mekanisme deposit/transfer Atlantic & saldo akun H2H.

---

## Arsitektur Singkat
```
WhatsApp (WA)
  │
  ▼
Go App (WhatsMeow) ──┬─ Intent Router (Gemini Flash Lite)
                      │
                      ├─ Tooling Layer (Atlantic H2H SDK ringan)
                      │     ├─ Price List / Transaksi / Tagihan / Deposit / Transfer
                      │     └─ Webhook Receiver (HTTP)
                      │
                      ├─ Media Pipeline
                      │     ├─ VN → audio bytes → Gemini (transcribe + intent)
                      │     └─ Gambar → bytes → Gemini (vision + intent)
                      │
                      ├─ State & Memory
                      │     └─ Supabase (users, messages, orders, deposits, api_keys, rate_limits)
                      │
                      └─ Cache & Circuit Breaker
                            ├─ Redis (price list cache, budget map)
                            └─ Gemini key rotator (cooldown 24h + exponential backoff)
```

---

## Fitur Utama
- **Salam & Small‑talk** (tone ramah, singkat): “Selamat pagi!” → balas otomatis + pertanyaan kontekstual.
- **Cari Produk** / **Cek Harga** (contoh: “viu berapa?”): fuzzy match *code/name/category/provider* + saran.
- **Filter Budget** (contoh: “punya 5000”): tampilkan opsi **≤ 5000** dan status *available*.
- **Top‑up Prabayar**: pilih layanan → `create transaksi` → polling / webhook status → notifikasi sukses + SN.
- **Cek & Bayar Tagihan** (pascabayar): `cek tagihan` → konfirmasi → `bayar` → notifikasi status.
- **Deposit**: daftar metode → buat deposit (QRIS/Bank/VA/E‑wallet) → pantau status → saldo H2H update.
- **Transfer**: list bank/ewallet → cek rekening → buat transfer → cek status.
- **Multimodal**:
  - **VN** → transkripsi + intent (contoh: user menyebut “top up ML 86 diamond ID 123456”).
  - **Gambar** → ekstraksi teks/konten (contoh: screenshot paket VIU, nomor pelanggan PLN) → intent.
- **Gemini Failover**: jika API‑Key #1 limit → tandai cooldown 24 jam, pakai Key #2 (urutan prioritas), otomatis re‑enable setelah cooldown berakhir.

---

## Tumpukan Teknologi
- **Go 1.22+** — service utama WA (whatsmeow), webhook server, adapter Atlantic & Supabase.
- **WhatsMeow** — WA client (login QR, inbound/outbound message, media download/upload).
- **Gemini (Flash Lite)** — NLU/NLG + vision + audio; model ID via ENV (mis: `GEMINI_MODEL_FLASH_LITE`).
- **Supabase** — Postgres + Auth + Storage (opsional); gunakan Row Level Security bila perlu.
- **Redis** — cache price list (TTL), rate limit counters, lock transaksi.
- **Docker** — kontainerisasi & deployment; **Prometheus/Grafana** untuk metrik.

---

## Struktur Proyek (usulan)
```
wa-sales-bot/
├─ cmd/
│  └─ app/main.go
├─ internal/
│  ├─ wa/                  # whatsmeow session, handlers, media
│  ├─ nlu/                 # gemini client, prompts, tool-calling
│  ├─ atl/                 # atlantic h2h client (endpoints + webhook verifier)
│  ├─ convo/               # intent router, dialog policies, budget logic
│  ├─ repo/                # supabase queries
│  ├─ cache/               # redis utils
│  ├─ rate/                # gemini key rotator + circuit breaker
│  ├─ httpapi/             # webhook server + health + admin-lite
│  └─ util/                # common (errors, logger, config)
├─ migrations/             # SQL for Supabase schema
├─ docs/                   # this Project.md + API notes
├─ .env.example
├─ docker-compose.yml
└─ Makefile
```

---

## Konfigurasi & ENV
Buat `.env` (lihat `.env.example`):
```
# WhatsApp
WA_DEVICE_DB=./device.db
WA_LOG_LEVEL=info

# Gemini
GEMINI_KEYS=key1,key2,key3         # urutan prioritas
GEMINI_MODEL_FLASH_LITE=gemini-1.5-flash-lite  # atau set sesuai katalog
GEMINI_TIMEOUT_MS=20000
GEMINI_MAX_TOKENS=1024
GEMINI_COOLDOWN_HOURS=24

# Atlantic H2H
ATL_BASE_URL=https://atlantich2h.com
ATL_API_KEY=xxx
ATL_WEBHOOK_SECRET_MD5_USERNAME=<md5_username_expected>

# Supabase
SUPABASE_URL=...
SUPABASE_ANON=...
SUPABASE_SERVICE_ROLE=...

# Redis
REDIS_URL=redis://localhost:6379

# Server
HTTP_ADDR=:8080
PUBLIC_BASE_URL=https://your-domain.com
```

---

## Skema Database (Supabase / Postgres)
**users**
- id (uuid, pk)
- wa_jid (text, unique)
- display_name (text)
- first_seen_at (timestamptz)
- last_seen_at (timestamptz)
- note (text)

**messages**
- id (uuid, pk)
- user_id (uuid, fk users)
- direction (enum: inbound|outbound)
- type (enum: text|image|audio|other)
- content_text (text, nullable)
- media_url (text, nullable)  
- meta (jsonb)  
- created_at (timestamptz)

**orders** (prabayar/pascabayar)
- id (uuid, pk)
- user_id (uuid)
- atl_id (text)          # id dari Atlantic
- reff_id (text)
- code (text)            # code layanan
- layanan (text)
- target (text)
- price (bigint)
- status (text)          # pending|success|failed
- sn (text, nullable)
- created_at, updated_at

**deposits**
- id (uuid, pk)
- user_id (uuid)
- atl_id (text)
- reff_id (text)
- metode (text)
- nominal (bigint)
- fee (bigint)
- get_balance (bigint)
- status (text)          # pending|processing|success|expired|failed
- created_at, updated_at

**transfers**
- id (uuid, pk)
- user_id (uuid)
- atl_id (text)
- reff_id (text)
- bank_code (text)
- nomor_tujuan (text)
- nominal (bigint)
- fee (bigint)
- total (bigint)
- status (text)
- created_at, updated_at

**api_keys** (Gemini)
- id (uuid, pk)
- key (text, encrypted at rest)
- priority (int)
- cooldown_until (timestamptz, nullable)
- last_error (text, nullable)
- created_at, updated_at

**price_cache** (opsional jika ingin persist)
- id (uuid, pk)
- type (text)        # prabayar|pascabayar
- payload (jsonb)
- fetched_at (timestamptz)
- ttl_sec (int)

---

## Desain Percakapan & Intent
**Intents kunci**:
- `greet` — salam/ice‑breaker.
- `price_lookup` — "viu berapa", "daftar harga games".
- `budget_filter` — "punya 5000", "budget 20 rb".
- `topup_create` — buat transaksi prabayar.
- `bill_check` / `bill_pay` — cek & bayar tagihan pascabayar.
- `deposit_*` — daftar metode, create, status, instant, cancel.
- `transfer_*` — list bank, cek rekening, create, status.
- `order_status` — cek status transaksi (by reff_id/id).
- `smalltalk` — selingan / klarifikasi.

**Kebijakan dialog** (ringkas):
1) Normalisasi input (text hasil transkripsi VN / OCR gambar).  
2) Panggil **Gemini** untuk klasifikasi intent + ekstrak slot (code, target, nominal, budget).  
3) Jika butuh data H2H → panggil tool Atlantic.  
4) Balas ringkas & kontekstual; konfirmasi saat tindakan yang *berisiko* (pembayaran/tagihan/transfer).  

**Contoh Prompt System (Gemini)**
- Persona ramah, singkat, fokus menjawab & bertanya balik bila slot kurang.
- Tool‑calling JSON schema (lihat *Tooling Atlantic* di bawah).
- Instruksi: bahasa Indonesia, hindari jargon teknis ke end‑user.

---

## Integrasi Atlantic H2H (Ringkas)
> Base URL configurable via `ATL_BASE_URL` (default `https://atlantich2h.com`).  
> Semua POST **form‑urlencoded**.  
> Gunakan `reff_id` unik pada create transaksi/deposit/transfer (idempotency).  
> Webhooks: `Content-Type: application/json`, header `X-ATL-Signature: md5(username)` — verifikasi dibanding hash yang diharapkan.

**Fungsi inti (ringkasan endpoint)**
- **Price List** (prabayar/pascabayar): `/layanan/price_list`
- **Create Transaksi (prabayar)**: `/transaksi/create`
- **Status Transaksi**: `/transaksi/status`
- **Cek Tagihan**: `/transaksi/tagihan`
- **Bayar Tagihan**: `/transaksi/tagihan/bayar`
- **Deposit**: metode `/deposit/metode`, create `/deposit/create`, status `/deposit/status`, instant `/deposit/instant`, cancel `/deposit/cancel`
- **Transfer**: list bank `/transfer/bank_list`, cek rekening `/transfer/cek_rekening`, create `/transfer/create`, status `/transfer/status`
- **Webhooks**: prabayar `event: "transaksi"`, pascabayar `event: "transaksi.pascabayar"`, transfer `event: "transfer"`, deposit `event: "deposit"`.

**Kaidah implementasi**
- Timeouts: 15–20s, retry 2x (idempotent ops saja).  
- Mapping status → user‑friendly (pending/processing/success/failed/expired).  
- Cache **price list** di Redis (TTL 5–15 menit) untuk respon cepat (budget & pencarian).

---

## Tooling Atlantic (Schema untuk NLU → Action)
> Agar Gemini bisa “memanggil tool”, definisikan daftar fungsi berikut di layer NLU:

- `price_list(type: "prabayar"|"pascabayar", code?: string)`
- `transaksi_create(code: string, reff_id: string, target: string, limit_price?: int)`
- `transaksi_status(id: string, type: "prabayar"|"pascabayar")`
- `tagihan_cek(code: string, reff_id: string, customer_no: string)`
- `tagihan_bayar(code: string, reff_id: string, customer_no: string)`
- `deposit_metode(type?: string, metode?: string)`
- `deposit_create(reff_id: string, nominal: int, type: string, metode: string)`
- `deposit_status(id: string)`
- `deposit_instant(id: string, action: boolean)`
- `deposit_cancel(id: string)`
- `bank_list()`
- `cek_rekening(bank_code: string, account_number: string)`
- `transfer_create(ref_id: string, kode_bank: string, nomor_akun: string, nama_pemilik: string, nominal: int, email?: string, phone?: string, note?: string)`
- `transfer_status(id: string)`

Di setiap fungsi, lakukan:
1) Validasi parameter.  
2) Panggil endpoint Atlantic.  
3) Translasi respon ke format ringkas untuk end‑user.  
4) Catat hasil ke Supabase (orders/deposits/transfers).  

---

## Logika Budget & Pencarian Produk
- **Fuzzy match**: normalisasi input → cari pada field `code|name|category|provider` (lowercase, de‑accent).  
- **Budget filter**: ambil **price list prabayar**, filter `status == "available"` dan `price <= budget`, sort ascending.  
- **Ambiguitas**: bila hasil >5 item → tampilkan 5 teratas + tombol teks saran (atau minta spesifik).  
- **Contoh**: “punya 5000” → tampilkan produk ≤ 5000 (mis: paket game mingguan, pulsa kecil) + ajak pilih.

---

## Multimodal (VN & Gambar)
**VN (Voice Note)**
- Unduh media via WhatsMeow → deteksi mime → kirim ke Gemini (audio) untuk **transkripsi + intent**.  
- Prompt: “Bahasa Indonesia, transkripsikan apa adanya, lalu tentukan intent & slot (code/target/nominal/budget).”

**Gambar**
- Unduh media → kirim ke Gemini (vision) untuk **OCR ringan** (contoh: nomor pelanggan, nama paket, kode).  
- Keamanan: hindari menyimpan gambar sensitif; hash nama file & TTL di Storage bila perlu.

---

## Failover & Rate Limit Gemini
- Simpan beberapa key di tabel **api_keys** berurutan by `priority`.
- **Rotasi**:
  1) Coba key aktif. Jika `429/Quota`/`rate_limit_exceeded` → set `cooldown_until = now + 24h`.
  2) Pilih key berikutnya (yang `cooldown_until` null atau sudah lewat).  
  3) Catat `last_error` & metrik.  
- **Reaktifasi**: background job tiap 5 menit mengecek key yang cooldown; re-enable jika lewat 24 jam.
- **Circuit breaker**: tutup penggunaan key yang sering error 5xx singkat (backoff 30s → 2m → 10m).

---

## Keamanan
- Simpan secret di ENV/secret store. Enkripsi field sensitif (api_keys.key) di DB.
- Validasi input target/nomor pelanggan (regex/len).  
- Webhook: verifikasi `X-ATL-Signature` terhadap hash yang diharapkan; log & tolak bila mismatch.  
- Idempotensi via `reff_id`.  
- Batasi command admin (whitelist JID).  
- PII hygiene: minimalkan retensi media; gunakan TTL & signed URL Supabase.

---

## Observabilitas
- **Logging**: zap/logrus (structured). Correlate by `reff_id` atau `wa_message_id`.
- **Metrics**: Prometheus — latensi Atlantic, success rate, quota hits Gemini, cooldown keys aktif, cache hit rate.
- **Tracing**: OpenTelemetry (opsional).

---

## Build & Run (Local)
```
make dev     # go run cmd/app/main.go (hot reload opsional dengan air)
make test    # unit test
make docker  # build image
```
**Langkah awal**
1) Login WA via WhatsMeow (scan QR).  
2) Set `ATL_API_KEY`, `GEMINI_KEYS`, `SUPABASE_*`.  
3) Jalankan server `:8080`.  
4) Konfigurasikan Webhook URL di dashboard Atlantic → arahkan ke `POST /webhook/atlantic`.

---

## Endpoint Internal (Server Kita)
- `POST /webhook/atlantic` — menerima semua event (prabayar/pascabayar/transfer/deposit).  
- `GET  /healthz` — kesehatan app.  
- `GET  /metrics` — Prometheus.  
- `POST /admin/reload-price-cache` — refresh manual (admin‑only).

---

## Alur Contoh
**1) “Selamat pagi”**
- Balasan: “Selamat pagi! Mau cek harga atau top up produk apa nih? 😊”

**2) “viu berapa?”**
- Intent `price_lookup` → `price_list(type=prabayar)` → fuzzy "viu" → tampilkan variasi (mingguan/bulanan).

**3) “saya cuma punya 5000”**
- Intent `budget_filter` → filter list `price <= 5000` → daftar 3–5 opsi.

**4) “topup jenis A ke 0812xxxx”**
- Konfirmasi ringkas → `transaksi_create` → tunggu webhook/status → notifikasi + SN bila success.

**5) “tagihan PLN 123456 bayar”**
- `tagihan_cek` → render detail & total → minta konfirmasi → `tagihan_bayar` → notifikasi status.

**6) “deposit 50k via qris”**
- `deposit_create` → kirim QR (image/url) → pantau status (webhook) → saldo bertambah.

**7) “tf 100k ke dana 08xxxx a.n. Budi”**
- `cek_rekening` → konfirmasi → `transfer_create` → notifikasi status → simpan riwayat.

---

## Penanganan Error & Edge Cases
- Atlantic timeout: beritahu user bahwa sistem lagi padat, kita ulangi otomatis.
- Produk *empty*: tawarkan alternatif kategori serupa.
- `limit_price` dilanggar: minta konfirmasi revisi atau ganti varian.
- Media gagal diunduh: minta user kirim ulang.
- Gemini `quota`: rotasi key; jika semua cooldown → fallback template FAQ singkat + janji coba lagi nanti.

---

## Rencana Kerja (5 Fase)
**Fase 1 — Core WA & Data (H+1)**
- Setup repo, config, logging.
- WhatsMeow: login, handler text/media dasar.
- Supabase schema + migrasi + repo layer.
- Health/metrics endpoint.

**Fase 2 — Otak Percakapan (H+3)**
- Integrasi Gemini (Flash Lite): prompts, intent extractor, tool schema.
- Small‑talk + intent `price_lookup`, `budget_filter`.
- Cache price list + fuzzy search + budget filter.

**Fase 3 — Atlantic H2H Transaksi (H+7)**
- Implement semua endpoint inti (create/status/tagihan/bayar/deposit/transfer).
- Webhook receiver + verifikasi signature + penyimpanan ke DB.
- Alur konfirmasi transaksi & notifikasi hasil.

**Fase 4 — Multimodal (H+10)**
- VN → transkripsi + intent; Gambar → OCR ringan + intent.
- UX balasan ringkas khusus media (contoh penggalan isi & konfirmasi).

**Fase 5 — Hardening & Deploy (H+14)**
- Gemini key rotator + circuit breaker + metrik.
- Rate limit, retry, idempotensi.
- Docker, monitoring, alerting; dokumentasi operasional.

---

## Contoh Pseudocode Kritis
**Rotasi Gemini Key (sederhana)**
```go
func withGemini(fn func(client *GeminiClient) error) error {
    keys := repo.ListActiveKeysOrderedByPriority()
    var lastErr error
    for _, k := range keys {
        if k.CooldownUntil != nil && time.Now().Before(*k.CooldownUntil) { continue }
        c := NewGeminiClient(k.Value)
        err := fn(c)
        if err == nil { return nil }
        if isQuotaErr(err) {
            repo.SetCooldown(k.ID, time.Now().Add(24*time.Hour))
        }
        lastErr = err
    }
    return lastErr
}
```

**Budget Filter**
```go
func ListAffordable(typeStr string, max int64) ([]Item, error) {
    list := cache.GetPriceList(typeStr)
    if list == nil { list = atl.PriceList(typeStr) }
    items := Filter(list, func(x Item){ return x.Status=="available" && x.Price<=max })
    sort.Slice(items, func(i,j int){ return items[i].Price<items[j].Price })
    return items[:min(5,len(items))], nil
}
```

---

## Testing
- **Unit**: parser intent, budget filter, rotator key, mapper status Atlantic.
- **Integration**: mock Atlantic (httptest), webhook end‑to‑end, WhatsMeow handler.
- **Load**: cache price list, parallel transaksi create/status.

---

## Roadmap Tambahan
- Admin panel harga & transaksi (Next.js + Supabase Auth).
- Notifikasi broadcast status gangguan produk tertentu.
- Smart upsell (bundling murah sesuai budget user).

---

## Lisensi & Kredit
Internal project. Ikuti ToS WhatsApp & kebijakan penggunaan Gemini/Atlantic. Pastikan nomor WA & akun H2H mematuhi regulasi setempat.

