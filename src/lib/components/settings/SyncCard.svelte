<script lang="ts">
	import { dataState } from "$lib/dataState/dataState.svelte";
	import { syncState } from "$lib/syncState.svelte";
	import { Button } from "$lib/components/ui/button";
	import RefreshCwIcon from "@lucide/svelte/icons/refresh-cw";
	import GraduationCapIcon from "@lucide/svelte/icons/graduation-cap";
	import CalendarIcon from "@lucide/svelte/icons/calendar";
	import UploadCloudIcon from "@lucide/svelte/icons/upload-cloud";
	import CheckIcon from "@lucide/svelte/icons/check";
	import AlertCircleIcon from "@lucide/svelte/icons/alert-circle";
	import SyncSquareButton from "./SyncSquareButton.svelte";
	import { fade } from "svelte/transition";

	const isCanvasActive = $derived(
		Boolean(dataState.canvasSyncedAt) || syncState.canvasSynced,
	);
	const isGoogleInActive = $derived(
		Boolean(dataState.googleImportSyncedAt) || syncState.googleInSynced,
	);
	const isGoogleOutActive = $derived(
		Boolean(dataState.googleExportSyncedAt) || syncState.googleOutSynced,
	);

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

	// Dotted trail fill percentages (0 to 100)
	let trail1Progress = $state(0);
	let trail2Progress = $state(0);

	const isAnySyncing = $derived(syncState.isAnySyncing);

	function formatTime(date: Date | null): string {
		if (!date || isNaN(date.getTime())) return "Never";
		return date.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" });
	}

	function animateTrail(
		trailNum: 1 | 2,
		durationMs: number = 750,
	): Promise<void> {
		return new Promise((resolve) => {
			if (trailNum === 1) trail1Progress = 0;
			if (trailNum === 2) trail2Progress = 0;
			const startTime = performance.now();
			function step(now: number) {
				const elapsed = now - startTime;
				const p = Math.min(100, Math.round((100 * elapsed) / durationMs));
				if (trailNum === 1) trail1Progress = p;
				if (trailNum === 2) trail2Progress = p;
				if (p < 100) {
					requestAnimationFrame(step);
				} else {
					resolve();
				}
			}
			requestAnimationFrame(step);
		});
	}

	async function handleSyncCanvas() {
		if (isAnySyncing) return;
		try {
			await syncState.syncCanvas();
			if (isGoogleInActive) {
				await animateTrail(1, 600);
			}
		} catch (e) {
			// error is tracked in syncState.syncAllError
		} finally {
			trail1Progress = 0;
		}
	}

	async function handleSyncFromGoogle() {
		if (isAnySyncing) return;
		try {
			await syncState.syncFromGoogle();
			if (isCanvasActive) {
				await animateTrail(1, 600);
			}
			if (isGoogleOutActive) {
				await animateTrail(2, 600);
			}
		} catch (e) {
			// error is tracked in syncState.syncAllError
		} finally {
			trail1Progress = 0;
			trail2Progress = 0;
		}
	}

	async function handleSyncToGoogle() {
		if (isAnySyncing) return;
		try {
			await syncState.syncToGoogle();
			if (isGoogleInActive) {
				await animateTrail(2, 600);
			}
		} catch (e) {
			// error is tracked in syncState.syncAllError
		} finally {
			trail2Progress = 0;
		}
	}

	async function handleSyncAll() {
		if (isAnySyncing) return;
		trail1Progress = 0;
		trail2Progress = 0;

		try {
			await syncState.syncAll({
				onStepChange: async (step) => {
					if (step === "google-in" && isCanvasActive) {
						await animateTrail(1, 750);
					} else if (step === "google-out") {
						await animateTrail(2, 750);
					}
				},
			});
		} finally {
			trail1Progress = 0;
			trail2Progress = 0;
		}
	}
</script>

