---
title: ログ
description: 起動時・リクエストごとのログの読み方
---

## 起動時

```text
proxy listening on :8080
target domains: example.test
CA certificate: jheader-proxy-ca-cert.pem
auto-stop after 10m0s
allowed clients: 192.168.1.23
headers:
  X-Debug-User: jun
  Authorization: ***
```

- `auto-stop after ...` … `--duration` が有効なとき
- `allowed clients: ...` … `--allow` を指定したとき
- ヘッダー値の `***` … 機密ヘッダー、または `--redact` 指定時

## リクエストごと

```text
[MITM] app.example.test:443
[ADD HEADER] GET https://app.example.test/
[TUNNEL] www.google.com:443
[RESP] 200 https://app.example.test/
[DENY] 192.0.2.10:54321
```

| ログ | 意味 |
| --- | --- |
| `[MITM]` | 対象ドメインの HTTPS を MITM した |
| `[ADD HEADER]` | 対象リクエストにヘッダーを付与した |
| `[TUNNEL]` | 対象外の HTTPS を素通しした |
| `[RESP]` | `--verbose` 時、対象ドメインのレスポンス |
| `[DENY]` | `--allow` で許可されないクライアントを拒否した |

`--quiet` を指定すると `[MITM]` / `[TUNNEL]` / `[ADD HEADER]` を抑制します（`[DENY]` と起動バナーは出力されます）。
