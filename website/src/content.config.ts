import { defineCollection } from 'astro:content';
import { glob } from 'astro/loaders';
import { z } from 'astro/zod';

const blog = defineCollection({
	loader: glob({ base: './src/content/blog', pattern: '**/*.{md,mdx}' }),
	schema: z.object({
		title: z.string(),
		date: z.coerce.date(),
		tags: z.array(z.string()).optional(),
		description: z.string().optional(),
	}),
});

const ratingSchema = z.object({
	flavor: z.number().min(1).max(5),
	effort: z.number().min(1).max(5),
	accessibility: z.number().min(1).max(5),
	overall: z.number().min(1).max(5),
});

const recipes = defineCollection({
	loader: glob({ base: './src/content/recipes', pattern: '**/*.{md,mdx}' }),
	schema: ({ image }) =>
		z.object({
			title: z.string(),
			date: z.coerce.date(),
			type: z.enum(['adaptation', 'original', 'video']),
			sourceUrl: z.string().url().optional(),
			sourceAuthor: z.string().optional(),
			sourcePlatform: z.enum(['youtube', 'tiktok', 'website', 'book']).optional(),
			baseServings: z.number().optional(),
			myServings: z.number().optional(),
			tags: z.array(z.string()).optional(),
			image: image().optional(),
			processImage: image().optional(),
			rating: ratingSchema,
			description: z.string().optional(),
		}),
});

const reviews = defineCollection({
	loader: glob({ base: './src/content/reviews', pattern: '**/*.{md,mdx}' }),
	schema: ({ image }) =>
		z.object({
			title: z.string(),
			date: z.coerce.date(),
			restaurant: z.string(),
			location: z.string().optional(),
			tags: z.array(z.string()).optional(),
			image: image().optional(),
			rating: ratingSchema,
			description: z.string().optional(),
		}),
});

const gear = defineCollection({
	loader: glob({ base: './src/content/gear', pattern: '**/*.{md,mdx}' }),
	schema: ({ image }) =>
		z.object({
			title: z.string(),
			date: z.coerce.date(),
			type: z.enum(['equipment', 'ingredient']),
			tags: z.array(z.string()).optional(),
			image: image().optional(),
			description: z.string().optional(),
		}),
});

export const collections = { blog, recipes, reviews, gear };
