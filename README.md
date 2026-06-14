# jheader-proxy

[![CI](https://github.com/junara/jheader-proxy/actions/workflows/ci.yml/badge.svg)](https://github.com/junara/jheader-proxy/actions/workflows/ci.yml)

macOS上で動作するGo製のローカルHTTP/HTTPSプロキシCLI。

iPhoneのWi-Fiプロキシ設定でこのCLIに通信を通し、指定したドメインへのHTTP/HTTPSリクエストだけに、CLI引数で指定したHTTPヘッダーを追加します。

主な用途は、iPhone Safariで開発・検証用Webサイトにアクセスするときに、特定のリクエストヘッダーを付与することです。

詳しいマニュアルは [https://junara.github.io/jheader-proxy/](https://junara.github.io/jheader-proxy/) を参照してください。

## インストール

### Homebrew（おすすめ）

[Homebrew](https://brew.sh/)（macOS / Linux）で1コマンドでインストールできます。

```bash
brew install junara/tap/jheader-proxy
```

tap を明示的に追加する場合:

```bash
brew tap junara/tap
brew install jheader-proxy
```

更新・アンインストール:

```bash
brew upgrade jheader-proxy
brew uninstall jheader-proxy
```

### ソースからビルド

```bash
go build -o jheader-proxy ./cmd/jheader-proxy
# または
go install github.com/junara/jheader-proxy/cmd/jheader-proxy@latest
```

## 事前準備: CA証明書の生成（必須）

HTTPS通信にヘッダーを追加するには、対象ドメインの通信をMITM（TLSの復号・再暗号化）する必要があり、そのためのCA証明書・秘密鍵が必須です。組み込みのCAは使いません（秘密鍵が公開されており危険なため）。**必ず自分専用のCAを生成してください。**

```bash
go build -o jheader-proxy ./cmd/jheader-proxy

# 自分専用のCAを生成する（秘密鍵はこのMacにしか存在しない）
./jheader-proxy --gen-ca --ca-cert jheader-proxy-ca-cert.pem --ca-key jheader-proxy-ca-key.pem
```

- `jheader-proxy-ca-cert.pem` … iPhoneにインストールするCA証明書
- `jheader-proxy-ca-key.pem` … CAの秘密鍵。**絶対にGit管理・共有しないこと**（`.gitignore` 済み）

既存ファイルがある場合、誤って上書きしないよう生成は失敗します。

## 起動方法

CA生成後、`--ca-cert` と `--ca-key` を指定して起動します（どちらも必須）。

```bash
go run ./cmd/jheader-proxy \
  --listen ":8080" \
  --domain "example.test" \
  --header "X-Debug-User=jun" \
  --ca-cert jheader-proxy-ca-cert.pem \
  --ca-key jheader-proxy-ca-key.pem
```

ビルド後:

```bash
./jheader-proxy \
  --listen ":8080" \
  --domain "example.test" \
  --header "X-Debug-User=jun" \
  --ca-cert jheader-proxy-ca-cert.pem \
  --ca-key jheader-proxy-ca-key.pem
```

## GUI（ローカルWeb管理画面）

CLI引数を手で組み立てる代わりに、ブラウザから設定・起動/停止・CA生成・ログ閲覧ができます。

```bash
./jheader-proxy --gui
```

- 起動すると `http://127.0.0.1:9090` で管理画面が立ち上がり、ブラウザが自動で開きます。
- プロキシ本体（`:8080` 等）は管理画面の「開始」ボタンで起動し、「停止」で止まります。ブラウザを閉じてもプロキシは動き続けます。
- ターミナルで `Ctrl+C` すると管理画面とプロキシの両方が穏当に停止します。
- 設定（対象ドメイン・ヘッダー・CAパス等）は**直近の内容が自動保存**され、次回起動時にフォームへ復元されます（`~/Library/Application Support/jheader-proxy/config.json`、パーミッション `0600`）。
- CA証明書はGUIの「CA生成」ボタンで作成でき、「ファイルの場所を開く」からAirDropでiPhoneへ送れます。iPhoneのWi-Fiプロキシに設定するサーバ（LAN IP）とポートも画面に表示されます。

管理画面は `127.0.0.1` 限定で待ち受け、起動毎のランダムトークンでAPIを保護します（LANやiPhoneからは触れません）。

| GUI用の引数 | 説明 |
| --- | --- |
| `--gui` | ローカルWeb管理画面を起動する |
| `--gui-listen` | 管理画面の待受アドレス。デフォルトは `127.0.0.1:9090`。任意に変更可。指定ポートが使用中ならフォールバックせずエラーになる |
| `--no-open` | `--gui` 時にブラウザを自動起動しない |

## 設定ファイル（`--config`）

毎回長い引数を打つ代わりに、設定を JSON 1枚にまとめて `--config` で読み込めます。

```bash
./jheader-proxy --config jheader-proxy.json
```

設定ファイルの例（全項目は任意。省略した項目は既定値になります）:

```json
{
  "listen": ":8080",
  "domains": ["example.test", "api.example.test"],
  "headers": [
    { "name": "X-Debug-User", "value": "jun" }
  ],
  "allow": ["192.168.1.5"],
  "duration": "30m",
  "quiet": false,
  "verbose": false,
  "redact": false,
  "caCertPath": "jheader-proxy-ca-cert.pem",
  "caKeyPath": "jheader-proxy-ca-key.pem"
}
```

- **優先順位は「コマンドライン引数 > 設定ファイル > 既定値」**。同じ項目をコマンドラインでも指定すると、そちらが勝ちます。

  ```bash
  # 設定ファイルの listen を無視して :9090 で起動する
  ./jheader-proxy --config jheader-proxy.json --listen :9090
  ```

- `--domain` / `--header` / `--allow` は、コマンドラインで1つでも指定すると設定ファイルのリストを**置き換え**ます（マージはしません）。
- この形式は **GUI（`--gui`）が保存する `config.json` と互換**です。`--config "$HOME/Library/Application Support/jheader-proxy/config.json"` のように、GUIで作った設定をCLIでそのまま使えます。
- ヘッダー値に認証トークン等が入りうるため、設定ファイルのパーミッションは絞ること（例: `chmod 600`）。

## CLI引数

| 引数 | 説明 |
| --- | --- |
| `--config` | 設定をまとめた JSON ファイルのパス。GUIの `config.json` と互換。コマンドライン引数が設定ファイルより優先される |
| `--listen` | プロキシの待ち受けアドレス。デフォルトは `:8080` |
| `--domain` | ヘッダー追加対象のドメイン。複数回指定可能。サブドメインも対象になる。1つ以上必須 |
| `--header` | 追加するヘッダーを `Name=Value` 形式で指定。複数回指定可能。1つ以上必須 |
| `--ca-cert` | HTTPS MITMに使うCA証明書PEMのパス。必須 |
| `--ca-key` | HTTPS MITMに使うCA秘密鍵PEMのパス。必須 |
| `--duration` | この時間が過ぎると自動停止する。デフォルト `10m`。`0` で無制限 |
| `--allow` | 接続を許可するクライアントの IP または CIDR。複数回指定可能。未指定なら全許可 |
| `--redact` | 起動ログで全ヘッダー値をマスクする |
| `--quiet` | リクエストごとのログ（`[MITM]`/`[TUNNEL]`/`[ADD HEADER]`）を抑制する |
| `--verbose` | 対象ドメインのレスポンスもログ出力する（`[RESP]`） |
| `--gen-ca` | `--ca-cert`/`--ca-key` のパスに新しいCAを生成して終了する |
| `--force` | `--gen-ca` 時に既存ファイルを上書きする |
| `--version` | バージョンを表示して終了する |

> `--allow` を指定すると、許可リストにないクライアントの接続は受理時に拒否し `[DENY] <IP>` をログ出力します。共有Wi-Fiでは、iPhoneのIPを `--allow` で限定しておくと、第三者にプロキシを使われる事故を防げます。
>
> `Authorization` / `Cookie` / `Set-Cookie` / `X-Api-Key` / `Proxy-Authorization` は、`--redact` を付けなくても起動ログでは値が `***` にマスクされます。
>
> 停止忘れ防止のため、デフォルトで起動から **10分** 経過すると自動停止します（`auto-stop after 10m0s` をログ表示）。時間を変えるには `--duration 30m` のように指定し、無制限にするには `--duration 0` を指定します。
>
> `Ctrl+C`（SIGINT/SIGTERM）でも穏当に停止します（`shutting down...` を出して終了）。

## MacのIPアドレス確認方法

`ipconfig getifaddr en0` は、Wi-Fiが `en0` でない機種（有線アダプタ利用時など）や `en0` にIPが割り当たっていない場合に何も出力しません。インターフェース名に依存しない方法を使います。

おすすめ（現在の通信に使われているインターフェースのIPを取得）:

```bash
ipconfig getifaddr "$(route -n get default | awk '/interface:/{print $2}')"
```

うまくいかない場合（VPN接続中など）は、`127.0.0.1` 以外のIPv4アドレスを一覧表示し、iPhoneと同じWi-Fiの private アドレス（`192.168.x.x` / `10.x.x.x` / `172.16〜31.x.x`）を選びます。

```bash
ifconfig | awk '/inet / && $2 !~ /^127\./ {print $2}'
```

## iPhoneの設定方法

```text
設定
→ Wi-Fi
→ 接続中のWi-Fiの詳細
→ プロキシを構成
→ 手動
→ サーバ: MacのIPアドレス
→ ポート: 8080
```

## 対象ドメイン例

`--domain "example.test"` の場合:

対象（ヘッダーが追加される）:

```text
https://example.test/
https://app.example.test/
https://api.example.test/
```

対象外（ヘッダーは追加されず、HTTPSはMITMせずに素通し）:

```text
https://example.test.evil.com/
https://evilexample.test/
https://example.com/
```

## HTTPS利用時の注意

HTTPS通信にヘッダーを追加するには、`--gen-ca` で生成した**自分専用のCA証明書（`jheader-proxy-ca-cert.pem`）**をiPhoneにインストールし、信頼設定を有効化する必要があります。組み込みCAは使わないため、秘密鍵は生成したMac上にしか存在しません。

iPhoneに送るのは**証明書（`jheader-proxy-ca-cert.pem`）だけ**です。秘密鍵（`jheader-proxy-ca-key.pem`）は絶対に送らないでください。

#### 手順1: 証明書をiPhoneに送る

以下のいずれかの方法で送ります（プロキシ設定をする**前**に行うこと）。

**方法A: AirDrop（おすすめ・最も簡単）**

Mac上で `jheader-proxy-ca-cert.pem` を右クリック → 共有 → AirDrop → 自分のiPhoneを選択。

**方法B: ローカルHTTPサーバ経由**

証明書のあるフォルダで簡易サーバを立て、iPhoneのSafariでアクセスしてダウンロードします。

```bash
cd <証明書のあるフォルダ>
ipconfig getifaddr "$(route -n get default | awk '/interface:/{print $2}')"   # MacのIPアドレスを確認（例: 192.168.1.23）
python3 -m http.server 8000
```

iPhoneのSafariで `http://<MacのIP>:8000/jheader-proxy-ca-cert.pem` を開く。終わったら `Ctrl+C` でサーバを停止する。

**方法C: メール / メモ / ファイルApp**

`jheader-proxy-ca-cert.pem` を自分宛にメール添付、またはiCloud経由で渡し、iPhoneで開く。

> 拡張子は `.pem` のままでiOSは認識します。プロファイルとして開けない場合は `jheader-proxy-ca-cert.crt` にリネームして送ると確実です（中身は同じでOK）。

#### 手順2: インストールと信頼の有効化

ダウンロードしただけでは有効になりません。2段階必要です。

1. **プロファイルのインストール**: 設定 → 一般 → VPNとデバイス管理 → ダウンロード済みプロファイル → インストール
2. **信頼の有効化（必須）**: 設定 → 一般 → 情報 → 証明書信頼設定 → 「jheader-proxy local CA」のスイッチをON

> 手順1だけで手順2を忘れると、後述の `tls: unknown certificate` エラーで接続できません。

### セキュリティ上の注意

- **検証が終わったらiPhoneのWi-Fiプロキシ設定をOFFにすること**
- **検証が終わったらCA証明書の信頼設定をOFFにすること**
- **不要になったCA証明書は削除すること**
- **CAの秘密鍵（`jheader-proxy-ca-key.pem`）はGit管理・共有しないこと**（`.gitignore` 済み）
- CAを信頼している端末は、その秘密鍵を持つ者に全HTTPS通信をMITMされ得ます。信頼できないネットワーク（公衆Wi-Fiなど）に接続する前に必ず信頼をOFFにしてください
- このツールはデフォルトで追加するヘッダーの値をログに出力します。`Authorization` / `Cookie` / `X-Api-Key` などの機密情報になり得るヘッダーを扱う場合は、ログの取り扱いに注意してください
- 本番環境や不特定多数が使う公開プロキシとしての利用は想定していません

## トラブルシューティング

### プロキシのログに `tls: unknown certificate` が出てページが開けない

例:

```text
[MITM] app.example.test:443
[002] WARN: Cannot handshake client app.example.test:443 remote error: tls: unknown certificate
```

`remote error:` は **iPhone側が証明書を拒否している**という意味です。MITM自体は動いていますが、署名元のCAをiPhoneが信頼していないため弾かれています。ほぼ確実に **証明書信頼設定（手順2）がOFF** です。

1. 設定 → 一般 → 情報 → 証明書信頼設定 → 「jheader-proxy local CA」を **ON** にする
2. Safariのタブを一度閉じて（アプリスイッチャーから上スワイプで終了）再アクセスする

それでも直らない場合:

- iPhoneに入れた証明書が、いま起動中のプロキシが使う `jheader-proxy-ca-cert.pem` と同一か確認する（CAを作り直した場合は入れ直す）。Mac側で確認: `openssl x509 -in jheader-proxy-ca-cert.pem -noout -fingerprint -sha256`
- iPhoneがロックダウンモードだと、ユーザー追加CAでのMITMはブロックされる
- 会社管理端末（MDM）では、ユーザーCAの信頼が制限されていることがある

### 対象サイトにアクセスしてもヘッダーが付かない（ログに `[MITM]` が出ない）

- `--domain` の指定とアクセス先ドメインが一致しているか確認する（親ドメイン指定でサブドメインは対象だが、別ドメインは対象外）
- iPhoneのWi-Fiプロキシ設定が正しいMacのIP・ポートを向いているか確認する

### `ipconfig getifaddr en0` が何も出力しない

Wi-Fiが `en0` でない、または `en0` にIPが無い場合に起こります。[MacのIPアドレス確認方法](#macのipアドレス確認方法) のインターフェース名に依存しないコマンドを使ってください。

## 動作確認方法

1. ローカルで起動する（事前に `--gen-ca` でCAを生成しておく）

   ```bash
   go run ./cmd/jheader-proxy \
     --listen ":8080" \
     --domain "example.test" \
     --header "X-Debug-User=jun" \
     --ca-cert jheader-proxy-ca-cert.pem \
     --ca-key jheader-proxy-ca-key.pem
   ```

2. iPhoneのWi-Fiプロキシに、MacのIPアドレスとポート `8080` を設定する

3. iPhone Safariで対象サイトにアクセスする

   ```text
   https://app.example.test/
   ```

4. アプリケーション側でヘッダーが届いていることを確認する

   Railsの場合:

   ```ruby
   request.headers["X-Debug-User"]
   ```

## ログ

起動時:

```text
proxy listening on :8080
target domains: example.test
CA certificate: jheader-proxy-ca-cert.pem
CA expires: 2036-06-14 (in 3652 days)
headers:
  X-Debug-User: jun
```

`--allow` 指定時は `allowed clients: ...` 行が、機密ヘッダーや `--redact` 指定時は値が `***` で出ます。

CA証明書の有効期限と残り日数を `CA expires: ...` 行で表示します。残り14日以内になると `WARNING: CA certificate expires soon: ...`、期限切れなら `WARNING: CA certificate expired on ...` を出すので、検証端末でCAを更新し忘れて MITM が突然失敗するのを未然に防げます（更新は `--gen-ca --force` で再生成し、iPhoneへ入れ直してください）。

リクエストごと:

```text
[MITM] app.example.test:443
[ADD HEADER] GET https://app.example.test/
[TUNNEL] www.google.com:443
```

## テスト

```bash
go test ./...
```

静的解析（[golangci-lint](https://golangci-lint.run/)）:

```bash
golangci-lint run ./...
```

バージョンを埋め込んでビルドする場合:

```bash
go build -ldflags "-X main.version=v1.0.0" -o jheader-proxy ./cmd/jheader-proxy
./jheader-proxy --version   # jheader-proxy v1.0.0
```

## プロジェクト構成

クリーンアーキテクチャに沿って、依存方向が内側（`domain`）に向くようレイヤを分離しています。

```text
.
├── cmd/
│   └── jheader-proxy/
│       └── main.go              # 合成ルート（依存を組み立てて実行）
├── internal/
│   ├── domain/                  # エンティティ／値オブジェクト（標準ライブラリのみ依存）
│   │   ├── matcher.go           #   対象ドメイン判定
│   │   └── header.go            #   追加ヘッダーの解析・保持
│   ├── usecase/                 # アプリケーションのユースケースとポート（interface）
│   │   ├── ports.go             #   CAProvider / CAGenerator / ProxyServer / Logger
│   │   ├── run_proxy.go         #   プロキシ起動ユースケース
│   │   └── generate_ca.go       #   CA生成ユースケース
│   ├── adapter/
│   │   └── cli/                 # インターフェースアダプタ（フラグ解析）
│   │       └── cli.go
│   └── infra/                   # フレームワーク／ドライバ（具体実装）
│       ├── ca/                  #   crypto/x509 + ファイルシステムによるCA実装
│       │   └── ca.go
│       └── proxy/               #   goproxy によるプロキシ実装
│           └── goproxy.go
├── go.mod
├── go.sum
├── LICENSE
├── README.md
└── .gitignore
```

依存の向き: `infra` / `adapter` → `usecase` → `domain`。`usecase` がポート（interface）を定義し、`infra` がそれを実装します。`cmd/jheader-proxy/main.go` が両者を結線します。
