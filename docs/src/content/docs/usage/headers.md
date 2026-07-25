---
title: ヘッダー付与と対象ドメイン
description: ヘッダーの指定方法と、対象ドメインの判定ルール
---

## ヘッダーの指定

`--header "Name=Value"` 形式で指定します。複数回指定でき、`--header-file`（後述）と合わせて1つ以上必須です。

```bash
--header "X-Debug-User=jun"
--header "X-From-iPhone-Proxy=true"
--header "Authorization=Bearer dummy-token"
```

ルール:

- 最初の `=` で分割し、名前・値とも前後の空白を除去します
- `=` が無い指定（`X-Debug-User`）はエラー
- ヘッダー名が空（`=value`）はエラー
- 値は空文字（`X-Empty=`）を許可
- 同名を複数指定した場合は後勝ち

値は常に文字列として送られます（例: `123` は数値ではなく文字列）。

## ファイルからヘッダーを渡す（--header-file）

認証トークンなどの秘匿値は、`--header` で直接渡すとシェル履歴や `ps` の出力に残ります。1行1件の `Name=Value` を書いたファイルを `--header-file` で渡せば、これを避けられます。

```bash
./jheader-proxy run \
  --domain example.test \
  --header-file headers.txt \
  --ca-cert jheader-proxy-ca-cert.pem \
  --ca-key jheader-proxy-ca-key.pem
```

```text title="headers.txt"
# 検証用の秘匿ヘッダー（この行はコメント）
Authorization=Bearer dummy-token
X-Debug-User=jun
```

- 空行と `#` 始まりの行は無視します
- 書式・trim・重複解決のルールは `--header` と同じです
- `--header` と併用でき、同名は `--header` の明示指定が勝ちます
- ファイルには認証情報が入りうるため、パーミッションを絞ってください（例: `chmod 600 headers.txt`）

詳しくは[セキュリティ](/jheader-proxy/guides/security/)を参照してください。

## 対象ドメインの判定

`--domain` で対象ドメインを指定します。複数回指定でき、1つ以上必須です。指定ドメイン本体とそのサブドメインが対象になります。

`--domain "example.test"` の場合:

| 対象（ヘッダーが付与される） | 対象外（付与されない・HTTPSは素通し） |
| --- | --- |
| `example.test` | `evilexample.test` |
| `app.example.test` | `example.test.evil.com` |
| `api.example.test` | `example.com` |
| `foo.bar.example.test` | |

判定はホスト名を正規化（小文字化・ポート除去・空白除去）した上で、次の条件で行います。

```go
host == domain || strings.HasSuffix(host, "."+domain)
```

`strings.Contains` は使いません。`example.test.evil.com` のような別ドメインを誤って対象にしないためです。

## HTTP と HTTPS の扱い

- **HTTP**: 対象ドメインへのリクエストにヘッダーを追加します。
- **HTTPS**: 対象ドメインは MITM して TLS 内のリクエストにヘッダーを追加します。対象外は MITM せず CONNECT トンネルとして素通しします。
