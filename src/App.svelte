<script lang="ts">
	import Sidebar from '$lib/components/Sidebar/Sidebar.svelte';
	import { pageState } from '$lib/pageSystem.svelte';
	import { authState } from '$lib/authState.svelte';
	import HeroPage from '$lib/pages/HeroPage.svelte';
	import SettingsPage from '$lib/pages/SettingsPage.svelte';
	import LoginPage from '$lib/pages/LoginPage.svelte';
	import CanvasSetupPage from '$lib/pages/CanvasSetupPage.svelte';
	import { themeState } from '$lib/themeState.svelte';
	import { fade } from 'svelte/transition';

	let canvasSkipped = $state(false);
	const needsCanvasSetup = $derived(
		authState.isAuthenticated && !authState.record?.canvas_connected && !canvasSkipped
	);
</script>

<div class="flex h-screen w-full overflow-hidden bg-background text-foreground">
	<!-- Left sidebar with branding, calendar, and bottom user display -->
	<Sidebar />

	<!-- Adaptable right content area powered by the page system -->
	<main class="relative flex flex-1 flex-col overflow-hidden bg-background">
		{#if !authState.isAuthenticated}
			<!-- Show login screen instead of main content when not logged in -->
			<div class="absolute inset-0 flex flex-col" in:fade={{ duration: 250 }} out:fade={{ duration: 150 }}>
				<LoginPage />
			</div>
		{:else if pageState.current === 'settings'}
			<SettingsPage />
		{:else if needsCanvasSetup}
			<!-- Canvas integration setup required right after Google login -->
			<div class="absolute inset-0 flex flex-col" in:fade={{ duration: 250 }} out:fade={{ duration: 150 }}>
				<CanvasSetupPage onSkip={() => (canvasSkipped = true)} />
			</div>
		{:else if pageState.current === 'hero'}
			<div class="absolute inset-0 flex flex-col" in:fade={{ duration: 250, delay: 100 }} out:fade={{ duration: 150 }}>
				<HeroPage />
			</div>
		{:else}
			{@const ActiveComponent = pageState.activePage.component}
			<div class="absolute inset-0 flex flex-col" in:fade={{ duration: 250 }} out:fade={{ duration: 150 }}>
				<ActiveComponent />
			</div>
		{/if}
	</main>
</div>

<style>
	:global(#app) {
		width: 100% !important;
		max-width: 100% !important;
		margin: 0 !important;
		padding: 0 !important;
		border: none !important;
		text-align: left !important;
		height: 100vh !important;
		min-height: 100vh !important;
	}

	:global(h1),
	:global(h2) {
		color: var(--foreground) !important;
	}

	:global(h1),
	:global(h2),
	:global(.font-caveat),
	:global(.font-fascinate),
	:global(.font-keania) {
		font-family: 'Keania One', display, cursive, sans-serif !important;
		letter-spacing: 0.02em;
	}
</style>
