<script lang="ts">
	import type { Component } from "svelte";
	import RefreshCwIcon from "@lucide/svelte/icons/refresh-cw";

	interface Props {
		title: string;
		description: string;
		icon: Component;
		isSynced?: boolean;
		isBlue?: boolean;
		isSyncing?: boolean;
		syncingLabel?: string;
		isConnected?: boolean;
		lastSynced?: Date | null;
		disabled?: boolean;
		onclick?: () => void;
		titleTooltip?: string;
	}

	let {
		title,
		description,
		icon: Icon,
		isSynced = false,
		isBlue = false,
		isSyncing = false,
		syncingLabel = "Syncing",
		isConnected = true,
		lastSynced = null,
		disabled = false,
		onclick,
		titleTooltip,
	}: Props = $props();

	const active = $derived(isSynced || isBlue);

	function formatTime(date: Date | null): string {
		if (!date || isNaN(date.getTime())) return "Never";
		return date.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" });
	}

	const tooltip = $derived(
		titleTooltip ??
			(isConnected ? `Click to sync ${title}` : `${title} not connected`),
	);
</script>

<button
	type="button"
	class="group relative flex-1 min-w-[92px] max-w-[200px] aspect-square rounded-2xl border transition-all duration-300 overflow-hidden text-left select-none p-3 sm:p-4 flex flex-col justify-between cursor-pointer bg-muted/20 hover:bg-muted/35 {active
		? 'border-primary'
		: 'border-border/80 hover:border-primary/50'} {isSyncing
		? 'border-primary ring-2 ring-primary/20'
		: ''}"
	{onclick}
	{disabled}
	title={tooltip}
>
	<!-- Icon Background Layer: changes to refresh on hover or active sync -->
	<div
		class="absolute inset-0 flex items-center justify-center pointer-events-none p-3 overflow-hidden"
	>
		<!-- Base Service Icon: outline only & icon color should be primary -->
		<div
			class="sync-bg-icon {active
				? 'text-primary'
				: 'text-zinc-300 dark:text-zinc-700'} {isSyncing
				? 'opacity-0 scale-25'
				: 'group-hover:opacity-0 group-hover:scale-50 group-hover:rotate-[-10deg]'}"
		>
			<Icon class="size-8 sm:size-12" strokeWidth={1.5} />
		</div>

		<!-- Refresh Icon: visible on hover or active sync -->
		<div
			class="absolute transition-all duration-300 {isSyncing
				? 'opacity-100 scale-100 animate-spin text-primary'
				: 'opacity-0 scale-25 rotate-45 group-hover:opacity-100 group-hover:scale-100 group-hover:rotate-0 text-primary'}"
		>
			<RefreshCwIcon class="size-8 sm:size-12" />
		</div>
	</div>

	<!-- Foreground Text Layer (Above the background icon) -->
	<div class="relative z-10 space-y-0.5 pointer-events-none">
		<div
			class="text-xs sm:text-sm font-semibold text-card-foreground tracking-tight group-hover:text-primary transition-colors"
		>
			{title}
		</div>
	</div>

	<!-- Status Info at Bottom -->
	<div>
		<div
			class="relative z-10 flex items-center justify-between text-[10px] sm:text-[11px] font-mono text-muted-foreground/80 pointer-events-none"
		>
			{#if isSyncing}
				<span class="text-primary font-medium">
					{syncingLabel}
				</span>
			{:else if !isConnected}
				<span class="text-muted-foreground/50">Off</span>
			{:else if lastSynced}
				<span>
					{formatTime(lastSynced)}
				</span>
			{:else}
				<span>Ready</span>
			{/if}
		</div>
		<div
			class="text-[10px] sm:text-xs text-muted-foreground leading-tight line-clamp-2"
		>
			{description}
		</div>
	</div>
</button>

<style>
	.sync-bg-icon {
		transition:
			color 0.4s ease,
			opacity 0.35s ease,
			transform 0.35s ease;
	}
</style>