<div class="rounded-xl border border-border bg-card p-6 shadow-xs space-y-5">
	<!-- Header -->
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
		<div class="flex items-center gap-3">
			<div>
				<h2
					class="text-base font-normal tracking-wide text-card-foreground leading-none"
				>
					Data Synchronization
				</h2>
			</div>
		</div>

		<!-- Status & Sync All Button -->
		<div class="flex items-center gap-2 self-start sm:self-center">
			<Button
				size="sm"
				class="text-xs h-7 px-3 cursor-pointer gap-1.5 shrink-0 bg-primary text-primary-foreground shadow-xs hover:bg-primary/90 disabled:opacity-50"
				onclick={handleSyncAll}
				disabled={(!syncState.isCanvasConnected &&
					!syncState.isGoogleConnected) ||
					isAnySyncing}
				title="Run full pipeline sync"
			>
				<RefreshCwIcon class="size-3" />
				<span>{syncState.isSyncingAll ? "Syncing..." : "Sync All"}</span>
			</Button>
		</div>
	</div>

	<!-- Action Feedback Messages -->
	{#if syncState.syncAllMessage}
		<div
			class="flex items-center gap-2 text-xs text-emerald-500 bg-emerald-500/10 border border-emerald-500/20 rounded-md px-3 py-2"
			in:fade={{ duration: 150 }}
		>
			<CheckIcon class="size-4 shrink-0" />
			<span>{syncState.syncAllMessage}</span>
		</div>
	{/if}

	{#if syncState.syncAllError}
		<div
			class="flex items-center gap-2 text-xs text-destructive bg-destructive/10 border border-destructive/20 rounded-md px-3 py-2"
			in:fade={{ duration: 150 }}
		>
			<AlertCircleIcon class="size-4 shrink-0" />
			<span>{syncState.syncAllError}</span>
		</div>
	{/if}

	<!-- Linear Data Sync Pipeline (Arranged in a line, square parts, icon background, animated dotted trail) -->
	<div class="py-2">
		<div class="flex items-center justify-between gap-1.5 sm:gap-3 w-full">
			<!-- Part 1: Canvas LMS -->
			<SyncSquareButton
				title="Canvas LMS"
				description="Import coursework"
				icon={GraduationCapIcon}
				isSynced={isCanvasActive}
				isSyncing={dataState.isCanvasSyncing ||
					syncState.isSyncingCanvas ||
					syncState.syncingStep === "canvas"}
				syncingLabel="Syncing"
				isConnected={syncState.isCanvasConnected}
				lastSynced={canvasLastSynced}
				disabled={isAnySyncing}
				onclick={handleSyncCanvas}
			/>

			<!-- Trail 1: Animated Dotted Trail between Canvas and Google In -->
			<div
				class="flex-1 min-w-[20px] max-w-[80px] sm:max-w-[120px] flex items-center justify-evenly px-1 sm:px-2"
			>
				{#each Array(6) as _, i}
					{@const isDotActive =
						isAnySyncing && trail1Progress >= ((i + 0.5) / 6) * 100}
					<div
						class="size-1.5 sm:size-2 rounded-full transition-all duration-300 {isDotActive
							? 'bg-primary dot-primary-wave'
							: 'bg-zinc-300 dark:bg-zinc-700'}"
						style="--dot-idx: {i};"
					></div>
				{/each}
			</div>

			<!-- Part 2: Google Inbound -->
			<SyncSquareButton
				title="Google Import"
				description="Fetch calendars"
				icon={CalendarIcon}
				isSynced={isGoogleInActive}
				isSyncing={dataState.isGoogleImporting ||
					syncState.isSyncingGoogleIn ||
					syncState.syncingStep === "google-in"}
				syncingLabel="Fetching"
				isConnected={syncState.isGoogleConnected}
				lastSynced={googleImportLastSynced}
				disabled={isAnySyncing}
				onclick={handleSyncFromGoogle}
			/>

			<!-- Trail 2: Animated Dotted Trail between Google In and Google Out -->
			<div
				class="flex-1 min-w-[20px] max-w-[80px] sm:max-w-[120px] flex items-center justify-evenly px-1 sm:px-2"
			>
				{#each Array(6) as _, i}
					{@const isDotActive =
						isAnySyncing && trail2Progress >= ((i + 0.5) / 6) * 100}
					<div
						class="size-1.5 sm:size-2 rounded-full transition-all duration-300 {isDotActive
							? 'bg-primary dot-primary-wave'
							: 'bg-zinc-300 dark:bg-zinc-700'}"
						style="--dot-idx: {i + 6};"
					></div>
				{/each}
			</div>

			<!-- Part 3: Google Outbound -->
			<SyncSquareButton
				title="Google Export"
				description="Push tasks"
				icon={UploadCloudIcon}
				isSynced={isGoogleOutActive}
				isSyncing={dataState.isGoogleExporting ||
					syncState.isSyncingGoogleOut ||
					syncState.syncingStep === "google-out"}
				syncingLabel="Pushing"
				isConnected={syncState.isGoogleConnected}
				lastSynced={googleExportLastSynced}
				disabled={isAnySyncing}
				onclick={handleSyncToGoogle}
			/>
		</div>
	</div>
</div>

<style>
	@keyframes dotWave {
		0%,
		100% {
			transform: scale(1);
			opacity: 0.8;
		}
		50% {
			transform: scale(1.3);
			opacity: 1;
		}
	}

	.dot-primary-wave {
		animation: dotWave 1.4s ease-in-out infinite;
		animation-delay: calc(var(--dot-idx) * 140ms);
	}
</style>
