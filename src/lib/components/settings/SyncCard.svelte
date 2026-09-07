<script lang="ts">
	import { dataState } from "$lib/dataState/dataState.svelte";
	import { syncController } from "$lib/syncController.svelte";
	import { Button } from "$lib/components/ui/button";
	import RefreshCwIcon from "@lucide/svelte/icons/refresh-cw";
	import GraduationCapIcon from "@lucide/svelte/icons/graduation-cap";
	import CalendarIcon from "@lucide/svelte/icons/calendar";
	import UploadCloudIcon from "@lucide/svelte/icons/upload-cloud";
	import ArrowDownIcon from "@lucide/svelte/icons/arrow-down";
	import CheckIcon from "@lucide/svelte/icons/check";
	import AlertCircleIcon from "@lucide/svelte/icons/alert-circle";
	import SyncServiceCard from "./SyncServiceCard.svelte";
	import { fade } from "svelte/transition";

	function parseSyncDate(dateStr?: string | null): Date | null {
		if (!dateStr) return null;
		const d = new Date(dateStr.replace(" ", "T"));
		return isNaN(d.getTime()) ? null : d;
	}

	const canvasLastSynced = $derived(parseSyncDate(dataState.canvasSyncedAt));
	const googleImportLastSynced = $derived(
		parseSyncDate(dataState.googleImportSyncedAt),
	);
	const googleExportLastSynced = $derived(
		parseSyncDate(dataState.googleExportSyncedAt),
	);

	const isAnySyncing = $derived(syncController.isAnySyncing);

	const isCanvasSyncing = $derived(
		dataState.isCanvasSyncing ||
			syncController.isSyncingCanvas ||
			(syncController.isSyncingAll && syncController.syncingStage === 1),
	);

	const isGoogleInSyncing = $derived(
		dataState.isGoogleImporting ||
			syncController.isSyncingGoogleIn ||
			(syncController.isSyncingAll && syncController.syncingStage === 1),
	);

	const isGoogleOutSyncing = $derived(
		dataState.isGoogleExporting ||
			syncController.isSyncingGoogleOut ||
			(syncController.isSyncingAll && syncController.syncingStage === 2),
	);

	async function handleSyncCanvas() {
		if (isAnySyncing) return;
		try {
			await syncController.syncCanvas();
		} catch (e) {
			// error tracked in syncController.syncAllError
		}
	}

	async function handleSyncFromGoogle() {
		if (isAnySyncing) return;
		try {
			await syncController.syncFromGoogle();
		} catch (e) {
			// error tracked in syncController.syncAllError
		}
	}

	async function handleSyncToGoogle() {
		if (isAnySyncing) return;
		try {
			await syncController.syncToGoogle();
		} catch (e) {
			// error tracked in syncController.syncAllError
		}
	}

	async function handleSyncAll() {
		if (isAnySyncing) return;
		try {
			await syncController.syncAll();
		} catch (e) {
			// error tracked in syncController.syncAllError
		}
	}
</script>

