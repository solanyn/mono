<script>
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import { marked } from 'marked';

	let summary = null;
	let loading = true;
	let error = null;

	$: summaryId = $page.params.id;

	onMount(async () => {
		try {
			// TODO: Replace with actual API endpoint
			const response = await fetch(`/api/summaries/${summaryId}`);
			if (response.ok) {
				summary = await response.json();
			} else {
				// Fallback to mock data for development
				summary = {
					id: parseInt(summaryId),
					title: 'Sample Summary Title',
					date: '2025-06-18',
					content: `# Executive Summary

This is a sample summary with **markdown content**. In a real implementation, this would come from your backend API.

## Key Points

- Important development in technology sector
- Market analysis shows positive trends
- New product launches expected next quarter

## Detailed Analysis

Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.

### Technical Developments

- **AI Integration**: New machine learning capabilities
- **Performance**: 50% improvement in processing speed  
- **Security**: Enhanced encryption protocols

## Conclusion

The overall outlook remains positive with significant opportunities for growth in the coming months.`
				};
			}
		} catch (e) {
			error = e.message;
		} finally {
			loading = false;
		}
	});

	function formatDate(dateString) {
		const date = new Date(dateString);
		return date.toLocaleDateString('en-US', {
			year: 'numeric',
			month: 'long',
			day: 'numeric'
		});
	}
</script>

<svelte:head>
	<title>{summary?.title || 'Loading...'} - tldr</title>
</svelte:head>

<div class="summary">
	{#if loading}
		<div class="loading">Loading summary...</div>
	{:else if error}
		<div class="error">Error loading summary: {error}</div>
	{:else if summary}
		<article>
			<header class="summary-header">
				<h1>{summary.title}</h1>
				<div class="summary-meta">
					<time datetime={summary.date}>{formatDate(summary.date)}</time>
				</div>
			</header>
			
			<div class="summary-content">
				{@html marked(summary.content)}
			</div>
		</article>

		<nav class="summary-nav">
			<a href="/" class="back-link">← Back to all summaries</a>
		</nav>
	{:else}
		<div class="error">Summary not found</div>
	{/if}
</div>

<style>
	.summary {
		max-width: 700px;
		margin: 0 auto;
	}

	.loading, .error {
		text-align: center;
		padding: 2rem;
		color: #6c757d;
		font-style: italic;
	}

	.error {
		color: #dc3545;
	}

	.summary-header {
		margin-bottom: 2rem;
		text-align: center;
		border-bottom: 1px solid #e9ecef;
		padding-bottom: 1rem;
	}

	.summary-header h1 {
		margin-bottom: 0.5rem;
		line-height: 1.2;
	}

	.summary-meta {
		color: #6c757d;
		font-size: 0.875rem;
	}

	.summary-content {
		text-align: left;
		line-height: 1.6;
		margin-bottom: 3rem;
	}

	.summary-content :global(h1),
	.summary-content :global(h2),
	.summary-content :global(h3) {
		margin-top: 2rem;
		margin-bottom: 1rem;
		color: #495057;
	}

	.summary-content :global(h1) {
		font-size: 1.5rem;
	}

	.summary-content :global(h2) {
		font-size: 1.25rem;
	}

	.summary-content :global(h3) {
		font-size: 1.125rem;
	}

	.summary-content :global(p) {
		margin-bottom: 1rem;
	}

	.summary-content :global(ul),
	.summary-content :global(ol) {
		margin-bottom: 1rem;
		padding-left: 1.5rem;
	}

	.summary-content :global(li) {
		margin-bottom: 0.5rem;
	}

	.summary-content :global(strong) {
		font-weight: 600;
	}

	.summary-content :global(code) {
		background: #f8f9fa;
		padding: 0.125rem 0.25rem;
		border-radius: 0.25rem;
		font-family: 'Courier New', monospace;
		font-size: 0.875rem;
	}

	.summary-nav {
		text-align: center;
		padding-top: 2rem;
		border-top: 1px solid #e9ecef;
	}

	.back-link {
		text-decoration: none;
		color: #646cff;
		padding: 0.5rem 1rem;
		border-radius: 0.25rem;
		transition: background-color 0.2s;
	}

	.back-link:hover {
		background-color: #f8f9fa;
	}
</style>