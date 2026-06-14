// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// https://astro.build/config
export default defineConfig({
	site: 'https://junara.github.io',
	base: '/jheader-proxy',
	integrations: [
		starlight({
			title: 'jheader-proxy',
			social: [
				{ icon: 'github', label: 'GitHub', href: 'https://github.com/junara/jheader-proxy' },
			],
			// 単一言語（日本語）。英語を追加する場合は locales を ja/en に分け、
			// src/content/docs 配下を ja/ と en/ に分ける。
			locales: {
				root: { label: '日本語', lang: 'ja' },
			},
			sidebar: [
				{
					label: 'はじめに',
					items: [
						{ label: 'jheader-proxy とは', slug: 'getting-started/overview' },
						{ label: 'インストール', slug: 'getting-started/installation' },
						{ label: 'クイックスタート', slug: 'getting-started/quickstart' },
					],
				},
				{
					label: '使い方',
					items: [
						{ label: 'Web GUI', slug: 'usage/gui' },
						{ label: 'CA証明書とiPhone/Android設定', slug: 'usage/ca' },
						{ label: 'ヘッダー付与と対象ドメイン', slug: 'usage/headers' },
						{ label: '実行時オプション', slug: 'usage/runtime' },
					],
				},
				{
					label: 'リファレンス',
					items: [
						{ label: 'CLI引数', slug: 'reference/cli' },
						{ label: 'ログ', slug: 'reference/logs' },
					],
				},
				{
					label: 'ガイド',
					items: [
						{ label: 'セキュリティ', slug: 'guides/security' },
						{ label: 'トラブルシューティング', slug: 'guides/troubleshooting' },
					],
				},
			],
		}),
	],
});
