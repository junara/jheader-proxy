---
title: CA証明書とiPhone/Android設定
description: 自分専用CAの生成と、iPhone・Androidへのインストール・信頼設定
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
- **方法D: プロキシ経由でダウンロード（モバイル向け・おすすめ）** — 後述

:::note
拡張子は `.pem` のままで iOS は認識します。プロファイルとして開けない場合は `jheader-proxy-ca-cert.crt` にリネームすると確実です（中身は同じ）。
:::

## モバイルから直接ダウンロード（プロキシ経由）

端末を本機のプロキシに設定した状態で、ブラウザで **`http://jheader.proxy`** を開くと、CA 証明書をダウンロードできる案内ページが表示されます（mitmproxy の `mitm.it` と同じ仕組み）。AirDrop やファイル転送が不要で、iPhone・Android の両方で使えます。

1. 端末の Wi-Fi プロキシに本機（`MacのIP`:プロキシポート）を設定する
2. ブラウザのアドレス欄に **`http://jheader.proxy`** と入力して開く（必ず `http://` を付ける）
3. 「CA 証明書をダウンロード」をタップし、[インストールと信頼の有効化](#インストールと信頼の有効化2段階)（iPhone）／[Androidの場合](#androidの場合) に従う

:::note
配布されるのは証明書のみで、秘密鍵は配信されません。`https://` ではなく **`http://`** で開いてください（証明書をまだ信頼していないため HTTPS では開けません）。ブラウザが検索に変換してしまう場合は、フルで `http://jheader.proxy/` と入力してください。
:::

## インストールと信頼の有効化（2段階）

ダウンロードしただけでは有効になりません。**2段階**必要です。

1. **プロファイルのインストール**: 設定 → 一般 → VPNとデバイス管理 → ダウンロード済みプロファイル → インストール
2. **信頼の有効化（必須）**: 設定 → 一般 → 情報 → 証明書信頼設定 → 「jheader-proxy local CA」のスイッチを **ON**

:::danger[手順2を忘れると]
手順1だけで手順2（証明書信頼設定）を忘れると、`tls: unknown certificate` エラーで接続できません。[トラブルシューティング](/jheader-proxy/guides/troubleshooting/)を参照。
:::

## Androidの場合

Android は iOS と手順・前提が異なります。証明書（`jheader-proxy-ca-cert.pem`）を端末に転送してから、以下を行います。

### インストール（スロットの選択に注意）

設定 → セキュリティ → 暗号化と認証情報 → 証明書をインストール → **「CA証明書」**

:::caution[必ず「CA証明書」を選ぶ]
「VPNとアプリのユーザー証明書」スロットではなく、**「CA証明書」**を選んでください。間違えると MITM 対象ドメインで証明書エラーになり接続できません。インストール時に「ネットワークが監視される可能性があります」と警告が出ますが、想定どおりなので進めて構いません。
:::

:::note
拡張子 `.pem` が認識されない場合は `jheader-proxy-ca-cert.crt` にリネームすると確実です（中身は同じ）。送るのは証明書のみで、秘密鍵は絶対に送らないでください。
:::

### ブラウザ・アプリによる制約

Android 7 以降、ユーザーがインストールした CA の扱いがアプリごとに異なります。

| 利用先 | ユーザーCAの信頼 | 備考 |
| --- | --- | --- |
| Chrome などのブラウザ閲覧 | される | この用途は問題なく MITM 可能 |
| Firefox | 独自のCAストアを使う | システムに入れても効かない。Firefox の設定から別途インポートが必要 |
| 一般アプリ（非ブラウザ） | されない | Android 7 以降は無視されるため、原理的に MITM 不可（アプリ側の `network_security_config` が必要で第三者アプリでは不可） |

### つながらないときの切り分け

- **対象外ドメイン**（`--domain` に指定していない HTTPS サイト）は MITM しないため、CA 信頼に関係なく開けます。これが開けるなら到達は正常です。
- **対象ドメイン**だけが証明書エラー（`NET::ERR_CERT_AUTHORITY_INVALID` など）になる場合は、CA が「CA証明書」として正しく信頼されていません。インストールスロットを再確認してください。
