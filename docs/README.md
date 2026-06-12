# jheader-proxy ドキュメントサイト

[Astro Starlight](https://starlight.astro.build/) で構築したマニュアルサイトです。`main` の `docs/**` を更新すると GitHub Actions（`.github/workflows/docs.yml`）が GitHub Pages へ自動デプロイします。

公開URL: `https://junara.github.io/jheader-proxy/`

## ローカルで動かす

```bash
cd docs
npm install
npm run dev      # http://localhost:4321/jheader-proxy/
```

## ビルド

```bash
npm run build    # dist/ に出力
npm run preview  # ビルド結果をプレビュー
```

## 構成

- `astro.config.mjs` … サイト設定（`site` / `base` / サイドバー）
- `src/content/docs/**` … ページ本体（Markdown / MDX）
- `src/content.config.ts` … Starlight のコンテンツコレクション定義
- `public/` … 静的アセット（favicon など）

## GitHub Pages の有効化（初回のみ）

リポジトリの Settings → Pages → Build and deployment → Source を **GitHub Actions** に設定してください。

## 英語版を追加する場合

`astro.config.mjs` の `locales` を `ja` / `en` に分け、`src/content/docs/` 配下を `ja/` と `en/` に分けて同じページを用意します（[encfixture](https://github.com/junara/encfixture) と同じ構成）。
