<script lang="ts">
	import { canvasState } from "$lib/canvasState.svelte";
	import { googleCalendarState } from "$lib/googleCalendarState.svelte";
	import { Button } from "$lib/components/ui/button";
	import RefreshCwIcon from "@lucide/svelte/icons/refresh-cw";
	import DownloadCloudIcon from "@lucide/svelte/icons/download-cloud";
	import UploadCloudIcon from "@lucide/svelte/icons/upload-cloud";
	import GraduationCapIcon from "@lucide/svelte/icons/graduation-cap";
	import CalendarIcon from "@lucide/svelte/icons/calendar";
	import SparklesIcon from "@lucide/svelte/icons/sparkles";
	import CheckIcon from "@lucide/svelte/icons/check";
	import AlertCircleIcon from "@lucide/svelte/icons/alert-circle";
	import { fade } from "svelte/transition";

	let isSyncingAll = $state(false);
	let syncAllMessage = $state<string | null>(null);
	let syncAllError = $state<string | null>(null);

	const isAnySyncing = $derived(
		canvasState.isSyncing ||
		googleCalendarState.isSyncingFromGoogle ||
		googleCalendarState.isSyncingToGoogle ||
		isSyncingAll
	);

	function formatTime(date: Date | null): string {
		if (!date) return "Never";
		return date.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" });
	}

	async function handleSyncCanvas() {
		if (!canvasState.isConnected || isAnySyncing) return;
		try {
			syncAllError = null;
			await canvasState.syncCanvas();
		} catch (e) {
			console.error("Manual Canvas sync failed:", e);
		}
	}

	async function handleSyncFromGoogle() {
		if (!googleCalendarState.isConnected || isAnySyncing) return;
		try {
			syncAllError = null;
			await googleCalendarState.syncFromGoogle();
		} catch (e) {
			console.error("Manual Google sync failed:", e);
		}
	}

	async function handleSyncToGoogle() {
		if (!googleCalendarState.isConnected || isAnySyncing) return;
		try {
			syncAllError = null;
			await googleCalendarState.syncToGoogle();
		} catch (e) {
			console.error("Manual sync to Google failed:", e);
		}
	}

	async function handleSyncAll() {
		if (isAnySyncing) return;
		isSyncingAll = true;
		syncAllMessage = null;
		syncAllError = null;

		const results: string[] = [];
		const errors: string[] = [];

		try {
			// 1. Sync from Canvas (if connected)
			if (canvasState.isConnected) {
				try {
					const cRes = await canvasState.syncCanvas();
					results.push(`Canvas (${cRes?.tasksSynced ?? "all"} tasks)`);
				} catch (e: any) {
					errors.push(`Canvas: ${e.message || "failed"}`);
				}
			}

			// 2. Sync info from Google Calendar (if connected)
			if (googleCalendarState.isConnected) {
				try {
					const gFromRes = await googleCalendarState.syncFromGoogle();
					results.push(`Google Inbound (${gFromRes?.calendarCount ?? 0} cals, ${gFromRes?.eventCount ?? 0} events)`);
				} catch (e: any) {
					errors.push(`Google Inbound: ${e.message || "failed"}`);
				}
			}

			// 3. Sync info to Google Calendar (if connected)
			if (googleCalendarState.isConnected) {
				try {
					const gToRes = await googleCalendarState.syncToGoogle();
					results.push(`Google Outbound (${gToRes?.syncedCount ?? 0} pushed)`);
				} catch (e: any) {
					errors.push(`Google Outbound: ${e.message || "failed"}`);
				}
			}

			if (results.length > 0) {
				syncAllMessage = `Synced successfully: ${results.join(", ")}`;
			}
			if (errors.length > 0) {
				syncAllError = errors.join("; ");
			}
		} catch (err: any) {
			syncAllError = err?.message || "Sync all encountered an issue";
		} finally {
			isSyncingAll = false;
		}
	}
</script>

