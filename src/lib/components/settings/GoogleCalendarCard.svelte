<script lang="ts">
	import { pb, POCKETBASE_URL } from "$lib/pocketbase";
	import { authState } from "$lib/authState.svelte";
	import { syncState } from "$lib/syncState.svelte";
	import { calendarVisibilityState } from "$lib/calendarVisibilityState.svelte";
	import { dataState } from "$lib/dataState/dataState.svelte";
	import {
		getGoogleCalendars,
		resolveCalendarColor,
	} from "$lib/dataState/calendarQueries.svelte";
	import type { CalendarRecord } from "$lib/dataState/dataRecordInterfaces";
	import { Button } from "$lib/components/ui/button";
	import CalendarIcon from "@lucide/svelte/icons/calendar";
	import RefreshCwIcon from "@lucide/svelte/icons/refresh-cw";
	import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
	import TagIcon from "@lucide/svelte/icons/tag";
	import CheckIcon from "@lucide/svelte/icons/check";
	import RotateCcwIcon from "@lucide/svelte/icons/rotate-ccw";
	import AlertTriangleIcon from "@lucide/svelte/icons/alert-triangle";
	import UploadCloudIcon from "@lucide/svelte/icons/upload-cloud";
	import EyeIcon from "@lucide/svelte/icons/eye";
	import EyeOffIcon from "@lucide/svelte/icons/eye-off";
	import SparklesIcon from "@lucide/svelte/icons/sparkles";
	import Trash2Icon from "@lucide/svelte/icons/trash-2";
	import { fade, slide, scale } from "svelte/transition";
	import { SquareSwitch } from "$lib/components/ui/square-switch";

	let showDisconnectModal = $state(false);

	const isConnected = $derived(Boolean(authState.record?.google_connected));
	const googleCalendars = $derived(getGoogleCalendars());
	const readOnlyCalendars = $derived(
		googleCalendars.filter(
			(c) => (c.nickname || c.name).toLowerCase() !== "lasso",
		),
	);

	// Detail & nickname state
	let expandedCalendarId = $state<string | null>(null);
	let nicknameInput = $state("");
	let isSavingNickname = $state(false);
	let nicknameSuccessId = $state<string | null>(null);
	let isPurging = $state(false);
	let isExportEnabled = $state(
		typeof window !== "undefined"
			? localStorage.getItem("lasso_google_export_enabled") !== "false"
			: true,
	);

	$effect(() => {
		if (typeof window !== "undefined") {
			localStorage.setItem(
				"lasso_google_export_enabled",
				String(isExportEnabled),
			);
		}
	});

	const swatchColors = [
		"#2563eb", // blue
		"#16a34a", // green
		"#9333ea", // purple
		"#ea580c", // orange
		"#0d9488", // teal
		"#db2777", // pink
		"#b45309", // brown
		"#0284c7", // sky
	];

	function getSwatch(idx: number, fallback?: string): string {
		if (fallback && fallback.startsWith("#")) return fallback;
		return swatchColors[idx % swatchColors.length];
	}

	function toggleExpand(calendar: CalendarRecord) {
		if (expandedCalendarId === calendar.id) {
			expandedCalendarId = null;
		} else {
			expandedCalendarId = calendar.id;
			nicknameInput = calendar.nickname || "";
			nicknameSuccessId = null;
		}
	}

	async function saveNickname(e: SubmitEvent, calendarId: string) {
		e.preventDefault();
		isSavingNickname = true;
		try {
			const trimmed = nicknameInput.trim();
			await dataState.updateCalendar(calendarId, { nickname: trimmed });
			await fetch(`${POCKETBASE_URL}/api/calendar/nickname`, {
				method: "POST",
				headers: {
					"Content-Type": "application/json",
					Authorization: pb.authStore.token
						? `Bearer ${pb.authStore.token}`
						: "",
				},
				body: JSON.stringify({ calendarId, nickname: trimmed }),
			}).catch(() => {});
			nicknameSuccessId = calendarId;
			setTimeout(() => {
				if (nicknameSuccessId === calendarId) nicknameSuccessId = null;
			}, 3000);
		} catch (err) {
			console.error("Failed to save calendar nickname:", err);
		} finally {
			isSavingNickname = false;
		}
	}

	async function clearNickname(calendarId: string) {
		isSavingNickname = true;
		try {
			await dataState.updateCalendar(calendarId, { nickname: "" });
			await fetch(`${POCKETBASE_URL}/api/calendar/nickname`, {
				method: "POST",
				headers: {
					"Content-Type": "application/json",
					Authorization: pb.authStore.token
						? `Bearer ${pb.authStore.token}`
						: "",
				},
				body: JSON.stringify({ calendarId, nickname: "" }),
			}).catch(() => {});
			nicknameInput = "";
			nicknameSuccessId = calendarId;
			setTimeout(() => {
				if (nicknameSuccessId === calendarId) nicknameSuccessId = null;
			}, 3000);
		} catch (err) {
			console.error("Failed to clear calendar nickname:", err);
		} finally {
			isSavingNickname = false;
		}
	}

	async function handleConnect() {
		try {
			await authState.loginWithGoogle();
			await syncState.syncFromGoogle().catch(() => {});
			await dataState.refresh();
		} catch (err) {
			console.error("Failed to connect Google Calendar:", err);
		}
	}

	async function handleDisconnect() {
		showDisconnectModal = false;
		try {
			await fetch(`${POCKETBASE_URL}/api/google/disconnect`, {
				method: "POST",
				headers: {
					Authorization: pb.authStore.token
						? `Bearer ${pb.authStore.token}`
						: "",
				},
			}).catch(() => {});

			if (pb.authStore.record?.id) {
				await pb.collection("users").update(pb.authStore.record.id, {
					google_connected: false,
					google_access_token: "",
					google_refresh_token: "",
				});
			}
			await pb.collection("users").authRefresh();
			await dataState.refresh();
		} catch (e) {
			console.error("Disconnect Google error:", e);
		}
	}

	async function handleSyncToGoogle() {
		try {
			await syncState.syncToGoogle();
		} catch (err) {
			console.error("Sync to Google Calendar failed:", err);
		}
	}

	async function handlePurgeLassoCalendar() {
		if (isPurging) return;
		isPurging = true;
		try {
			const resp = await fetch(`${POCKETBASE_URL}/api/google/lasso/purge`, {
				method: "POST",
				headers: {
					"Content-Type": "application/json",
					Authorization: pb.authStore.token
						? `Bearer ${pb.authStore.token}`
						: "",
				},
			});
			const data = await resp.json();
			if (!resp.ok || !data.success) {
				throw new Error(
					data.message || "Failed to purge Lasso Google calendar",
				);
			}
			await dataState.refresh();
		} catch (err) {
			console.error("Purge Lasso Calendar failed:", err);
		} finally {
			isPurging = false;
		}
	}
