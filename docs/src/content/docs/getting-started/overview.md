---
title: jheader-proxy とは
description: ツールの目的と仕組みの概要
---

`jheader-proxy` は、macOS 上で動作する Go 製のローカル HTTP/HTTPS プロキシ CLI です。

iPhone の Wi-Fi プロキシ設定でこの CLI に通信を通し、指定したドメインへの HTTP/HTTPS リクエストだけに、CLI 引数で指定した HTTP ヘッダーを追加します。

主な用途は、iPhone Safari で開発・検証用 Web サイトにアクセスするときに、特定のリクエストヘッダー（PR 番号やデバッグ用フラグなど）を付与することです。

## 仕組み

```text
iPhone (Wi-Fi プロキシ設定)
   │  HTTP / HTTPS
   ▼
Mac: jheader-proxy (:8080)
   ├─ 対象ドメイン           → MITM して リクエストヘッダーを追加
   └─ それ以外               → CONNECT トンネルとして素通し（MITM しない）
```

- **対象ドメイン**（`--domain` 指定とそのサブドメイン）への通信のみ MITM し、ヘッダーを付与します。
- それ以外のドメインは MITM せず、通常の CONNECT トンネルとして中継します。

## 対象環境

- 開発言語: Go
- 実行環境: macOS
- 利用端末: iPhone（macOS と同じ Wi-Fi 上）
- iPhone 側で Wi-Fi の HTTP プロキシを手動設定する

## 想定しない用途

本番環境での利用、不特定多数が使う公開プロキシ、GUI アプリ化などは対象外です。あくまで開発・検証用のローカルツールです。
