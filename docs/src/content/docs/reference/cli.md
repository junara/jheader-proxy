---
title: コマンドとオプション
description: jheader-proxy の全コマンドとコマンドライン引数
---

`jheader-proxy` はサブコマンドで動作モードを選びます。各コマンドの詳細は `jheader-proxy <コマンド> --help` でも確認できます。

## コマンド一覧

| コマンド | 説明 |
| --- | --- |
| `run` | ヘッダーを付与するプロキシを起動する |
| `gen-ca` | 自分専用の CA 証明書・秘密鍵を生成する（HTTPS に必須） |
| `gui` | ブラウザで操作するローカル Web 管理画面を起動する（[Web GUI](/jheader-proxy/usage/gui/)） |
| `version` | バージョンを表示して終了する（`--version` でも可） |
| `help` | 使い方を表示する（`help <コマンド>` で各コマンドの詳細） |

:::note[従来のフラグ形式について]
`--gen-ca` / `--gui` やサブコマンドなしの実行など、従来のフラグ形式も後方互換のため動作しますが**非推奨**です（起動時に警告を表示します）。今後はサブコマンドを使ってください。`--version` はフラグとしても慣習的なので警告なしで受け付けます。
:::

## `run` のオプション

| 引数 | 説明 |
| --- | --- |
| `--config` | 設定をまとめた JSON ファイルのパス。GUI の `config.json` と互換。コマンドライン引数が優先される |
| `--listen` | プロキシの待ち受けアドレス。デフォルトは `:8080` |
| `--domain` | ヘッダー追加対象のドメイン。複数回指定可能。サブドメインも対象。1つ以上必須 |
| `--header` | 追加するヘッダーを `Name=Value` 形式で指定。複数回指定可能。`--header-file` と合わせて1つ以上必須 |
| `--header-file` | ヘッダーを書いたファイルのパス（1行1件の `Name=Value`。空行と `#` 始まりの行は無視）。複数回指定可能。**トークン等の秘匿値はシェル履歴や `ps` 出力に残る `--header` ではなくこちらで渡す** |
| `--ca-cert` | HTTPS MITM に使う CA 証明書 PEM のパス。必須 |
| `--ca-key` | HTTPS MITM に使う CA 秘密鍵 PEM のパス。必須 |
| `--duration` | この時間が過ぎると自動停止する。デフォルト `10m`。`0` で無制限 |
| `--allow` | 接続を許可するクライアントの IP または CIDR。複数回指定可能。未指定なら全許可 |
| `--redact` | 起動ログで全ヘッダー値をマスクする |
| `--quiet` | リクエストごとのログを抑制する（`--verbose` とは併用不可） |
| `--verbose` | 対象ドメインのレスポンスもログ出力する（`--quiet` とは併用不可） |

## `gen-ca` のオプション

| 引数 | 説明 |
| --- | --- |
| `--cert` | 生成する CA 証明書 PEM の出力先パス。必須（`--ca-cert` でも可） |
| `--key` | 生成する CA 秘密鍵 PEM の出力先パス。必須（`--ca-key` でも可） |
| `--force` | 既存ファイルを上書きする |

## `gui` のオプション

| 引数 | 説明 |
| --- | --- |
| `--listen` | 管理画面の待受アドレス。デフォルトは `127.0.0.1:9090` |
| `--no-open` | ブラウザを自動起動しない |

## 起動例

```bash
./jheader-proxy run \
  --listen ":8080" \
  --domain "example.test" \
  --header "X-Debug-User=jun" \
  --ca-cert jheader-proxy-ca-cert.pem \
  --ca-key jheader-proxy-ca-key.pem
```

## CA生成例

```bash
./jheader-proxy gen-ca \
  --cert jheader-proxy-ca-cert.pem \
  --key jheader-proxy-ca-key.pem
```

- RSA 2048bit、有効期限約10年の自己署名 CA を生成します
- 秘密鍵ファイルはパーミッション `0600` で書き出します
- 既存ファイルがある場合はエラー（`--force` で上書き）

## GUI起動例

```bash
./jheader-proxy gui
# 管理画面ポートを変える場合
./jheader-proxy gui --listen 127.0.0.1:9191
```

`http://127.0.0.1:9090`（または指定ポート）で管理画面が開きます。詳しくは [Web GUI](/jheader-proxy/usage/gui/) を参照してください。

## エラー終了する条件

- `--domain` / `--header`（または `--header-file`）が未指定
- `--header` に `=` が無い、またはヘッダー名が空
- `--ca-cert` / `--ca-key` が未指定、または読み込み失敗（CA 証明書でない場合を含む）
- `gen-ca` で出力先に既存ファイルがある（`--force` 未指定時）
- `--allow` の指定が IP / CIDR として不正
- `--quiet` と `--verbose` の同時指定
- listen に失敗（アドレス使用中など）、プロキシ起動に失敗

:::tip[エラー時のヒント]
CA ファイルが見つからないときは `gen-ca` での生成コマンド例を、待受アドレスが使用中のときは `--listen` でのポート変更案内を、エラーメッセージに添えて表示します。
:::
