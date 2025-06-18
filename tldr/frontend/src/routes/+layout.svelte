<script>
	import '../app.css';
</script>

<div class="app">
	<header>
		<h1><a href="/">tldr</a></h1>
		<nav>
			<a href="/">Home</a>
			<a href="/about">About</a>
		</nav>
	</header>

	<main>
		<slot />
	</main>

	<footer>
		<p>&copy; 2025 tldr</p>
	</footer>
</div>

<style>
	.app {
		display: flex;
		flex-direction: column;
		min-height: 100vh;
	}

	header {
		padding: 1rem;
		background: #f8f9fa;
		border-bottom: 1px solid #e9ecef;
		display: flex;
		justify-content: space-between;
		align-items: center;
	}

	header h1 {
		margin: 0;
		font-size: 1.5rem;
	}

	header h1 a {
		text-decoration: none;
		color: inherit;
	}

	nav {
		display: flex;
		gap: 1rem;
	}

	nav a {
		text-decoration: none;
		color: #6c757d;
		padding: 0.5rem;
		border-radius: 0.25rem;
		transition: background-color 0.2s;
	}

	nav a:hover {
		background-color: #e9ecef;
	}

	main {
		flex: 1;
		padding: 2rem;
		max-width: 800px;
		margin: 0 auto;
		width: 100%;
	}

	footer {
		padding: 1rem;
		background: #f8f9fa;
		border-top: 1px solid #e9ecef;
		text-align: center;
		color: #6c757d;
		font-size: 0.875rem;
	}
</style>