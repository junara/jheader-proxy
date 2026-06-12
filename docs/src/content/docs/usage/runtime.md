---
title: 実行時オプション
description: 自動停止・クライアント制限・ログ制御・マスキング
---

## 自動停止（--duration）

停止忘れを防ぐため、起動から一定時間で自動的に穏当に停止します。**既定は10分**です。

```bash
--duration 30m   # 30分に延長
--duration 0     # 無制限（手動停止）
```

起動時に `auto-stop after 10m0s` のようにログ表示します。`Ctrl+C`（SIGINT/SIGTERM）でも同じ経路で穏当に停止します（`shutting down...`）。

## クライアント制限（--allow）

`--listen ":8080"` は全インターフェースで待ち受けるため、同じ Wi-Fi の第三者にもプロキシを使われ得ます。`--allow` で接続元 IP / CIDR を限定できます（複数回指定可、未指定なら全許可）。

```bash
--allow 192.168.1.23          # iPhone の IP だけ許可
--allow 192.168.1.0/24        # CIDR も可
```

許可リストにないクライアントは接続を拒否し、`[DENY] <IP>` をログ出力します。

## 機密ヘッダーのマスキング（--redact）

`Authorization` / `Cookie` / `Set-Cookie` / `X-Api-Key` / `Proxy-Authorization` は、`--redact` 無しでも起動ログでは値が `***` にマスクされます。

```bash
--redact   # すべてのヘッダー値を起動ログでマスク
```

これは起動バナーの表示のみが対象です（`[ADD HEADER]` 行は元から URL のみ）。

## ログ量の制御（--quiet / --verbose）

```bash
--quiet     # リクエストごとのログ（[MITM]/[TUNNEL]/[ADD HEADER]）を抑制
--verbose   # 対象ドメインのレスポンスも [RESP] として出力
```

詳しくは[ログ](/jheader-proxy/reference/logs/)を参照してください。
