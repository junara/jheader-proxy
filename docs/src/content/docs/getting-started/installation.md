---
title: インストール
description: jheader-proxy のビルド・インストール方法
---

## 必要条件

- macOS（iPhone と同じ Wi-Fi ネットワーク）
- ソースからビルドする場合は Go 1.26 以降

## Homebrew（おすすめ）

[Homebrew](https://brew.sh/)（macOS / Linux）が入っていれば、1コマンドでインストールできます。

```bash
brew install junara/tap/jheader-proxy
```

`junara/tap/jheader-proxy` は「`junara` の `homebrew-tap` リポジトリにある `jheader-proxy`」という意味で、tap の追加とインストールを同時に行います。tap を明示的に追加してからインストールすることもできます。

```bash
brew tap junara/tap
brew install jheader-proxy
```

インストールの確認:

```bash
jheader-proxy --version
```

### 更新・アンインストール

```bash
brew upgrade jheader-proxy      # 最新版へ更新
brew uninstall jheader-proxy    # アンインストール
brew untap junara/tap           # tap も削除する場合
```

:::note
配布バイナリは未署名です。Homebrew cask 経由では `com.apple.quarantine` 属性が自動で外れるため、そのまま実行できます（手動でバイナリを置いた場合に Gatekeeper でブロックされたら `xattr -dr com.apple.quarantine ./jheader-proxy` を実行してください）。
:::

## ソースからビルド

```bash
git clone https://github.com/junara/jheader-proxy.git
cd jheader-proxy
go build -o jheader-proxy ./cmd/jheader-proxy
```

`go run` で直接実行することもできます。

```bash
go run ./cmd/jheader-proxy --version
```

## go install

```bash
go install github.com/junara/jheader-proxy/cmd/jheader-proxy@latest
```

## バージョンの埋め込み

リリースビルドでバージョンを埋め込むには `-ldflags` を使います。

```bash
go build -ldflags "-X main.version=v1.0.0" -o jheader-proxy ./cmd/jheader-proxy
./jheader-proxy --version   # jheader-proxy v1.0.0
```
