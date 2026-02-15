const R2_BASE = import.meta.env.PUBLIC_R2_URL || '/assets';

export function assetUrl(path: string, filename: string) {
	return `${R2_BASE}/${path}/${filename}`;
}
