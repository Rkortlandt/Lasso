<script lang="ts">
	import { pb, POCKETBASE_URL } from "$lib/pocketbase";
	import { authState } from "$lib/authState.svelte";
	import { syncState } from "$lib/syncState.svelte";
	import { Button } from "$lib/components/ui/button";
	import BookOpenIcon from "@lucide/svelte/icons/book-open";
	import CalendarIcon from "@lucide/svelte/icons/calendar";
	import RefreshCwIcon from "@lucide/svelte/icons/refresh-cw";
	import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
	import TagIcon from "@lucide/svelte/icons/tag";
	import CheckIcon from "@lucide/svelte/icons/check";
	import RotateCcwIcon from "@lucide/svelte/icons/rotate-ccw";
	import AlertTriangleIcon from "@lucide/svelte/icons/alert-triangle";
	import EyeIcon from "@lucide/svelte/icons/eye";
	import EyeOffIcon from "@lucide/svelte/icons/eye-off";
	import { calendarVisibilityState } from "$lib/calendarVisibilityState.svelte";
	import { fade, slide, scale } from "svelte/transition";
	import { dataState } from "$lib/dataState/dataState.svelte";
	import {
		getCourseworkCalendars,
		resolveCalendarColor,
	} from "$lib/dataState/calendarQueries.svelte";
	import { type CalendarRecord } from "$lib/dataState/dataRecordInterfaces";

	let showConnectForm = $state(false);
	let showDisconnectModal = $state(false);
	let canvasUrl = $state(
		authState.record?.canvas_url || "https://canvas.instructure.com",
	);
	let canvasToken = $state("");
	let isLoading = $state(false);

	const isConnected = $derived(Boolean(authState.record?.canvas_connected));
	const courses = $derived(getCourseworkCalendars());

	// Section detail & nickname state
	let expandedCourseId = $state<string | null>(null);
	let nicknameInput = $state("");
	let isSavingNickname = $state(false);
	let nicknameSuccessId = $state<string | null>(null);

	const swatchColors = [
		"#16a34a", // green
		"#9333ea", // purple
		"#b45309", // brown/olive
		"#15803d", // dark green
		"#2563eb", // blue
		"#db2777", // pink
		"#0d9488", // teal
		"#ea580c", // orange
	];

	function getSwatch(idx: number): string {
		return swatchColors[idx % swatchColors.length];
	}

	function toggleExpand(course: CalendarRecord) {
		if (expandedCourseId === course.id) {
			expandedCourseId = null;
		} else {
			expandedCourseId = course.id;
			nicknameInput = course.nickname || "";
			nicknameSuccessId = null;
		}
	}

	async function saveNickname(e: SubmitEvent, courseId: string) {
		e.preventDefault();
		isSavingNickname = true;
		try {
			const trimmedNickname = nicknameInput.trim();
			await dataState.updateCalendar(courseId, { nickname: trimmedNickname });
			await fetch(`${POCKETBASE_URL}/api/calendar/nickname`, {
				method: "POST",
				headers: {
					"Content-Type": "application/json",
					Authorization: pb.authStore.token ? `Bearer ${pb.authStore.token}` : "",
				},
				body: JSON.stringify({ courseId, nickname: trimmedNickname }),
			}).catch(() => {});
			nicknameSuccessId = courseId;
			setTimeout(() => {
				if (nicknameSuccessId === courseId) nicknameSuccessId = null;
			}, 3000);
		} catch (err) {
			console.error("Failed to save nickname:", err);
		} finally {
			isSavingNickname = false;
		}
	}

	async function clearNickname(courseId: string) {
		isSavingNickname = true;
		try {
			await dataState.updateCalendar(courseId, { nickname: "" });
			await fetch(`${POCKETBASE_URL}/api/calendar/nickname`, {
				method: "POST",
				headers: {
					"Content-Type": "application/json",
					Authorization: pb.authStore.token ? `Bearer ${pb.authStore.token}` : "",
				},
				body: JSON.stringify({ courseId, nickname: "" }),
			}).catch(() => {});
			nicknameInput = "";
			nicknameSuccessId = courseId;
			setTimeout(() => {
				if (nicknameSuccessId === courseId) nicknameSuccessId = null;
			}, 3000);
		} catch (err) {
			console.error("Failed to clear nickname:", err);
		} finally {
			isSavingNickname = false;
		}
	}

	async function handleConnect(e: SubmitEvent) {
		e.preventDefault();
		if (!canvasToken.trim()) return;

		let cleanUrl = canvasUrl.trim();
		if (!cleanUrl) {
			cleanUrl = "https://canvas.instructure.com";
		} else if (
			!cleanUrl.startsWith("http://") &&
			!cleanUrl.startsWith("https://")
		) {
			cleanUrl = "https://" + cleanUrl;
		}
		cleanUrl = cleanUrl.replace(/\/+$/, "");
		canvasUrl = cleanUrl;

		isLoading = true;
		try {
			if (pb.authStore.record?.id) {
				await pb.collection("users").update(pb.authStore.record.id, {
					canvas_url: cleanUrl,
					canvas_token: canvasToken.trim(),
				});
			}

			const res = await fetch(`${POCKETBASE_URL}/api/canvas/verify`, {
				method: "POST",
				headers: {
					"Content-Type": "application/json",
					Authorization: pb.authStore.token ? `Bearer ${pb.authStore.token}` : "",
				},
				body: JSON.stringify({ canvasUrl: cleanUrl, canvasToken: canvasToken.trim() }),
			});
			const data = await res.json();
			if (!res.ok || !data.success) {
				throw new Error(data.message || data.error || "Failed to verify Canvas token");
			}

			await pb.collection("users").authRefresh();
			await syncState.syncCanvas().catch(() => {});
			await dataState.refresh();
			showConnectForm = false;
			canvasToken = "";
		} catch (err: any) {
			console.error("Canvas connect error:", err);
		} finally {
			isLoading = false;
		}
	}

	async function handleDisconnect() {
		showDisconnectModal = false;
		try {
			await fetch(`${POCKETBASE_URL}/api/canvas/disconnect`, {
				method: "POST",
				headers: {
					Authorization: pb.authStore.token ? `Bearer ${pb.authStore.token}` : "",
				},
			}).catch(() => {});

			if (pb.authStore.record?.id) {
				await pb.collection("users").update(pb.authStore.record.id, {
					canvas_url: "",
					canvas_token: "",
					canvas_connected: false,
					canvas_student_name: "",
				});
			}
			await pb.collection("users").authRefresh();
			await dataState.refresh();
		} catch (err) {
			console.error("Disconnect error:", err);
		}
	}

	let syncAlert = $state<{ type: "success" | "error"; message: string } | null>(
		null,
	);

	async function handleSyncCanvas() {
		syncAlert = null;
		try {
			const res = await syncState.syncCanvas();
			syncAlert = {
				type: "success",
				message:
					res?.message ||
					`Successfully synced Canvas courses and assignments.`,
			};
			setTimeout(() => {
				syncAlert = null;
			}, 6000);
		} catch (err: any) {
			syncAlert = {
				type: "error",
				message:
					err?.message ||
					"Failed to sync Canvas. Please verify your Canvas access token and institution URL.",
			};
		}
	}
