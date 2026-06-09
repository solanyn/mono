// @ts-check
import mdx from '@astrojs/mdx';
import sitemap from '@astrojs/sitemap';
import mermaid from 'beautiful-mermaid';
import { defineConfig } from 'astro/config';

export default defineConfig({
	site: 'https://blog.goyangi.io',
	integrations: [mdx(), sitemap()],
	markdown: {
		remarkPlugins: [mermaid],
	},
	vite: {
		server: {
			fs: {
				strict: import.meta.env.PROD,
			},
		},
	},
});
