const R2_BASE = import.meta.env.PUBLIC_R2_URL || '/content-images';

export function contentImage(collection: string, slug: string, filename: string) {
	return `${R2_BASE}/${collection}/${slug}/${filename}`;
}
