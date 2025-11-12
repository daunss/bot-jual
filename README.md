# Bot Jual WA

Bot WhatsApp penjualan dengan pengalaman obrolan seperti manusia yang didukung oleh Gemini, terintegrasi dengan Atlantic H2H untuk produk digital, dan menggunakan Supabase untuk penyimpanan data.

## Tumpukan Teknologi

* **Go (WhatsMeow)**: Layanan utama WhatsApp
* **Gemini (Flash Lite)**: Pemahaman dan pembuatan bahasa alami
* **Atlantic H2H**: Produk digital (PPOB)
* **Supabase (Postgres)**: Penyimpanan data
* **Redis**: Cache
* **Docker**: Kontainerisasi
* **Prometheus/Grafana**: Metrik

## Fitur Utama

- **Percakapan Alami**: Terlibat dalam obrolan santai untuk penemuan produk, pemeriksaan harga, dan transaksi.
- **Pencarian Fleksibel**: Cari produk berdasarkan kategori, kata kunci, atau bahkan filter anggaran.
- **Dukungan Multimodal**: Memahami pertanyaan berbasis teks, gambar, dan catatan suara.
- **Integrasi Penuh**: Bekerja mulus dengan Atlantic H2H untuk semua transaksi.
- **Failover Cerdas**: Secara otomatis beralih ke kunci API Gemini cadangan jika batas tercapai.

## Arsitektur

Arsitekturnya terdiri dari aplikasi Go yang menggunakan WhatsMeow untuk terhubung ke WhatsApp. Aplikasi ini memanfaatkan Gemini untuk pemrosesan bahasa alami, Atlantic H2H untuk transaksi, Supabase untuk penyimpanan data, dan Redis untuk caching.

```
WhatsApp (WA)
  │
  ▼
Aplikasi Go (WhatsMeow) ──┬─ Router Intent (Gemini Flash Lite)
                      │
                      ├─ Lapisan Perkakas (SDK Atlantic H2H ringan)
                      │
                      ├─ Alur Media (VN & Gambar)
                      │
                      ├─ Keadaan & Memori (Supabase)
                      │
                      └─ Cache & Pemutus Sirkuit (Redis)
```

## Konfigurasi

1. Buat file `.env` dari `.env.example`.
2. Isi variabel lingkungan yang diperlukan:
   - Kredensial WhatsApp
   - Kunci API Gemini
   - Kredensial Atlantic H2H
   - Kredensial Supabase
   - URL Redis
   - Pengaturan server

## Menjalankan Secara Lokal

1. **Login ke WhatsApp**: Pindai kode QR yang dihasilkan oleh WhatsMeow.
2. **Atur Variabel Lingkungan**: Pastikan file `.env` Anda dikonfigurasi dengan benar.
3. **Jalankan Server**: `buat dev`
4. **Konfigurasi Webhook**: Arahkan webhook Atlantic H2H Anda ke `POST /webhook/atlantic`.