<div class="rounded-xl border border-border bg-card p-6 shadow-xs space-y-5">
	<!-- Header -->
	<div class="flex items-center justify-between">
		<div class="flex items-center gap-3">
			<div
				class="size-9 rounded-lg bg-primary/10 flex items-center justify-center text-primary shrink-0"
			>
				<RefreshCwIcon class="size-5 {isAnySyncing ? 'animate-spin' : ''}" />
			</div>
			<div>
				<h2 class="text-base font-normal tracking-wide text-card-foreground leading-none">
					Data Synchronization
				</h2>
				<p class="text-xs text-muted-foreground mt-1">
					Manage incoming and outgoing data between Canvas LMS, Google Calendar, and Lasso.
				</p>
			</div>
		</div>

		<!-- Status Badge -->
		{#if canvasState.isConnected && googleCalendarState.isConnected}
			<span class="inline-flex items-center rounded-full bg-emerald-500/10 px-2.5 py-1 text-xs font-medium text-emerald-500 leading-none">
				Canvas & Google Connected
			</span>
		{:else if canvasState.isConnected}
			<span class="inline-flex items-center rounded-full bg-blue-500/10 px-2.5 py-1 text-xs font-medium text-blue-500 leading-none">
				Canvas Connected
			</span>
		{:else if googleCalendarState.isConnected}
			<span class="inline-flex items-center rounded-full bg-blue-500/10 px-2.5 py-1 text-xs font-medium text-blue-500 leading-none">
				Google Connected
			</span>
		{:else}
			<span class="inline-flex items-center rounded-full bg-muted px-2.5 py-1 text-xs font-medium text-muted-foreground leading-none">
				No Services Connected
			</span>
		{/if}
	</div>

	<!-- Action Feedback Messages -->
	{#if syncAllMessage}
		<div
			class="flex items-center gap-2 text-xs text-emerald-500 bg-emerald-500/10 border border-emerald-500/20 rounded-md px-3 py-2"
			in:fade={{ duration: 150 }}
		>
			<CheckIcon class="size-4 shrink-0" />
			<span>{syncAllMessage}</span>
		</div>
	{/if}

	{#if syncAllError}
		<div
			class="flex items-center gap-2 text-xs text-destructive bg-destructive/10 border border-destructive/20 rounded-md px-3 py-2"
			in:fade={{ duration: 150 }}
		>
			<AlertCircleIcon class="size-4 shrink-0" />
			<span>{syncAllError}</span>
		</div>
	{/if}

	<!-- 4 Sync Buttons Grid -->
	<div class="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-1">
		<!-- 1. Sync from Canvas -->
		<div class="rounded-lg border border-border/80 bg-muted/25 p-3.5 flex flex-col justify-between gap-3 transition-colors hover:border-border">
			<div class="space-y-1">
				<div class="flex items-center gap-2">
					<div class="size-6 rounded bg-red-500/10 text-red-500 flex items-center justify-center shrink-0">
						<GraduationCapIcon class="size-3.5" />
					</div>
					<h3 class="text-xs font-semibold text-card-foreground">Canvas LMS</h3>
				</div>
				<p class="text-[11px] text-muted-foreground leading-relaxed">
					Pull courses, assignments, submissions, and grades from Canvas into Lasso.
				</p>
			</div>

			<div class="flex items-center justify-between gap-2 pt-1 border-t border-border/40">
				<span class="text-[10px] text-muted-foreground/80 font-mono truncate">
					{canvasState.lastSynced ? `Synced ${formatTime(canvasState.lastSynced)}` : 'Not synced yet'}
				</span>
				<Button
					variant="outline"
					size="sm"
					class="text-xs h-7 px-2.5 cursor-pointer gap-1.5 shrink-0"
					onclick={handleSyncCanvas}
					disabled={!canvasState.isConnected || isAnySyncing}
				>
					{#if canvasState.isSyncing}
						<RefreshCwIcon class="size-3 animate-spin" />
						<span>Syncing...</span>
					{:else}
						<DownloadCloudIcon class="size-3" />
						<span>Sync from Canvas</span>
					{/if}
				</Button>
			</div>
		</div>

		<!-- 2. Sync info from Google Calendar -->
		<div class="rounded-lg border border-border/80 bg-muted/25 p-3.5 flex flex-col justify-between gap-3 transition-colors hover:border-border">
			<div class="space-y-1">
				<div class="flex items-center gap-2">
					<div class="size-6 rounded bg-blue-500/10 text-blue-500 flex items-center justify-center shrink-0">
						<CalendarIcon class="size-3.5" />
					</div>
					<h3 class="text-xs font-semibold text-card-foreground">Google Calendar (Inbound)</h3>
				</div>
				<p class="text-[11px] text-muted-foreground leading-relaxed">
					Fetch personal Google calendars and events for viewing on Lasso's calendar grid.
				</p>
			</div>

			<div class="flex items-center justify-between gap-2 pt-1 border-t border-border/40">
				<span class="text-[10px] text-muted-foreground/80 font-mono truncate">
					{googleCalendarState.lastSyncedFromGoogle ? `Synced ${formatTime(googleCalendarState.lastSyncedFromGoogle)}` : 'Not synced yet'}
				</span>
				<Button
					variant="outline"
					size="sm"
					class="text-xs h-7 px-2.5 cursor-pointer gap-1.5 shrink-0"
					onclick={handleSyncFromGoogle}
					disabled={!googleCalendarState.isConnected || isAnySyncing}
				>
					{#if googleCalendarState.isSyncingFromGoogle}
						<RefreshCwIcon class="size-3 animate-spin" />
						<span>Fetching...</span>
					{:else}
						<DownloadCloudIcon class="size-3" />
						<span>Sync info from Google Calendar</span>
					{/if}
				</Button>
			</div>
		</div>

		<!-- 3. Sync info to Google Calendar -->
		<div class="rounded-lg border border-border/80 bg-muted/25 p-3.5 flex flex-col justify-between gap-3 transition-colors hover:border-border">
			<div class="space-y-1">
				<div class="flex items-center gap-2">
					<div class="size-6 rounded bg-indigo-500/10 text-indigo-500 flex items-center justify-center shrink-0">
						<UploadCloudIcon class="size-3.5" />
					</div>
					<h3 class="text-xs font-semibold text-card-foreground">Google Calendar (Outbound)</h3>
				</div>
				<p class="text-[11px] text-muted-foreground leading-relaxed">
					Push Lasso coursework tasks and deadlines to your dedicated "Lasso" Google Calendar.
				</p>
			</div>

			<div class="flex items-center justify-between gap-2 pt-1 border-t border-border/40">
				<span class="text-[10px] text-muted-foreground/80 font-mono truncate">
					{googleCalendarState.lastSyncedToGoogle ? `Synced ${formatTime(googleCalendarState.lastSyncedToGoogle)}` : 'Not synced yet'}
				</span>
				<Button
					variant="outline"
					size="sm"
					class="text-xs h-7 px-2.5 cursor-pointer gap-1.5 shrink-0"
					onclick={handleSyncToGoogle}
					disabled={!googleCalendarState.isConnected || isAnySyncing}
				>
					{#if googleCalendarState.isSyncingToGoogle}
						<RefreshCwIcon class="size-3 animate-spin" />
						<span>Pushing...</span>
					{:else}
						<UploadCloudIcon class="size-3" />
						<span>Sync info to Google Calendar</span>
					{/if}
				</Button>
			</div>
		</div>

		<!-- 4. Sync All -->
		<div class="rounded-lg border border-primary/30 bg-primary/5 p-3.5 flex flex-col justify-between gap-3 transition-colors hover:border-primary/50">
			<div class="space-y-1">
				<div class="flex items-center gap-2">
					<div class="size-6 rounded bg-primary/20 text-primary flex items-center justify-center shrink-0">
						<SparklesIcon class="size-3.5" />
					</div>
					<h3 class="text-xs font-semibold text-foreground">Sync All Services</h3>
				</div>
				<p class="text-[11px] text-muted-foreground leading-relaxed">
					Run the complete pipeline: pull latest Canvas tasks, fetch Google events, and export to Google Calendar.
				</p>
			</div>

			<div class="flex items-center justify-between gap-2 pt-1 border-t border-primary/20">
				<span class="text-[10px] text-primary/80 font-medium truncate">
					{isSyncingAll ? 'Synchronizing all services...' : 'Full pipeline synchronization'}
				</span>
				<Button
					size="sm"
					class="text-xs h-7 px-3 cursor-pointer gap-1.5 shrink-0 bg-primary text-primary-foreground shadow-xs hover:bg-primary/90"
					onclick={handleSyncAll}
					disabled={(!canvasState.isConnected && !googleCalendarState.isConnected) || isAnySyncing}
				>
					{#if isSyncingAll}
						<RefreshCwIcon class="size-3 animate-spin" />
						<span>Syncing All...</span>
					{:else}
						<RefreshCwIcon class="size-3" />
						<span>Sync all</span>
					{/if}
				</Button>
			</div>
		</div>
	</div>
</div>
