---
title: クイックスタート
description: CA生成から iPhone での確認までの最短手順
---

import { Steps } from '@astrojs/starlight/components';

:::tip[ブラウザから操作したい場合]
`./jheader-proxy --gui` で管理画面が開き、CA生成・設定・起動/停止・ログ閲覧をブラウザから行えます。詳しくは [Web GUI](/jheader-proxy/usage/gui/) を参照してください。以下は CLI での手順です。
:::

<Steps>

1. **自分専用CAを生成する**

   HTTPS にヘッダーを付与するには CA が必須です。組み込みCAは使わず、必ず自分で生成します。

   ```bash
   ./jheader-proxy --gen-ca \
     --ca-cert jheader-proxy-ca-cert.pem \
     --ca-key jheader-proxy-ca-key.pem
   ```

2. **プロキシを起動する**

   ```bash
   ./jheader-proxy \
     --listen ":8080" \
     --domain "example.test" \
     --header "X-Debug-User=jun" \
     --ca-cert jheader-proxy-ca-cert.pem \
     --ca-key jheader-proxy-ca-key.pem
   ```

3. **MacのIPアドレスを確認する**

   ```bash
   ipconfig getifaddr "$(route -n get default | awk '/interface:/{print $2}')"
   ```

4. **iPhone の Wi-Fi プロキシを設定する**

   設定 → Wi-Fi → 接続中のWi-Fiの詳細 → プロキシを構成 → 手動 → サーバに MacのIP、ポートに `8080`。

5. **CA証明書を iPhone にインストールして信頼する**

   `jheader-proxy-ca-cert.pem` を iPhone に送り、プロファイルをインストール後、**証明書信頼設定をON**にします（[詳細](/jheader-proxy/usage/ca/)）。

6. **iPhone Safari で対象サイトにアクセスする**

   `https://app.example.test/` にアクセスすると、リクエストに `X-Debug-User: jun` が付与されて届きます。

</Steps>

:::caution[検証が終わったら]
iPhone の Wi-Fi プロキシ設定と証明書信頼設定を **OFF** に戻してください。詳しくは[セキュリティ](/jheader-proxy/guides/security/)を参照。なお、既定では起動から **10分** で自動停止します。
:::