</script>

<!-- Canvas LMS integration status with grouped sections -->
<div class="rounded-xl border border-border bg-card p-6 shadow-xs space-y-3">
	<div class="flex items-center justify-between">
		<div class="flex items-center gap-3">
			<div
				class="size-9 rounded-lg bg-primary/10 flex items-center justify-center text-primary shrink-0"
			>
				<BookOpenIcon class="size-5" />
			</div>
			<h2
				class="text-base font-normal tracking-wide text-card-foreground leading-none"
			>
				Canvas LMS
			</h2>
		</div>

		{#if !isConnected}
			<span
				class="inline-flex items-center rounded-full bg-muted px-2.5 py-1 text-xs font-medium text-muted-foreground leading-none"
			>
				Not Connected
			</span>
		{/if}
	</div>

	<p class="text-xs text-muted-foreground w-full">
		Synchronize academic courses, section schedules, and assignment deadlines.
		Click a section to view dates or customize course nicknames.
	</p>

	{#if isConnected}
		<div class="mt-4 pt-4 border-t border-border flex flex-col gap-4">
			<div
				class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs bg-muted/40 p-3 rounded-lg border border-border/50"
			>
				<div class="space-y-1">
					<div>
						<span class="text-muted-foreground">Institution: </span>
						<span class="font-medium font-mono text-foreground"
							>{authState.record?.canvas_url || "https://canvas.instructure.com"}</span
						>
					</div>
					{#if authState.record?.canvas_student_name}
						<div>
							<span class="text-muted-foreground">Student: </span>
							<span class="font-medium text-foreground"
								>{authState.record?.canvas_student_name}</span
							>
						</div>
					{/if}
					{#if dataState.tasks.length > 0}
						<div>
							<span class="text-muted-foreground">Synced Tasks: </span>
							<span class="font-medium text-foreground"
								>{dataState.tasks.length} tasks</span
							>
						</div>
					{/if}
				</div>
			</div>

			{#if syncAlert}
				<div
					transition:slide={{ duration: 180 }}
					class="p-3 rounded-lg text-xs flex items-start gap-2.5 {syncAlert.type ===
					'error'
						? 'bg-destructive/10 text-destructive border border-destructive/20'
						: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20'}"
				>
					{#if syncAlert.type === "error"}
						<AlertTriangleIcon class="size-4 shrink-0 mt-0.5" />
					{:else}
						<CheckIcon class="size-4 shrink-0 mt-0.5" />
					{/if}
					<div class="flex-1 min-w-0">
						<p class="font-medium">{syncAlert.message}</p>
						{#if syncAlert.type === "error"}
							<p class="text-[11px] opacity-80 mt-0.5">
								Please check your Canvas token and institution URL.
							</p>
						{/if}
					</div>
				</div>
			{/if}

			{#if courses.length > 0}
				<div class="pt-2">
					<!-- Filter Header -->
					<div
						class="flex flex-col sm:flex-row sm:items-center justify-between gap-2"
					>
						<div class="flex items-center gap-2">
							<span class="text-xs text-foreground uppercase font-bold"
								>Course Sections ({courses.length})</span
							>
						</div>
					</div>

					<!-- Course groups listing with interactive expand panels -->
					<div class="space-y-2 pt-2">
						{#each courses as course, idx (course.id)}
							{@render courseItem(course, getSwatch(idx))}
						{/each}
					</div>
				</div>
			{/if}

			<div class="pt-2 flex justify-end">
				<Button
					variant="outline"
					size="sm"
					class="text-xs text-destructive hover:text-destructive hover:bg-destructive/10 border-border cursor-pointer"
					onclick={() => (showDisconnectModal = true)}
				>
					Disconnect Canvas
				</Button>
			</div>
		</div>
	{:else if showConnectForm}
		<form
			onsubmit={handleConnect}
			novalidate
			class="mt-4 pt-4 border-t border-border space-y-3"
			in:fade={{ duration: 150 }}
		>
			<div>
				<label
					for="settings-canvas-url"
					class="block text-xs font-medium text-muted-foreground mb-1"
				>
					Canvas URL
				</label>
				<input
					id="settings-canvas-url"
					type="text"
					inputmode="url"
					bind:value={canvasUrl}
					placeholder="canvas.instructure.com or school.instructure.com"
					class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-xs text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
					required
				/>
			</div>
			<div>
				<label
					for="settings-canvas-token"
					class="block text-xs font-medium text-muted-foreground mb-1"
				>
					Access Token
				</label>
				<input
					id="settings-canvas-token"
					type="password"
					bind:value={canvasToken}
					placeholder="Paste token"
					class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-xs text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
					required
				/>
			</div>
			<div class="flex items-center justify-between pt-1">
				<div class="flex gap-2">
					<Button
						type="button"
						variant="ghost"
						size="sm"
						class="text-xs cursor-pointer"
						onclick={() => (showConnectForm = false)}
					>
						Cancel
					</Button>
					<Button
						type="submit"
						size="sm"
						class="text-xs cursor-pointer"
						disabled={isLoading || !canvasToken.trim()}
					>
						{isLoading ? "Connecting..." : "Connect"}
					</Button>
				</div>
			</div>
		</form>
	{:else}
		<div
			class="mt-4 pt-4 border-t border-border flex items-center justify-between"
		>
			<p class="text-xs text-muted-foreground">No Canvas account linked yet.</p>
			<Button
				variant="outline"
				size="sm"
				class="text-xs cursor-pointer"
				onclick={() => (showConnectForm = true)}
			>
				Connect Canvas
			</Button>
		</div>
	{/if}
</div>

<!-- Confirmation Modal for Disconnecting Canvas -->
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
		aria-labelledby="disconnect-canvas-title"
		aria-describedby="disconnect-canvas-desc"
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
						id="disconnect-canvas-title"
						class="text-base font-semibold text-card-foreground leading-snug"
					>
						Disconnect Canvas LMS?
					</h3>
					<p
						id="disconnect-canvas-desc"
						class="text-xs text-muted-foreground leading-relaxed"
					>
						Are you sure you want to disconnect Canvas LMS? This will remove all
						synced academic courses, section schedules, and assignment dates
						from your Lasso calendar.
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

<!-- Reusable snippet for course section card with interactive details and nickname editing -->
{#snippet courseItem(course: CalendarRecord, swatchColor: string)}
	{@const isExpanded = expandedCourseId === course.id}
	{@const courseColor = resolveCalendarColor(course, swatchColor)}
	{@const calId = course.id}
	{@const isHiddenInSidebar = calendarVisibilityState.isHiddenInSidebar(
		calId,
		course.name,
	)}
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
				class="flex items-center gap-2.5 min-w-0 flex-1 cursor-pointer text-left group"
				onclick={() => toggleExpand(course)}
				aria-expanded={isExpanded}
			>
				<span
					class="size-2.5 rounded-xs shrink-0 shadow-2xs transition-opacity {isHiddenInSidebar
						? 'opacity-40'
						: ''}"
					style="background-color: {courseColor};"
				></span>
				<span
					class="font-medium truncate text-xs leading-none {isHiddenInSidebar
						? 'text-muted-foreground line-through opacity-75'
						: 'text-foreground'}"
				>
					{course.nickname || course.name}
				</span>
			</button>

			<!-- Right controls: Grey outlined button with no text + expand chevron -->
			<div class="flex items-center gap-2 shrink-0">
				<!-- Grey outlined visibility button with no text -->
				<button
					type="button"
					class="size-7 rounded-md border border-border/70 text-muted-foreground hover:text-foreground hover:border-foreground/40 hover:bg-muted/40 flex items-center justify-center transition-colors cursor-pointer {isHiddenInSidebar
						? 'opacity-50'
						: ''}"
					onclick={(e) => {
						e.stopPropagation();
						calendarVisibilityState.toggleSidebarVisibility(calId, course.name);
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
					class="p-1 text-muted-foreground hover:text-foreground cursor-pointer transition-colors"
					onclick={() => toggleExpand(course)}
					aria-label="Toggle course details"
				>
					<ChevronDownIcon
						class="size-3.5 transition-transform duration-200 {isExpanded
							? 'rotate-180 text-foreground'
							: ''}"
					/>
				</button>
			</div>
		</div>

		<!-- Expanded detail panel: dates, IDs, and local nickname editor -->
		{#if isExpanded}
			<div
				class="px-3 pb-3 pt-1 border-t border-border/50 bg-muted/20 space-y-3"
				transition:slide={{ duration: 180 }}
			>
				<!-- Grid of section metadata -->
				<div class="grid grid-cols-2 sm:grid-cols-3 gap-2.5 pt-1 text-[11px]">
					{#if course.course_id}
						<div class="flex flex-col gap-0.5">
							<span class="text-muted-foreground">Course ID</span>
							<span class="font-mono text-foreground font-medium">
								#{course.course_id}
							</span>
						</div>
					{/if}

					<div class="flex flex-col gap-0.5">
						<span class="text-muted-foreground">Calendar ID</span>
						<span class="font-mono text-foreground font-medium truncate" title={course.id}>
							{course.id}
						</span>
					</div>

					<div class="flex flex-col gap-0.5">
						<span class="text-muted-foreground">Original Name</span>
						<span class="font-mono text-foreground font-medium truncate" title={course.name}>
							{course.name}
						</span>
					</div>
				</div>

				<!-- Local Nickname Editor -->
				<div class="pt-2 border-t border-border/40">
					<form onsubmit={(e) => saveNickname(e, course.id)} class="space-y-2">
						<div class="flex items-center justify-between">
							<label
								for="nickname-{course.id}"
								class="text-[11px] font-medium text-muted-foreground flex items-center gap-1"
							>
								<TagIcon class="size-3 text-primary" />
								Course Nickname
							</label>
							{#if nicknameSuccessId === course.id}
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
								id="nickname-{course.id}"
								type="text"
								bind:value={nicknameInput}
								placeholder="e.g. Calc II, Physics Lab..."
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
							{#if course.nickname}
								<Button
									type="button"
									variant="ghost"
									size="sm"
									class="h-7 px-2 text-xs text-muted-foreground hover:text-foreground cursor-pointer gap-1"
									onclick={() => clearNickname(course.id)}
									title="Reset to default course name"
									disabled={isSavingNickname}
								>
									<RotateCcwIcon class="size-3" />
									<span>Reset</span>
								</Button>
							{/if}
						</div>
						<p class="text-[10px] text-muted-foreground">
							Stored in your Lasso workspace. Customizes the course
							title on your calendar and sidebar.
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
