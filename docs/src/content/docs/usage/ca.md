---
title: CA証明書とiPhone設定
description: 自分専用CAの生成と、iPhoneへのインストール・信頼設定
---

HTTPS 通信にヘッダーを付与するには、対象ドメインの通信を MITM（TLS の復号・再暗号化）する必要があり、そのための CA 証明書・秘密鍵が必須です。**組み込みのCAは使いません**（秘密鍵が公開されており危険なため）。必ず自分専用の CA を生成してください。

## CAを生成する

```bash
./jheader-proxy --gen-ca \
  --ca-cert jheader-proxy-ca-cert.pem \
  --ca-key jheader-proxy-ca-key.pem
```

- `jheader-proxy-ca-cert.pem` … iPhone にインストールする CA 証明書
- `jheader-proxy-ca-key.pem` … CA の秘密鍵。**絶対に Git 管理・共有しないこと**

既存ファイルがある場合は誤って上書きしないよう失敗します。意図して上書きするときは `--force` を付けます。

## 証明書を iPhone に送る

証明書（`jheader-proxy-ca-cert.pem`）**のみ**を送ります。秘密鍵は絶対に送らないでください。プロキシ設定をする**前**に行います。

- **方法A: AirDrop（おすすめ）** — Finder で右クリック → 共有 → AirDrop
- **方法B: ローカルHTTPサーバ** — 証明書のあるフォルダで配信し、iPhone Safari で取得

  ```bash
  cd <証明書のあるフォルダ>
  ipconfig getifaddr "$(route -n get default | awk '/interface:/{print $2}')"
  python3 -m http.server 8000
  # iPhone Safari で http://<MacのIP>:8000/jheader-proxy-ca-cert.pem を開く
  ```
- **方法C: メール / メモ / ファイルApp**

:::note
拡張子は `.pem` のままで iOS は認識します。プロファイルとして開けない場合は `jheader-proxy-ca-cert.crt` にリネームすると確実です（中身は同じ）。
:::

## インストールと信頼の有効化（2段階）

ダウンロードしただけでは有効になりません。**2段階**必要です。

1. **プロファイルのインストール**: 設定 → 一般 → VPNとデバイス管理 → ダウンロード済みプロファイル → インストール
2. **信頼の有効化（必須）**: 設定 → 一般 → 情報 → 証明書信頼設定 → 「jheader-proxy local CA」のスイッチを **ON**

:::danger[手順2を忘れると]
手順1だけで手順2（証明書信頼設定）を忘れると、`tls: unknown certificate` エラーで接続できません。[トラブルシューティング](/jheader-proxy/guides/troubleshooting/)を参照。
:::
