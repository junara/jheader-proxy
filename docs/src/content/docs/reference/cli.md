---
title: CLI引数
description: jheader-proxy の全コマンドライン引数
---

## 一覧

| 引数 | 説明 |
| --- | --- |
| `--listen` | プロキシの待ち受けアドレス。デフォルトは `:8080` |
| `--domain` | ヘッダー追加対象のドメイン。複数回指定可能。サブドメインも対象。1つ以上必須 |
| `--header` | 追加するヘッダーを `Name=Value` 形式で指定。複数回指定可能。1つ以上必須 |
| `--ca-cert` | HTTPS MITM に使う CA 証明書 PEM のパス。必須 |
| `--ca-key` | HTTPS MITM に使う CA 秘密鍵 PEM のパス。必須 |
| `--duration` | この時間が過ぎると自動停止する。デフォルト `10m`。`0` で無制限 |
| `--allow` | 接続を許可するクライアントの IP または CIDR。複数回指定可能。未指定なら全許可 |
| `--redact` | 起動ログで全ヘッダー値をマスクする |
| `--quiet` | リクエストごとのログを抑制する |
| `--verbose` | 対象ドメインのレスポンスもログ出力する |
| `--gen-ca` | `--ca-cert`/`--ca-key` のパスに新しい CA を生成して終了する |
| `--force` | `--gen-ca` 時に既存ファイルを上書きする |
| `--version` | バージョンを表示して終了する |
| `--gui` | ローカル Web 管理画面を起動する（[Web GUI](/jheader-proxy/usage/gui/)） |
| `--gui-listen` | `--gui` 時の管理画面の待受アドレス。デフォルトは `127.0.0.1:9090` |
| `--no-open` | `--gui` 時にブラウザを自動起動しない |

## 起動例

```bash
./jheader-proxy \
  --listen ":8080" \
  --domain "example.test" \
  --header "X-Debug-User=jun" \
  --ca-cert jheader-proxy-ca-cert.pem \
  --ca-key jheader-proxy-ca-key.pem
```

## CA生成例

```bash
./jheader-proxy --gen-ca \
  --ca-cert jheader-proxy-ca-cert.pem \
  --ca-key jheader-proxy-ca-key.pem
```

- RSA 2048bit、有効期限約10年の自己署名 CA を生成します
- 秘密鍵ファイルはパーミッション `0600` で書き出します
- 既存ファイルがある場合はエラー（`--force` で上書き）

## GUI起動例

```bash
./jheader-proxy --gui
# 管理画面ポートを変える場合
./jheader-proxy --gui --gui-listen 127.0.0.1:9191
```

`http://127.0.0.1:9090`（または指定ポート）で管理画面が開きます。詳しくは [Web GUI](/jheader-proxy/usage/gui/) を参照してください。

## エラー終了する条件

- `--domain` / `--header` が未指定
- `--header` に `=` が無い、またはヘッダー名が空
- `--ca-cert` / `--ca-key` が未指定、または読み込み失敗（CA 証明書でない場合を含む）
- `--gen-ca` で出力先に既存ファイルがある（`--force` 未指定時）
- `--allow` の指定が IP / CIDR として不正
- listen に失敗、プロキシ起動に失敗
