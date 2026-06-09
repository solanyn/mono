// @ts-check
import mdx from '@astrojs/mdx';
import sitemap from '@astrojs/sitemap';
import { parseMermaid } from 'beautiful-mermaid';
import { defineConfig } from 'astro/config';

export default defineConfig({
	site: 'https://blog.goyangi.io',
	integrations: [mdx(), sitemap()],
	markdown: {
		remarkPlugins: [parseMermaid],
	},
	vite: {
		server: {
			fs: {
				strict: import.meta.env.PROD,
			},
		},
	},
});
