export const onRequest: PagesFunction = async ({ request }) => {
	const url = new URL(request.url);
	const period = url.searchParams.get('period');
	const key = period
		? `vgc/reports/meta-report-${period}.html`
		: 'vgc/reports/meta-report-latest.html';
	const r2Url = `https://assets.goyangi.io/${key}`;

	const response = await fetch(r2Url);

	if (!response.ok) {
		return new Response('<h1>Report not yet available</h1><p>The first weekly report will be published after the pipeline runs.</p>', {
			status: 404,
			headers: { 'Content-Type': 'text/html; charset=utf-8' },
		});
	}

	const html = await response.text();
	return new Response(html, {
		headers: {
			'Content-Type': 'text/html; charset=utf-8',
			'Cache-Control': 'public, max-age=3600',
		},
	});
};