</script>

<!-- Google Calendar integration status with grouped calendars -->
<div class="rounded-xl border border-border bg-card p-6 shadow-xs space-y-2">
	<div class="flex items-center justify-between">
		<div class="flex items-center gap-3">
			<div>
				<h2
					class="text-base font-normal tracking-wide text-card-foreground leading-none"
				>
					Google Calendar
				</h2>
				<span class="text-xs py-1 text-muted-foreground">
					Personal calendars • Sync calendar to google
				</span>
			</div>
		</div>
	</div>
	<hr />

	{#if isConnected}
		<!-- Account Header -->
		<div
			class="flex flex-col sm:flex-row sm:items-center justify-between gap-1 text-xs"
		>
			<div>
				<span class="text-muted-foreground">Account: </span>
				<span class="font-medium font-mono text-foreground">
					{authState.record?.google_email ||
						authState.user?.email ||
						"Google User"}
				</span>
			</div>
			<div>
				<span class="text-muted-foreground">Calendars Available: </span>
				<span class="font-medium text-foreground">
					{googleCalendars.length}
				</span>
			</div>
		</div>

		<!-- PERSONAL READ-ONLY CALENDARS LIST -->
		{#if readOnlyCalendars.length > 0}
			<div class="pt-2 space-y-3">
				<div class="flex items-center justify-between">
					<span class="text-xs font-medium text-foreground"
						>Personal Calendars ({readOnlyCalendars.length})</span
					>
				</div>

				<div class="space-y-2 pt-1">
					{#each readOnlyCalendars as calendar, idx (calendar.id)}
						{@render calendarItem(calendar, getSwatch(idx, calendar.color))}
					{/each}
				</div>
			</div>
		{/if}

		<!-- DEDICATED LASSO CALENDAR SYNC CARD -->
		<div
			class="flex flex-col sm:flex-row sm:items-center justify-between p-2.5 rounded-lg border border-border bg-card/60 overflow-hidden transition-all duration-200"
		>
			<div class="flex items-start gap-3">
				<div>
					<div class="flex items-center gap-2 pb-1">
						<span class="size-2.5 rounded-xs shrink-0 shadow-2xs bg-primary"
						></span>

						<span class="text-xs text-foreground">Lasso Calendar Sync</span>
					</div>
					<p class="text-xs text-muted-foreground mt-0.5 lg:w-3/4">
						When enabled all Canvas coursework and deadlines are pushed to this
						calendar in Google, tagged by course.
					</p>
				</div>
			</div>

			<div class="flex items-center gap-2.5 shrink-0 pt-2 sm:pt-0">
				<SquareSwitch
					id="lasso-calendar-sync-switch"
					bind:checked={isExportEnabled}
					label="Enabled"
				/>
			</div>
		</div>
		<hr />
		<div
			class="flex flex-col sm:flex-row sm:items-center justify-between gap-1 text-[11px] text-muted-foreground pt-1"
		>
			{#if dataState.googleExportSyncedAt}
				<span
					>Last synced: {new Date(
						dataState.googleExportSyncedAt,
					).toLocaleTimeString([], {
						hour: "numeric",
						minute: "2-digit",
					})}</span
				>
			{/if}
			<Button
				variant="outline"
				size="sm"
				class="text-xs text-destructive hover:text-destructive hover:bg-destructive/10 border-border cursor-pointer"
				onclick={() => (showDisconnectModal = true)}
			>
				Disconnect Google Calendar
			</Button>
		</div>
	{:else}
		<div
			class="mt-4 pt-4 border-t border-border flex flex-col sm:flex-row sm:items-center justify-between gap-3"
		>
			<p class="text-xs text-muted-foreground">
				{#if authState.user?.email}
					Signed in as <span class="font-medium font-mono text-foreground"
						>{authState.user.email}</span
					>. Connect to create your dedicated Lasso calendar and view personal
					events.
				{:else}
					Sign in with Google to synchronize your calendars with Lasso.
				{/if}
			</p>
			<div class="flex items-center gap-2">
				<Button
					variant="outline"
					size="sm"
					class="text-xs cursor-pointer gap-1.5"
					onclick={handleConnect}
					disabled={authState.isLoading}
				>
					{#if authState.isLoading}
						<RefreshCwIcon class="size-3.5 animate-spin" />
						<span>Connecting...</span>
					{:else}
						<CalendarIcon class="size-3.5" />
						<span>Connect Google Calendar</span>
					{/if}
				</Button>
			</div>
		</div>
	{/if}
</div>

<!-- Confirmation Modal for Disconnecting Google Calendar -->
{#if showDisconnectModal}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div
		class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/65 backdrop-blur-xs"
		transition:fade={{ duration: 150 }}
		onclick={(e) => {
			if (e.target === e.currentTarget) showDisconnectModal = false;
		}}
		onkeydown={(e) => {
			if (e.key === "Escape") showDisconnectModal = false;
		}}
		tabindex="-1"
		role="alertdialog"
		aria-modal="true"
		aria-labelledby="disconnect-google-title"
		aria-describedby="disconnect-google-desc"
	>
		<div
			class="w-full max-w-md rounded-xl border border-border bg-card p-6 shadow-2xl space-y-4"
			transition:scale={{ start: 0.95, duration: 150 }}
		>
			<div class="flex items-start gap-3.5">
				<div
					class="size-10 rounded-full bg-destructive/15 flex items-center justify-center text-destructive shrink-0 mt-0.5"
				>
					<AlertTriangleIcon class="size-5" />
				</div>
				<div class="space-y-1">
					<h3
						id="disconnect-google-title"
						class="text-base font-semibold text-card-foreground leading-snug"
					>
						Disconnect Google Calendar?
					</h3>
					<p
						id="disconnect-google-desc"
						class="text-xs text-muted-foreground leading-relaxed"
					>
						Are you sure you want to disconnect Google Calendar? This will clear
						your stored Google credentials. Your personal calendars remain
						untouched.
					</p>
				</div>
			</div>

			<div
				class="flex items-center justify-end gap-2.5 pt-2 border-t border-border/50"
			>
				<Button
					type="button"
					variant="outline"
					size="sm"
					class="text-xs cursor-pointer"
					onclick={() => (showDisconnectModal = false)}
				>
					Cancel
				</Button>
				<Button
					type="button"
					variant="destructive"
					size="sm"
					class="text-xs cursor-pointer gap-1.5"
					onclick={handleDisconnect}
				>
					Disconnect
				</Button>
			</div>
		</div>
	</div>
{/if}

<!-- Reusable snippet for calendar item card with interactive details and nickname editing -->
{#snippet calendarItem(calendar: CalendarRecord, swatchColor: string)}
	{@const isExpanded = expandedCalendarId === calendar.id}
	{@const isHiddenInSidebar = calendarVisibilityState.isHiddenInSidebar(
		calendar.id,
		calendar.name,
	)}
	{@const calColor = resolveCalendarColor(calendar, swatchColor)}
	<div
		class="rounded-lg border border-border bg-card/60 overflow-hidden transition-all duration-200 {isExpanded
			? 'border-primary/50 shadow-xs ring-1 ring-primary/20'
			: 'hover:border-border/80'}"
	>
		<!-- Clickable header card with right-aligned grey outlined visibility button -->
		<div
			class="w-full p-2.5 flex items-center justify-between gap-2.5 select-none"
		>
			<button
				type="button"
				class="flex items-center gap-2.5 min-w-0 flex-1 cursor-pointer text-left group bg-transparent border-none p-0"
				onclick={() => toggleExpand(calendar)}
				aria-expanded={isExpanded}
			>
				<span
					class="size-2.5 rounded-xs shrink-0 shadow-2xs transition-opacity {isHiddenInSidebar
						? 'opacity-40'
						: ''}"
					style="background-color: {calColor};"
				></span>
				<span
					class="font-medium truncate text-xs leading-none {isHiddenInSidebar
						? 'text-muted-foreground line-through opacity-75'
						: 'text-foreground'}"
				>
					{calendar.nickname || calendar.name}
				</span>
			</button>

			<div class="flex items-center gap-2 shrink-0">
				<!-- Grey outlined visibility button matching CanvasCard -->
				<button
					type="button"
					class="size-7 rounded-md border border-border/70 text-muted-foreground hover:text-foreground hover:border-foreground/40 hover:bg-muted/40 flex items-center justify-center transition-colors cursor-pointer {isHiddenInSidebar
						? 'opacity-50'
						: ''}"
					onclick={(e) => {
						e.stopPropagation();
						calendarVisibilityState.toggleSidebarVisibility(
							calendar.id,
							calendar.name,
						);
					}}
					title={isHiddenInSidebar
						? "Hidden in sidebar (click to show)"
						: "Visible in sidebar (click to hide)"}
					aria-label={isHiddenInSidebar ? "Show in sidebar" : "Hide in sidebar"}
				>
					{#if isHiddenInSidebar}
						<EyeOffIcon class="size-3.5" />
					{:else}
						<EyeIcon class="size-3.5" />
					{/if}
				</button>

				<button
					type="button"
					class="p-1 text-muted-foreground hover:text-foreground cursor-pointer bg-transparent border-none"
					onclick={() => toggleExpand(calendar)}
					aria-label="Toggle details"
				>
					<ChevronDownIcon
						class="size-3.5 transition-transform duration-200 {isExpanded
							? 'rotate-180 text-foreground'
							: ''}"
					/>
				</button>
			</div>
		</div>

		<!-- Expanded detail panel: IDs and local nickname editor -->
		{#if isExpanded}
			<div
				class="px-3 pb-3 pt-1 border-t border-border/50 bg-muted/20 space-y-3"
				transition:slide={{ duration: 180 }}
			>
				<!-- Grid of calendar metadata -->
				<div class="grid grid-cols-2 sm:grid-cols-3 gap-2.5 pt-1 text-[11px]">
					<div class="flex flex-col gap-0.5 col-span-2 sm:col-span-1">
						<span class="text-muted-foreground">Calendar ID</span>
						<span
							class="font-mono text-foreground font-medium truncate text-[10px]"
							title={calendar.id}
						>
							{calendar.id}
						</span>
					</div>

					{#if calendar.calendar_id}
						<div class="flex flex-col gap-0.5 col-span-2 sm:col-span-2">
							<span class="text-muted-foreground">Google Calendar ID</span>
							<span
								class="text-muted-foreground font-mono text-[10px] truncate"
								title={calendar.calendar_id}
							>
								{calendar.calendar_id}
							</span>
						</div>
					{/if}
				</div>

				<!-- Local Nickname Editor -->
				<div class="pt-2 border-t border-border/40">
					<form
						onsubmit={(e) => saveNickname(e, calendar.id)}
						class="space-y-2"
					>
						<div class="flex items-center justify-between">
							<label
								for="cal-nickname-{calendar.id}"
								class="text-[11px] font-medium text-muted-foreground flex items-center gap-1"
							>
								<TagIcon class="size-3 text-primary" />
								Calendar Nickname
							</label>
							{#if nicknameSuccessId === calendar.id}
								<span
									class="text-[10px] text-emerald-400 font-medium inline-flex items-center gap-0.5"
									in:fade={{ duration: 150 }}
								>
									<CheckIcon class="size-3" /> Saved!
								</span>
							{/if}
						</div>

						<div class="flex gap-2">
							<input
								id="cal-nickname-{calendar.id}"
								type="text"
								bind:value={nicknameInput}
								placeholder="e.g. Personal, Work Shifts, Family..."
								maxlength="59"
								class="flex-1 rounded-md border border-input bg-background px-2.5 py-1 text-xs text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
							/>
							<Button
								type="submit"
								size="sm"
								class="h-7 px-2.5 text-xs cursor-pointer gap-1"
								disabled={isSavingNickname || !nicknameInput.trim()}
							>
								{isSavingNickname ? "Saving..." : "Save Nickname"}
							</Button>
							{#if calendar.nickname}
								<Button
									type="button"
									variant="ghost"
									size="sm"
									class="h-7 px-2 text-xs text-muted-foreground hover:text-foreground cursor-pointer gap-1"
									onclick={() => clearNickname(calendar.id)}
									title="Reset to default calendar name"
									disabled={isSavingNickname}
								>
									<RotateCcwIcon class="size-3" />
									<span>Reset</span>
								</Button>
							{/if}
						</div>
						<p class="text-[10px] text-muted-foreground">
							Stored in your Lasso workspace. Customizes the calendar title on
							your agenda and sidebar.
						</p>
					</form>
				</div>
			</div>
		{/if}
	</div>
{/snippet}

<style>
	h2 {
		margin: 0px;
	}
</style>