<div class="rounded-xl border border-border bg-card p-5 sm:p-6 shadow-xs space-y-4">
	<!-- Header -->
	<div class="flex items-center justify-between gap-3">
		<h2
			class="text-base font-normal tracking-wide text-card-foreground leading-none"
		>
			Data Synchronization
		</h2>

		<!-- Global Sync All Button -->
		<Button
			size="sm"
			class="text-xs h-7 px-3 cursor-pointer gap-1.5 shrink-0 bg-primary text-primary-foreground shadow-xs hover:bg-primary/90 disabled:opacity-50"
			onclick={handleSyncAll}
			disabled={(!syncController.isCanvasConnected &&
				!syncController.isGoogleConnected) ||
				isAnySyncing}
			title="Run full sync pipeline"
		>
			<RefreshCwIcon
				class="size-3 {syncController.isSyncingAll ? 'animate-spin' : ''}"
			/>
			<span>
				{#if syncController.isSyncingAll}
					{#if syncController.syncingStage === 1}
						Importing...
					{:else if syncController.syncingStage === 2}
						Exporting...
					{:else}
						Syncing...
					{/if}
				{:else}
					Sync All
				{/if}
			</span>
		</Button>
	</div>

	<!-- Stage 1: Inbound Import -->
	<div class="space-y-2 pt-1">
		<div
			class="text-[11px] font-semibold text-muted-foreground uppercase tracking-wider"
		>
			Import to Lasso
		</div>

		<div class="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
			<SyncServiceCard
				title="Canvas LMS"
				icon={GraduationCapIcon}
				iconClass="bg-rose-500/10 text-rose-600 dark:text-rose-400"
				isConnected={syncController.isCanvasConnected}
				isSyncing={isCanvasSyncing}
				lastSynced={canvasLastSynced}
				disabled={isAnySyncing}
				onclick={handleSyncCanvas}
			/>

			<SyncServiceCard
				title="Google Calendar"
				icon={CalendarIcon}
				iconClass="bg-blue-500/10 text-blue-600 dark:text-blue-400"
				isConnected={syncController.isGoogleConnected}
				isSyncing={isGoogleInSyncing}
				lastSynced={googleImportLastSynced}
				disabled={isAnySyncing}
				onclick={handleSyncFromGoogle}
			/>
		</div>
	</div>

	<!-- Stage Flow Indicator -->
	<div class="flex items-center justify-center py-0.5">
		<div
			class="flex items-center gap-1.5 text-[10px] text-muted-foreground font-medium uppercase tracking-wider"
		>
			<span>Then</span>
			<ArrowDownIcon class="size-3 text-muted-foreground/80" />
		</div>
	</div>

	<!-- Stage 2: Outbound Export -->
	<div class="space-y-2">
		<div
			class="text-[11px] font-semibold text-muted-foreground uppercase tracking-wider"
		>
			Export from Lasso
		</div>

		<div>
			<SyncServiceCard
				title="Google Calendar Export"
				icon={UploadCloudIcon}
				iconClass="bg-blue-500/10 text-blue-600 dark:text-blue-400"
				isConnected={syncController.isGoogleConnected}
				isEnabled={dataState.isGoogleExportEnabled}
				isSyncing={isGoogleOutSyncing}
				lastSynced={googleExportLastSynced}
				disabled={isAnySyncing}
				onclick={handleSyncToGoogle}
			/>
		</div>
	</div>

	<!-- Error / Status Readouts at the bottom of the card -->
	{#if syncController.syncAllMessage || syncController.syncAllError || dataState.canvasSyncError || dataState.googleImportError || dataState.googleExportError}
		<div class="pt-2 border-t border-border/40 space-y-2">
			{#if syncController.syncAllMessage}
				<div
					class="flex items-center gap-2 text-xs text-emerald-600 dark:text-emerald-400 bg-emerald-500/10 border border-emerald-500/20 rounded-md px-3 py-2"
					in:fade={{ duration: 150 }}
				>
					<CheckIcon class="size-3.5 shrink-0" />
					<span>{syncController.syncAllMessage}</span>
				</div>
			{/if}

			{#if syncController.syncAllError}
				<div
					class="flex items-center gap-2 text-xs text-destructive bg-destructive/10 border border-destructive/20 rounded-md px-3 py-2"
					in:fade={{ duration: 150 }}
				>
					<AlertCircleIcon class="size-3.5 shrink-0" />
					<span>{syncController.syncAllError}</span>
				</div>
			{/if}

			{#if dataState.canvasSyncError && !syncController.syncAllError}
				<div
					class="flex items-center gap-2 text-xs text-destructive bg-destructive/10 border border-destructive/20 rounded-md px-3 py-2"
					in:fade={{ duration: 150 }}
				>
					<AlertCircleIcon class="size-3.5 shrink-0" />
					<span>Canvas: {dataState.canvasSyncError}</span>
				</div>
			{/if}

			{#if dataState.googleImportError && !syncController.syncAllError}
				<div
					class="flex items-center gap-2 text-xs text-destructive bg-destructive/10 border border-destructive/20 rounded-md px-3 py-2"
					in:fade={{ duration: 150 }}
				>
					<AlertCircleIcon class="size-3.5 shrink-0" />
					<span>Google Import: {dataState.googleImportError}</span>
				</div>
			{/if}

			{#if dataState.googleExportError && !syncController.syncAllError}
				<div
					class="flex items-center gap-2 text-xs text-destructive bg-destructive/10 border border-destructive/20 rounded-md px-3 py-2"
					in:fade={{ duration: 150 }}
				>
					<AlertCircleIcon class="size-3.5 shrink-0" />
					<span>Google Export: {dataState.googleExportError}</span>
				</div>
			{/if}
		</div>
	{/if}
</div>
