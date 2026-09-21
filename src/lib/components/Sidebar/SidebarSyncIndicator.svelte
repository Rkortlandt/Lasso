<script lang="ts">
	import { syncController } from "$lib/syncController.svelte";
	import { dataState } from "$lib/dataState/dataState.svelte";
	import RefreshCwIcon from "@lucide/svelte/icons/refresh-cw";

	let currentTime = $state(
		new Date().toLocaleTimeString([], { hour: "numeric", minute: "2-digit" }),
	);

	$effect(() => {
		const interval = setInterval(() => {
			currentTime = new Date().toLocaleTimeString([], {
				hour: "numeric",
				minute: "2-digit",
			});
		}, 10000);
		return () => clearInterval(interval);
	});

	function parseSyncDate(dateStr?: string | null): Date | null {
		if (!dateStr) return null;
		const d = new Date(dateStr.replace(" ", "T"));
		return isNaN(d.getTime()) ? null : d;
	}

	const isAnySyncing = $derived(syncController.isAnySyncing);

	const isAnyConnected = $derived(
		syncController.isCanvasConnected || syncController.isGoogleConnected,
	);

	const oldestSyncDate = $derived.by(() => {
		const timestamps: number[] = [];
		if (syncController.isCanvasConnected && dataState.canvasSyncedAt) {
			const d = parseSyncDate(dataState.canvasSyncedAt);
			if (d) timestamps.push(d.getTime());
		}
		if (syncController.isGoogleConnected) {
			if (dataState.googleImportSyncedAt) {
				const d = parseSyncDate(dataState.googleImportSyncedAt);
				if (d) timestamps.push(d.getTime());
			}
			if (dataState.googleExportSyncedAt) {
				const d = parseSyncDate(dataState.googleExportSyncedAt);
				if (d) timestamps.push(d.getTime());
			}
		}
		if (timestamps.length === 0) return null;
		return new Date(Math.min(...timestamps));
	});

	const oldestSyncTimeFormatted = $derived(
		oldestSyncDate
			? oldestSyncDate.toLocaleTimeString([], {
					hour: "numeric",
					minute: "2-digit",
				})
			: null,
	);

	const syncErrorMessage = $derived(
		syncController.syncAllError ||
			dataState.canvasSyncError ||
			dataState.googleImportError ||
			dataState.googleExportError ||
			null,
	);
	const hasSyncError = $derived(Boolean(syncErrorMessage));

	const syncTooltip = $derived.by(() => {
		if (isAnySyncing) {
			if (syncController.syncingStage === 1) {
				return "Stage 1: Importing coursework and schedules...";
			}
			if (syncController.syncingStage === 2) {
				return "Stage 2: Exporting deadlines to Google Calendar...";
			}
			return "Syncing in progress...";
		}
		if (hasSyncError) {
			return `Sync issue: ${syncErrorMessage} — Click to retry`;
		}
		if (!isAnyConnected) return "No sync services connected";
		if (oldestSyncTimeFormatted) {
			return `Last sync: ${oldestSyncTimeFormatted} — Click to sync all`;
		}
		return "Connected — Click to sync all";
	});

	function handleSyncClick() {
		if (isAnyConnected && !isAnySyncing) {
			syncController.syncAll().catch(() => {});
		}
	}
</script>

<button
	type="button"
	class="flex items-center justify-center w-full px-4 text-muted-foreground select-none hover:text-foreground transition-colors cursor-pointer"
	title={syncTooltip}
	onclick={handleSyncClick}
>
	<!-- Left: Live Clock -->
	<div class="flex-1 text-right pr-2">
		<span class="text-[11px] font-medium text-foreground/80 tabular-nums">
			{currentTime}
		</span>
	</div>

	<!-- Center: Sync Status Dot -->
	{#if isAnySyncing}
		<RefreshCwIcon class="size-2.5 animate-spin text-primary shrink-0" />
	{:else if hasSyncError}
		<span
			class="size-1.5 rounded-full bg-destructive shadow-[0_0_5px_rgba(239,68,68,0.7)] shrink-0"
		></span>
	{:else if isAnyConnected}
		<span
			class="size-1.5 rounded-full bg-emerald-500 shadow-[0_0_5px_rgba(16,185,129,0.5)] shrink-0"
		></span>
	{:else}
		<span class="size-1.5 rounded-full bg-muted-foreground/40 shrink-0"></span>
	{/if}

	<!-- Right: Last Sync Time / Status -->
	<div class="flex-1 text-left pl-2 truncate">
		<span
			class="text-[10px] tabular-nums {hasSyncError && !isAnySyncing
				? 'text-destructive font-medium'
				: 'text-muted-foreground/80'}"
		>
			{#if isAnySyncing}
				{#if syncController.syncingStage === 1}
					importing...
				{:else if syncController.syncingStage === 2}
					exporting...
				{:else}
					syncing...
				{/if}
			{:else if hasSyncError}
				error
			{:else if oldestSyncTimeFormatted}
				{oldestSyncTimeFormatted}
			{:else if isAnyConnected}
				ready
			{:else}
				offline
			{/if}
		</span>
	</div>
</button>
