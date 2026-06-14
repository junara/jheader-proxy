# Changelog

All notable changes to this project are documented in this file.

このプロジェクトの主な変更点を記録します。フォーマットは [Keep a Changelog](https://keepachangelog.com/),バージョニングは [Semantic Versioning](https://semver.org/) に従います。

## [0.3.0] - 2026-06-14

### Features

- **gui**: Web GUIとCA配布ポータルをダークモードに対応
- **cli**: 引数なし実行で使い方とオプション一覧を表示（日本語）

### Documentation

- READMEのプロジェクト構成を現状に更新
- Web GUIの実画面スクリーンショットを掲載

## [0.2.0] - 2026-06-14

### Features

- CLI設定ファイル --config を追加（GUIのconfig.jsonと互換）
- 起動ログにCA証明書の有効期限と残り日数を表示

### Bug Fixes

- **cli**: --config のドメイン/許可リストも空要素を除去し挙動を統一

### Refactor

- RunConfig→usecase入力の変換を config.ToRunProxyInput に一元化

## [0.1.1] - 2026-06-14

### Features

- ローカルWeb GUI (--gui) とCA証明書ダウンロードポータルを追加

### Bug Fixes

- **gui**: ファイルの場所を開く操作をクロスプラットフォーム化

### Documentation

- Web GUI・CAダウンロードポータル・Android手順を追加

## [0.1.0] - 2026-06-12

### Features

- IPhone向けヘッダー付与プロキシ CLI を実装

### Documentation

- Astro Starlight 製マニュアルサイトを追加

### Build System

- GoReleaser による Homebrew リリースに対応


