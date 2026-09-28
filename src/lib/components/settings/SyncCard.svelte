<script lang="ts">
	import { dataState } from "$lib/dataState/dataState.svelte";
	import { syncController } from "$lib/syncController.svelte";
	import { Button } from "$lib/components/ui/button";
	import RefreshCwIcon from "@lucide/svelte/icons/refresh-cw";
	import GraduationCapIcon from "@lucide/svelte/icons/graduation-cap";
	import CalendarIcon from "@lucide/svelte/icons/calendar";
	import UploadCloudIcon from "@lucide/svelte/icons/upload-cloud";
	import CheckIcon from "@lucide/svelte/icons/check";
	import AlertCircleIcon from "@lucide/svelte/icons/alert-circle";
	import { fade } from "svelte/transition";

	function parseSyncDate(dateStr?: string | null): Date | null {
		if (!dateStr) return null;
		const d = new Date(dateStr.replace(" ", "T"));
		return isNaN(d.getTime()) ? null : d;
	}

	function formatTime(date: Date | null): string {
		if (!date || isNaN(date.getTime())) return "";
		return date.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" });
	}

	const canvasLastSynced = $derived(parseSyncDate(dataState.canvasSyncedAt));
	const googleImportLastSynced = $derived(
		parseSyncDate(dataState.googleImportSyncedAt),
	);
	const googleExportLastSynced = $derived(
		parseSyncDate(dataState.googleExportSyncedAt),
	);

	const isCanvasConnected = $derived(syncController.isCanvasConnected);
	const isGoogleConnected = $derived(syncController.isGoogleConnected);
	const isGoogleExportEnabled = $derived(dataState.isGoogleExportEnabled);

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

	const isWaitingForImports = $derived(
		syncController.isSyncingAll && syncController.syncingStage === 1,
	);

	// Item count heuristics
	const canvasTaskCount = $derived(
		dataState.tasks.filter((t) => {
			const cal = dataState.calendars.find((c) => c.id === t.calendar);
			return cal?.source === "canvas" || Boolean(cal?.course_id);
		}).length,
	);

	const googleImportEventCount = $derived(
		dataState.events.filter((e) => e.source === "google" || Boolean(e.google_event_id)).length,
	);

	const googleExportEventCount = $derived(
		dataState.events.filter((e) => e.deadline || e.recurr || (e.source !== "google" && !e.google_event_id)).length,
	);

	// Errors
	const rawCanvasError = $derived(
		dataState.canvasSyncError ||
			(syncController.syncAllError &&
			syncController.syncAllError.toLowerCase().includes("canvas")
				? syncController.syncAllError
				: null),
	);

	const canvasErrorMessage = $derived.by(() => {
		if (!rawCanvasError) return null;
		const lower = rawCanvasError.toLowerCase();
		if (lower.includes("token") || lower.includes("401") || lower.includes("unauthorized")) {
			return "Couldn't reach Canvas. Check your token.";
		}
		if (lower.includes("failed to fetch") || lower.includes("network")) {
			return "Couldn't reach Canvas. Check your network or URL.";
		}
		return rawCanvasError;
	});

	const rawGoogleImportError = $derived(
		dataState.googleImportError ||
			(syncController.syncAllError &&
			(syncController.syncAllError.toLowerCase().includes("google inbound") ||
				syncController.syncAllError.toLowerCase().includes("google in"))
				? syncController.syncAllError
				: null),
	);

	const googleImportErrorMessage = $derived.by(() => {
		if (!rawGoogleImportError) return null;
		const lower = rawGoogleImportError.toLowerCase();
		if (lower.includes("token") || lower.includes("401") || lower.includes("unauthorized") || lower.includes("auth")) {
			return "Google authentication failed. Reconnect your account.";
		}
		return rawGoogleImportError;
	});

	const rawGoogleExportError = $derived(
		dataState.googleExportError ||
			(syncController.syncAllError &&
			(syncController.syncAllError.toLowerCase().includes("google outbound") ||
				syncController.syncAllError.toLowerCase().includes("google out"))
				? syncController.syncAllError
				: null),
	);

	const googleExportErrorMessage = $derived.by(() => {
		if (!rawGoogleExportError) return null;
		const lower = rawGoogleExportError.toLowerCase();
		if (lower.includes("token") || lower.includes("401") || lower.includes("unauthorized") || lower.includes("auth")) {
			return "Google authentication failed. Reconnect your account.";
		}
		return rawGoogleExportError;
	});

	// Step completion / status indicators
	const isStep1Syncing = $derived(isCanvasSyncing || isGoogleInSyncing);
	const hasStep1Error = $derived(
		Boolean((isCanvasConnected && canvasErrorMessage) || (isGoogleConnected && googleImportErrorMessage)),
	);
	const isStep1Success = $derived(
		!isStep1Syncing &&
			!hasStep1Error &&
			(canvasLastSynced !== null || googleImportLastSynced !== null),
	);

	const isStep2Syncing = $derived(isGoogleOutSyncing);
	const hasStep2Error = $derived(
		Boolean(isGoogleConnected && isGoogleExportEnabled && googleExportErrorMessage),
	);
	const isStep2Success = $derived(
		!isStep2Syncing && !hasStep2Error && googleExportLastSynced !== null,
	);

	async function handleSyncCanvas() {
		if (isAnySyncing) return;
		try {
			await syncController.syncCanvas();
		} catch (e) {
			// error tracked in syncController.syncAllError / dataState
		}
	}

	async function handleSyncFromGoogle() {
		if (isAnySyncing) return;
		try {
			await syncController.syncFromGoogle();
		} catch (e) {
			// error tracked in syncController.syncAllError / dataState
		}
	}

	async function handleSyncToGoogle() {
		if (isAnySyncing) return;
		try {
			await syncController.syncToGoogle();
		} catch (e) {
			// error tracked in syncController.syncAllError / dataState
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

<div class="rounded-2xl border border-border bg-card p-5 sm:p-6 shadow-xs space-y-6">
	<!-- Card Header: Title & Global Sync Button -->
	<div class="flex items-center justify-between gap-3">
		<h2 class="text-base font-normal tracking-wide text-card-foreground leading-none">
			Data Synchronization
		</h2>

		<Button
			size="sm"
			class="text-xs h-7 px-3 cursor-pointer gap-1.5 shrink-0 bg-primary text-primary-foreground shadow-xs hover:bg-primary/90 disabled:opacity-50"
			onclick={handleSyncAll}
			disabled={(!isCanvasConnected && !isGoogleConnected) || isAnySyncing}
			title="Run full sync pipeline"
		>
			<RefreshCwIcon class="size-3 {syncController.isSyncingAll ? 'animate-spin' : ''}" />
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

	<!-- Stepped Pipeline Layout -->
	<div class="relative flex gap-3.5 sm:gap-4.5">
		<!-- Left Timeline Column -->
		<div class="flex flex-col items-center shrink-0 w-7 sm:w-8 pt-0.5">
			<!-- Step 1 Circle Indicator -->
			{#if isStep1Syncing}
				<div
					class="size-7 sm:size-8 rounded-full flex items-center justify-center bg-emerald-950/60 border border-emerald-500/50 text-emerald-400 shrink-0 shadow-xs"
					title="Step 1 importing..."
				>
					<RefreshCwIcon class="size-3.5 animate-spin" />
				</div>
			{:else if hasStep1Error}
				<div
					class="size-7 sm:size-8 rounded-full flex items-center justify-center bg-amber-950/60 border border-amber-500/50 text-amber-400 shrink-0 shadow-xs"
					title="Step 1 import warning"
				>
					<AlertCircleIcon class="size-3.5" />
				</div>
			{:else if isStep1Success}
				<div
					class="size-7 sm:size-8 rounded-full flex items-center justify-center bg-[#0d2a1b] border border-emerald-600/40 text-emerald-400 shrink-0 shadow-xs"
					title="Step 1 completed"
				>
					<CheckIcon class="size-3.5 sm:size-4 stroke-[2.5]" />
				</div>
			{:else}
				<div
					class="size-7 sm:size-8 rounded-full flex items-center justify-center border border-dashed border-zinc-600/80 bg-card/80 text-muted-foreground font-mono text-xs shrink-0"
					title="Step 1: Import"
				>
					1
				</div>
			{/if}

			<!-- Connecting Line -->
			<div class="w-[1.5px] flex-1 my-1.5 bg-border/80 min-h-[36px]"></div>

			<!-- Step 2 Circle Indicator -->
			{#if isStep2Syncing}
				<div
					class="size-7 sm:size-8 rounded-full flex items-center justify-center bg-emerald-950/60 border border-emerald-500/50 text-emerald-400 shrink-0 shadow-xs"
					title="Step 2 exporting..."
				>
					<RefreshCwIcon class="size-3.5 animate-spin" />
				</div>
			{:else if hasStep2Error}
				<div
					class="size-7 sm:size-8 rounded-full flex items-center justify-center bg-amber-950/60 border border-amber-500/50 text-amber-400 shrink-0 shadow-xs"
					title="Step 2 export warning"
				>
					<AlertCircleIcon class="size-3.5" />
				</div>
			{:else if isStep2Success}
				<div
					class="size-7 sm:size-8 rounded-full flex items-center justify-center bg-[#0d2a1b] border border-emerald-600/40 text-emerald-400 shrink-0 shadow-xs"
					title="Step 2 completed"
				>
					<CheckIcon class="size-3.5 sm:size-4 stroke-[2.5]" />
				</div>
			{:else}
				<div
					class="size-7 sm:size-8 rounded-full flex items-center justify-center border border-dashed border-zinc-600/80 bg-card/80 text-muted-foreground font-mono text-xs shrink-0"
					title="Step 2: Export"
				>
					2
				</div>
			{/if}
		</div>

		<!-- Right Content Column -->
		<div class="flex-1 min-w-0 space-y-5 sm:space-y-6">
			<!-- Step 1: Import Section -->
			<div class="space-y-2.5">
				<div
					class="h-7 sm:h-8 flex items-center text-sm font-medium text-foreground tracking-tight select-none"
				>
					Step 1 · import
				</div>

				<div class="space-y-2.5">
					<!-- Canvas LMS Card -->
					<div
						class="rounded-xl border transition-all p-3.5 sm:p-4 flex items-center justify-between gap-3 {canvasErrorMessage
							? 'border-amber-500/30 bg-amber-500/[0.03]'
							: isCanvasSyncing
								? 'border-primary/50 bg-primary/[0.03] ring-1 ring-primary/20'
								: 'border-border/80 bg-card/60 hover:border-border'} {!isCanvasConnected ? 'opacity-65' : ''}"
					>
						<div class="flex items-center gap-3.5 min-w-0">
							<div
								class="size-10 sm:size-11 rounded-lg flex items-center justify-center shrink-0 bg-[#2d1215] border border-rose-900/40 text-rose-400"
							>
								<GraduationCapIcon class="size-5" />
							</div>

							<div class="min-w-0">
								<div class="text-sm font-medium text-foreground leading-tight">
									Canvas LMS
								</div>
								<div class="text-xs sm:text-[13px] leading-tight mt-1">
									{#if isCanvasSyncing}
										<span class="flex items-center gap-1.5 text-primary">
											<RefreshCwIcon class="size-3 animate-spin shrink-0" />
											<span>Syncing Canvas...</span>
										</span>
									{:else if canvasErrorMessage}
										<span class="text-amber-400 dark:text-amber-400">
											{canvasErrorMessage}
										</span>
									{:else if !isCanvasConnected}
										<span class="text-muted-foreground">
											Not connected · Configure Canvas in settings below
										</span>
									{:else if canvasLastSynced}
										<span class="flex items-center gap-1.5 text-muted-foreground">
											<span class="size-1.5 rounded-full bg-emerald-500 shrink-0"></span>
											<span>
												Synced {formatTime(canvasLastSynced)}{#if canvasTaskCount > 0} · {canvasTaskCount} {canvasTaskCount === 1 ? "task" : "tasks"}{/if}
											</span>
										</span>
									{:else}
										<span class="text-muted-foreground">Ready to sync</span>
									{/if}
								</div>
							</div>
						</div>

						<div class="shrink-0">
							{#if canvasErrorMessage}
								<Button
									variant="outline"
									size="sm"
									class="h-8 px-3.5 text-xs font-medium rounded-lg border-border/80 bg-background/50 hover:bg-muted text-foreground cursor-pointer"
									onclick={handleSyncCanvas}
									disabled={isAnySyncing}
								>
									Retry
								</Button>
							{:else}
								<Button
									variant="ghost"
									size="icon"
									class="size-8 rounded-lg border border-border/70 bg-background/40 hover:bg-muted text-muted-foreground hover:text-foreground cursor-pointer disabled:opacity-40"
									onclick={handleSyncCanvas}
									disabled={isAnySyncing || !isCanvasConnected}
									title="Sync Canvas LMS"
								>
									<RefreshCwIcon
										class="size-3.5 {isCanvasSyncing ? 'animate-spin text-primary' : ''}"
									/>
								</Button>
							{/if}
						</div>
					</div>

					<!-- Google Calendar Card -->
					<div
						class="rounded-xl border transition-all p-3.5 sm:p-4 flex items-center justify-between gap-3 {googleImportErrorMessage
							? 'border-amber-500/30 bg-amber-500/[0.03]'
							: isGoogleInSyncing
								? 'border-primary/50 bg-primary/[0.03] ring-1 ring-primary/20'
								: 'border-border/80 bg-card/60 hover:border-border'} {!isGoogleConnected ? 'opacity-65' : ''}"
					>
						<div class="flex items-center gap-3.5 min-w-0">
							<div
								class="size-10 sm:size-11 rounded-lg flex items-center justify-center shrink-0 bg-[#0c203a] border border-blue-900/40 text-blue-400"
							>
								<CalendarIcon class="size-5" />
							</div>

							<div class="min-w-0">
								<div class="text-sm font-medium text-foreground leading-tight">
									Google Calendar
								</div>
								<div class="text-xs sm:text-[13px] leading-tight mt-1">
									{#if isGoogleInSyncing}
										<span class="flex items-center gap-1.5 text-primary">
											<RefreshCwIcon class="size-3 animate-spin shrink-0" />
											<span>Syncing Google Calendar...</span>
										</span>
									{:else if googleImportErrorMessage}
										<span class="text-amber-400 dark:text-amber-400">
											{googleImportErrorMessage}
										</span>
									{:else if !isGoogleConnected}
										<span class="text-muted-foreground">
											Not connected · Sign in to Google in settings below
										</span>
									{:else if googleImportLastSynced}
										<span class="flex items-center gap-1.5 text-muted-foreground">
											<span class="size-1.5 rounded-full bg-emerald-500 shrink-0"></span>
											<span>
												Synced {formatTime(googleImportLastSynced)}{#if googleImportEventCount > 0} · {googleImportEventCount} {googleImportEventCount === 1 ? "event" : "events"}{/if}
											</span>
										</span>
									{:else}
										<span class="text-muted-foreground">Ready to sync</span>
									{/if}
								</div>
							</div>
						</div>

						<div class="shrink-0">
							{#if googleImportErrorMessage}
								<Button
									variant="outline"
									size="sm"
									class="h-8 px-3.5 text-xs font-medium rounded-lg border-border/80 bg-background/50 hover:bg-muted text-foreground cursor-pointer"
									onclick={handleSyncFromGoogle}
									disabled={isAnySyncing}
								>
									Retry
								</Button>
							{:else}
								<Button
									variant="ghost"
									size="icon"
									class="size-8 rounded-lg border border-border/70 bg-background/40 hover:bg-muted text-muted-foreground hover:text-foreground cursor-pointer disabled:opacity-40"
									onclick={handleSyncFromGoogle}
									disabled={isAnySyncing || !isGoogleConnected}
									title="Sync Google Calendar"
								>
									<RefreshCwIcon
										class="size-3.5 {isGoogleInSyncing ? 'animate-spin text-primary' : ''}"
									/>
								</Button>
							{/if}
						</div>
					</div>
				</div>
			</div>

			<!-- Step 2: Export Section -->
			<div class="space-y-2.5">
				<div
					class="h-7 sm:h-8 flex items-center text-sm font-medium text-foreground tracking-tight select-none"
				>
					Step 2 · export
				</div>

				<div class="space-y-2.5">
					<!-- Google Calendar Export Card -->
					<div
						class="rounded-xl border transition-all p-3.5 sm:p-4 flex items-center justify-between gap-3 {isWaitingForImports
							? 'border-dashed border-border/70 bg-card/30 opacity-90'
							: googleExportErrorMessage
								? 'border-amber-500/30 bg-amber-500/[0.03]'
								: isGoogleOutSyncing
									? 'border-primary/50 bg-primary/[0.03] ring-1 ring-primary/20'
									: 'border-border/80 bg-card/60 hover:border-border'} {!isGoogleConnected || !isGoogleExportEnabled ? 'opacity-65' : ''}"
					>
						<div class="flex items-center gap-3.5 min-w-0">
							<div
								class="size-10 sm:size-11 rounded-lg flex items-center justify-center shrink-0 {isWaitingForImports
									? 'bg-muted/40 border border-border/40 text-muted-foreground/70'
									: 'bg-[#0c203a] border border-blue-900/40 text-blue-400'}"
							>
								<UploadCloudIcon class="size-5" />
							</div>

							<div class="min-w-0">
								<div class="text-sm font-medium text-foreground leading-tight">
									Google Calendar export
								</div>
								<div class="text-xs sm:text-[13px] leading-tight mt-1">
									{#if isWaitingForImports}
										<span class="text-muted-foreground">
											Waiting for imports to finish
										</span>
									{:else if isGoogleOutSyncing}
										<span class="flex items-center gap-1.5 text-primary">
											<RefreshCwIcon class="size-3 animate-spin shrink-0" />
											<span>Exporting deadlines to Google Calendar...</span>
										</span>
									{:else if googleExportErrorMessage}
										<span class="text-amber-400 dark:text-amber-400">
											{googleExportErrorMessage}
										</span>
									{:else if !isGoogleConnected}
										<span class="text-muted-foreground">
											Not connected · Sign in to Google in settings below
										</span>
									{:else if !isGoogleExportEnabled}
										<span class="text-muted-foreground">
											Export disabled in settings
										</span>
									{:else if googleExportLastSynced}
										<span class="flex items-center gap-1.5 text-muted-foreground">
											<span class="size-1.5 rounded-full bg-emerald-500 shrink-0"></span>
											<span>
												Synced {formatTime(googleExportLastSynced)}{#if googleExportEventCount > 0} · {googleExportEventCount} {googleExportEventCount === 1 ? "event" : "events"}{/if}
											</span>
										</span>
									{:else}
										<span class="text-muted-foreground">Ready to export</span>
									{/if}
								</div>
							</div>
						</div>

						<div class="shrink-0">
							{#if isWaitingForImports}
								<!-- Hidden/Disabled while waiting -->
							{:else if googleExportErrorMessage}
								<Button
									variant="outline"
									size="sm"
									class="h-8 px-3.5 text-xs font-medium rounded-lg border-border/80 bg-background/50 hover:bg-muted text-foreground cursor-pointer"
									onclick={handleSyncToGoogle}
									disabled={isAnySyncing}
								>
									Retry
								</Button>
							{:else}
								<Button
									variant="ghost"
									size="icon"
									class="size-8 rounded-lg border border-border/70 bg-background/40 hover:bg-muted text-muted-foreground hover:text-foreground cursor-pointer disabled:opacity-40"
									onclick={handleSyncToGoogle}
									disabled={isAnySyncing || !isGoogleConnected || !isGoogleExportEnabled}
									title="Export to Google Calendar"
								>
									<RefreshCwIcon
										class="size-3.5 {isGoogleOutSyncing ? 'animate-spin text-primary' : ''}"
									/>
								</Button>
							{/if}
						</div>
					</div>
				</div>
			</div>
		</div>
	</div>

	<!-- Status / Success Feedback readout -->
	{#if syncController.syncAllMessage}
		<div
			class="flex items-center gap-2 text-xs text-emerald-600 dark:text-emerald-400 bg-emerald-500/10 border border-emerald-500/20 rounded-md px-3 py-2"
			in:fade={{ duration: 150 }}
		>
			<CheckIcon class="size-3.5 shrink-0" />
			<span>{syncController.syncAllMessage}</span>
		</div>
	{/if}
</div>
