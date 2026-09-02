<script lang="ts">
	import { canvasState, type CanvasCourse } from "$lib/canvasState.svelte";
	import { Button } from "$lib/components/ui/button";
	import BookOpenIcon from "@lucide/svelte/icons/book-open";
	import CalendarIcon from "@lucide/svelte/icons/calendar";
	import RefreshCwIcon from "@lucide/svelte/icons/refresh-cw";
	import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
	import TagIcon from "@lucide/svelte/icons/tag";
	import CheckIcon from "@lucide/svelte/icons/check";
	import RotateCcwIcon from "@lucide/svelte/icons/rotate-ccw";
	import SparklesIcon from "@lucide/svelte/icons/sparkles";
	import AlertTriangleIcon from "@lucide/svelte/icons/alert-triangle";
	import EyeIcon from "@lucide/svelte/icons/eye";
	import EyeOffIcon from "@lucide/svelte/icons/eye-off";
	import { pb } from "$lib/pocketbase";
	import { calendarVisibilityState } from "$lib/calendarVisibilityState.svelte";
	import { fade, slide, scale } from "svelte/transition";

	interface CalendarRecord {
		id: string;
		name: string;
		color?: string;
		nickname?: string;
		course_id?: string;
	}

	let pbCalendars = $state<CalendarRecord[]>([]);

	async function loadPocketBaseCalendars() {
		if (!pb.authStore.isValid) return;
		try {
			pbCalendars = await pb
				.collection("calendars")
				.getFullList<CalendarRecord>({
					requestKey: null,
				});
		} catch (err) {
			console.warn("Failed to load calendars in CanvasCard:", err);
		}
	}

	$effect(() => {
		if (canvasState.isConnected) {
			loadPocketBaseCalendars();
		}
	});

	function getCalendarForCourse(
		course: CanvasCourse,
	): CalendarRecord | undefined {
		const name = (course.name || "").toLowerCase().trim();
		const orig = (course.original_name || "").toLowerCase().trim();
		const code = (course.course_code || "").toLowerCase().trim();
		return pbCalendars.find((c) => {
			const cName = (c.name || "").toLowerCase().trim();
			const cCourseId = c.course_id ? String(c.course_id) : "";
			return (
				(cCourseId && cCourseId === String(course.id)) ||
				cName === name ||
				(orig && cName === orig) ||
				(code && cName === code) ||
				c.id === String(course.id)
			);
		});
	}

	let showConnectForm = $state(false);
	let showDisconnectModal = $state(false);
	let canvasUrl = $state(canvasState.canvasUrl);
	let canvasToken = $state("");
	let selectedSectionTab = $state<"all" | "current" | "upcoming" | "previous">(
		"all",
	);

	// Section detail & nickname state
	let expandedCourseId = $state<number | string | null>(null);
	let nicknameInput = $state("");
	let isSavingNickname = $state(false);
	let nicknameSuccessId = $state<number | string | null>(null);

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

	function formatDate(dateStr?: string | null): string {
		if (!dateStr) return "Not specified";
		try {
			const d = new Date(dateStr);
			if (isNaN(d.getTime())) return "Not specified";
			return d.toLocaleDateString(undefined, {
				month: "short",
				day: "numeric",
				year: "numeric",
			});
		} catch (e) {
			return "Not specified";
		}
	}

	function toggleExpand(course: CanvasCourse) {
		if (expandedCourseId === course.id) {
			expandedCourseId = null;
		} else {
			expandedCourseId = course.id;
			const cal = getCalendarForCourse(course);
			nicknameInput = course.nickname || cal?.nickname || "";
			nicknameSuccessId = null;
		}
	}

	async function saveNickname(e: SubmitEvent, courseId: number | string) {
		e.preventDefault();
		isSavingNickname = true;
		try {
			await canvasState.updateNickname(courseId, nicknameInput.trim());
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

	async function clearNickname(courseId: number | string) {
		isSavingNickname = true;
		try {
			await canvasState.updateNickname(courseId, "");
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

		try {
			await canvasState.connect(cleanUrl, canvasToken.trim());
			showConnectForm = false;
			canvasToken = "";
		} catch (err) {}
	}

	let syncAlert = $state<{ type: "success" | "error"; message: string } | null>(
		null,
	);

	async function handleSyncCanvas() {
		syncAlert = null;
		try {
			const res = await canvasState.syncCanvas();
			syncAlert = {
				type: "success",
				message:
					res.message ||
					`Successfully synced ${res.coursesSynced} courses and ${res.tasksSynced} tasks.`,
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

		{#if !canvasState.isConnected}
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

	{#if canvasState.isConnected}
		<div class="mt-4 pt-4 border-t border-border flex flex-col gap-4">
			<div
				class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs bg-muted/40 p-3 rounded-lg border border-border/50"
			>
				<div class="space-y-1">
					<div>
						<span class="text-muted-foreground">Institution: </span>
						<span class="font-medium font-mono text-foreground"
							>{canvasState.canvasUrl}</span
						>
					</div>
					{#if canvasState.studentName}
						<div>
							<span class="text-muted-foreground">Student: </span>
							<span class="font-medium text-foreground"
								>{canvasState.studentName}</span
							>
						</div>
					{/if}
					{#if canvasState.tasks.length > 0}
						<div>
							<span class="text-muted-foreground">Synced Tasks: </span>
							<span class="font-medium text-foreground"
								>{canvasState.tasks.length} tasks</span
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

			{#if canvasState.validCourses.length > 0}
				<div class="pt-2">
					<!-- Filter Header -->
					<div
						class="flex flex-col sm:flex-row sm:items-center justify-between gap-2"
					>
						<div class="flex items-center gap-2">
							<span class="text-xs text-foreground uppercase font-bold"
								>Course Sections</span
							>
						</div>

						<!-- Group tabs -->
						<div
							class="flex flex-wrap gap-1 bg-muted/60 p-1 rounded-lg text-[11px]"
						>
							<button
								type="button"
								class="px-2 py-0.5 rounded-md font-medium transition-colors cursor-pointer {selectedSectionTab ===
								'all'
									? 'bg-background text-foreground shadow-xs'
									: 'text-muted-foreground hover:text-foreground'}"
								onclick={() => (selectedSectionTab = "all")}
							>
								All ({canvasState.validCourses.length})
							</button>
							<button
								type="button"
								class="px-2 py-0.5 rounded-md font-medium transition-colors cursor-pointer {selectedSectionTab ===
								'current'
									? 'bg-background text-foreground shadow-xs'
									: 'text-muted-foreground hover:text-foreground'}"
								onclick={() => (selectedSectionTab = "current")}
							>
								Current ({canvasState.currentCourses.length})
							</button>
							<button
								type="button"
								class="px-2 py-0.5 rounded-md font-medium transition-colors cursor-pointer {selectedSectionTab ===
								'upcoming'
									? 'bg-background text-foreground shadow-xs'
									: 'text-muted-foreground hover:text-foreground'}"
								onclick={() => (selectedSectionTab = "upcoming")}
							>
								Upcoming ({canvasState.upcomingCourses.length})
							</button>
							<button
								type="button"
								class="px-2 py-0.5 rounded-md font-medium transition-colors cursor-pointer {selectedSectionTab ===
								'previous'
									? 'bg-background text-foreground shadow-xs'
									: 'text-muted-foreground hover:text-foreground'}"
								onclick={() => (selectedSectionTab = "previous")}
							>
								Previous ({canvasState.previousCourses.length})
							</button>
						</div>
					</div>

					<!-- Course groups listing with interactive expand panels -->
					<div class="space-y-4 pt-1">
						<!-- Current Sections -->
						{#if (selectedSectionTab === "all" || selectedSectionTab === "current") && canvasState.currentCourses.length > 0}
							<div class="space-y-1.5">
								{#if selectedSectionTab === "all"}
									<div
										class="flex items-center gap-1.5 text-[11px] font-semibold tracking-wider text-muted-foreground uppercase"
									>
										<span>Current ({canvasState.currentCourses.length})</span>
									</div>
								{/if}
								{#each canvasState.currentCourses as course, idx}
									{@render courseItem(course, getSwatch(idx))}
								{/each}
							</div>
						{/if}

						<!-- Upcoming Sections (Not Published) -->
						{#if (selectedSectionTab === "all" || selectedSectionTab === "upcoming") && canvasState.upcomingCourses.length > 0}
							<div class="space-y-1.5">
								{#if selectedSectionTab === "all"}
									<div
										class="flex items-center gap-1.5 text-[11px] font-semibold tracking-wider text-muted-foreground uppercase"
									>
										<span
											>Upcoming / Unpublished ({canvasState.upcomingCourses
												.length})</span
										>
									</div>
								{/if}
								{#each canvasState.upcomingCourses as course, idx}
									{@render courseItem(
										course,
										course.color ||
											course.backgroundColor ||
											getSwatch(idx + 5),
									)}
								{/each}
							</div>
						{/if}

						<!-- Previous Sections -->
						{#if (selectedSectionTab === "all" || selectedSectionTab === "previous") && canvasState.previousCourses.length > 0}
							<div class="space-y-1.5">
								{#if selectedSectionTab === "all"}
									<div
										class="flex items-center gap-1.5 text-[11px] font-semibold tracking-wider text-muted-foreground uppercase"
									>
										<span>Previous ({canvasState.previousCourses.length})</span>
									</div>
								{/if}
								{#each canvasState.previousCourses as course, idx}
									{@render courseItem(
										course,
										course.color ||
											course.backgroundColor ||
											getSwatch(idx + 2),
									)}
								{/each}
							</div>
						{/if}
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
						disabled={canvasState.isLoading || !canvasToken.trim()}
					>
						{canvasState.isLoading ? "Connecting..." : "Connect"}
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
					onclick={() => {
						showDisconnectModal = false;
						canvasState.disconnect();
					}}
				>
					Disconnect
				</Button>
			</div>
		</div>
	</div>
{/if}

<!-- Reusable snippet for course section card with interactive details and nickname editing -->
{#snippet courseItem(course: CanvasCourse, swatchColor: string)}
	{@const isExpanded = expandedCourseId === course.id}
	{@const courseColor = course.color || course.backgroundColor || swatchColor}
	{@const cal = getCalendarForCourse(course)}
	{@const calId = cal ? cal.id : String(course.id)}
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
					{course.nickname || cal?.nickname || course.name}
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
					<div class="flex flex-col gap-0.5">
						<span class="text-muted-foreground flex items-center gap-1">
							<CalendarIcon class="size-3" />
							Start Date
						</span>
						<span class="font-medium text-foreground">
							{formatDate(course.start_at)}
						</span>
					</div>

					<div class="flex flex-col gap-0.5">
						<span class="text-muted-foreground flex items-center gap-1">
							<CalendarIcon class="size-3" />
							End Date
						</span>
						<span class="font-medium text-foreground">
							{formatDate(course.end_at)}
						</span>
					</div>

					<div class="flex flex-col gap-0.5">
						<span class="text-muted-foreground">Course ID</span>
						<span class="font-mono text-foreground font-medium">
							#{course.id}
						</span>
					</div>

					{#if course.original_name && course.original_name !== course.name}
						<div class="col-span-2 sm:col-span-3 flex flex-col gap-0.5">
							<span class="text-muted-foreground">Original Course Name</span>
							<span
								class="text-muted-foreground font-mono text-[10px] truncate"
							>
								{course.original_name}
							</span>
						</div>
					{/if}
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
							{#if course.nickname || cal?.nickname}
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
							Stored locally in your Lasso workspace. Customizes the course
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
