# Changelog

All notable changes to this project are documented in this file.

このプロジェクトの主な変更点を記録します。フォーマットは [Keep a Changelog](https://keepachangelog.com/),バージョニングは [Semantic Versioning](https://semver.org/) に従います。

## [0.3.2] - 2026-07-25

### Features

- **cli**: サブコマンド run / gen-ca / gui / version を導入する
- **cli**: --header-file で秘匿ヘッダーをファイルから渡せるようにする
- 起動時エラーに原因と次の一手のヒントを添える
- **cli**: Quiet と verbose の同時指定をエラーにする
- **plugin**: このリポジトリを Claude Code プラグインマーケットプレイスにする

### Bug Fixes

- **cli**: 明示的な --help を stdout へ出力する
- **cli**: レビュー指摘の修正（設定とフラグの quiet/verbose 干渉ほか）

### Documentation

- **cli**: ヘルプにマニュアルと不具合報告先のリンクを載せる
- マニュアルサイトをサブコマンド形式に更新する
- **skill**: Jheader-proxy 利用ガイドのスキルを追加する

## [0.3.1] - 2026-07-24

### Features

- 起動中のバージョンをログとWeb管理画面に表示する

### Performance

- **proxy**: MITM証明書をキャッシュして並行アクセスを捌けるようにする

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


