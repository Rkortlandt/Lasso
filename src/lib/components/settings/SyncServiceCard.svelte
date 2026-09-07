<script lang="ts">
	import type { Component } from "svelte";
	import { Button } from "$lib/components/ui/button";
	import RefreshCwIcon from "@lucide/svelte/icons/refresh-cw";

	interface Props {
		title: string;
		icon: Component;
		iconClass?: string;
		isConnected?: boolean;
		isEnabled?: boolean;
		isSyncing?: boolean;
		syncingLabel?: string;
		lastSynced?: Date | null;
		statusText?: string | null;
		disabled?: boolean;
		onclick?: () => void;
	}

	let {
		title,
		icon: Icon,
		iconClass = "bg-primary/10 text-primary",
		isConnected = true,
		isEnabled = true,
		isSyncing = false,
		syncingLabel = "Syncing...",
		lastSynced = null,
		statusText = null,
		disabled = false,
		onclick,
	}: Props = $props();

	function formatTime(date: Date | null): string {
		if (!date || isNaN(date.getTime())) return "Never synced";
		return date.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" });
	}
</script>

<div
	class="flex items-center justify-between gap-3 rounded-lg border border-border/80 bg-card p-3 sm:p-3.5 transition-all hover:border-border {isSyncing
		? 'border-primary/50 ring-1 ring-primary/20 bg-primary/5'
		: ''} {!isConnected || !isEnabled ? 'opacity-65' : ''}"
>
	<div class="flex items-center gap-3 min-w-0">
		<div
			class="size-8 sm:size-9 rounded-lg flex items-center justify-center shrink-0 {iconClass}"
		>
			<Icon class="size-4 sm:size-4.5" />
		</div>
		<div class="min-w-0">
			<div class="text-xs sm:text-sm font-medium text-foreground leading-tight">
				{title}
			</div>
			<div
				class="text-[11px] font-mono text-muted-foreground leading-tight mt-0.5"
			>
				{#if isSyncing}
					<span class="text-primary">{syncingLabel}</span>
				{:else if !isConnected}
					<span>Not connected</span>
				{:else if !isEnabled}
					<span>Disabled</span>
				{:else if statusText}
					<span>{statusText}</span>
				{:else if lastSynced}
					<span>Synced {formatTime(lastSynced)}</span>
				{:else}
					<span>Never synced</span>
				{/if}
			</div>
		</div>
	</div>

	<Button
		variant="outline"
		size="sm"
		class="text-xs h-7 px-2.5 gap-1.5 cursor-pointer shrink-0 border-border hover:bg-muted/80 disabled:opacity-50"
		{onclick}
		disabled={disabled || isSyncing || !isConnected || !isEnabled}
	>
		<RefreshCwIcon
			class="size-3 {isSyncing ? 'animate-spin text-primary' : ''}"
		/>
		<span>{isSyncing ? "Syncing" : "Sync"}</span>
	</Button>
</div>
