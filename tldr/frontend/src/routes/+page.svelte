<script>
	import { onMount } from 'svelte';

	let summaries = [];
	let loading = true;
	let error = null;

	onMount(async () => {
		try {
			// TODO: Replace with actual API endpoint
			const response = await fetch('/api/summaries');
			if (response.ok) {
				summaries = await response.json();
			} else {
				// Fallback to mock data for development
				summaries = [
					{
						id: 1,
						title: 'Weekly Tech Summary - Week 25',
						date: '2025-06-18',
						excerpt: 'Key developments in AI, cloud computing, and web development this week...'
					},
					{
						id: 2,
						title: 'Monthly Industry Report - June 2025',
						date: '2025-06-15',
						excerpt: 'Comprehensive overview of technology trends and market movements...'
					},
					{
						id: 3,
						title: 'Quick Update - Breaking News',
						date: '2025-06-17',
						excerpt: 'Important updates on recent technology announcements and releases...'
					}
				];
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
	<title>tldr - Latest Summaries</title>
</svelte:head>

<div class="home">
	<h1>Latest Summaries</h1>
	<p>Stay up to date with concise summaries of the latest developments.</p>

	{#if loading}
		<div class="loading">Loading summaries...</div>
	{:else if error}
		<div class="error">Error loading summaries: {error}</div>
	{:else if summaries.length === 0}
		<div class="empty">No summaries available yet.</div>
	{:else}
		<div class="summary-list">
			{#each summaries as summary (summary.id)}
				<article class="summary-item">
					<h2 class="summary-title">
						<a href="/summary/{summary.id}">{summary.title}</a>
					</h2>
					<div class="summary-date">{formatDate(summary.date)}</div>
					<p class="summary-excerpt">{summary.excerpt}</p>
				</article>
			{/each}
		</div>
	{/if}
</div>

<style>
	.home {
		text-align: center;
	}

	.home h1 {
		font-size: 2.5rem;
		margin-bottom: 0.5rem;
	}

	.home p {
		font-size: 1.125rem;
		color: #6c757d;
		margin-bottom: 2rem;
	}

	.loading, .error, .empty {
		padding: 2rem;
		color: #6c757d;
		font-style: italic;
	}

	.error {
		color: #dc3545;
	}

	.summary-title a {
		text-decoration: none;
		color: inherit;
	}

	.summary-title a:hover {
		color: #646cff;
	}
</style>