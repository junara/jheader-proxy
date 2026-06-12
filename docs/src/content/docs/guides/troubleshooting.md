---
title: トラブルシューティング
description: よくある問題と対処
---

## `tls: unknown certificate` が出てページが開けない

```text
[MITM] app.example.test:443
[002] WARN: Cannot handshake client app.example.test:443 remote error: tls: unknown certificate
```

`remote error:` は **iPhone 側が証明書を拒否している**という意味です。MITM 自体は動いていますが、署名元の CA を iPhone が信頼していないため弾かれています。ほぼ確実に **証明書信頼設定（手順2）が OFF** です。

1. 設定 → 一般 → 情報 → 証明書信頼設定 → 「jheader-proxy local CA」を **ON**
2. Safari のタブを一度閉じて（アプリスイッチャーから上スワイプで終了）再アクセス

それでも直らない場合:

- iPhone に入れた証明書が、いま起動中のプロキシが使う `jheader-proxy-ca-cert.pem` と同一か確認する（CA を作り直した場合は入れ直す）。Mac 側で確認:

  ```bash
  openssl x509 -in jheader-proxy-ca-cert.pem -noout -fingerprint -sha256
  ```
- iPhone がロックダウンモードだと、ユーザー追加 CA での MITM はブロックされる
- 会社管理端末（MDM）では、ユーザー CA の信頼が制限されていることがある

## ヘッダーが付かない（`[MITM]` が出ない）

- `--domain` の指定とアクセス先ドメインが一致しているか確認する（親ドメイン指定でサブドメインは対象、別ドメインは対象外）
- iPhone の Wi-Fi プロキシ設定が正しい Mac の IP・ポートを向いているか確認する

## 接続が拒否される（`[DENY]` が出る）

`--allow` を指定している場合、許可リストにないクライアントは拒否されます。iPhone の IP を `--allow` に含めてください。

## `ipconfig getifaddr en0` が何も出力しない

Wi-Fi が `en0` でない、または `en0` に IP が無い場合に起こります。インターフェース名に依存しない方法を使います。

```bash
ipconfig getifaddr "$(route -n get default | awk '/interface:/{print $2}')"
# VPN 中などで上記が当てにならない場合は、127.0.0.1 以外の IPv4 を一覧して選ぶ
ifconfig | awk '/inet / && $2 !~ /^127\./ {print $2}'
```

## 起動してすぐ終了する

既定で 10分（`--duration`）の自動停止があります。意図せず終了する場合は `--duration` を延ばすか `0` で無制限にしてください。
