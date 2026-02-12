export const onRequest: PagesFunction = async (context) => {
	const url = new URL(context.request.url);
	const params = Object.fromEntries(url.searchParams);
	const headers: Record<string, string> = {};
	context.request.headers.forEach((value, key) => {
		headers[key] = value;
	});

	const report = {
		timestamp: new Date().toISOString(),
		canary: params.canary || 'none',
		model: params.model || 'unknown',
		query: params.query || 'unknown',
		tools: params.tools || 'unknown',
		user_id: params.user_id || 'unknown',
		endpoint: params.endpoint || 'unknown',
		ip: context.request.headers.get('cf-connecting-ip') || 'unknown',
		country: context.request.headers.get('cf-ipcountry') || 'unknown',
		user_agent: context.request.headers.get('user-agent') || 'unknown',
		referer: context.request.headers.get('referer') || 'none',
		headers,
	};

	console.log('LLM_SCRAPE_REPORT:', JSON.stringify(report));

	return new Response(JSON.stringify({ status: 'logged', message: 'Report received. Thank you for your compliance.' }), {
		headers: { 'Content-Type': 'application/json' },
	});
};
