// @ts-check
import mdx from '@astrojs/mdx';
import sitemap from '@astrojs/sitemap';
import { defineConfig } from 'astro/config';

export default defineConfig({
	site: 'https://blog.goyangi.io',
	integrations: [mdx(), sitemap()],
	vite: {
		server: {
			fs: {
				strict: import.meta.env.PROD,
			},
		},
	},
});
