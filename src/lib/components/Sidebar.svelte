<script lang="ts">
	import * as Avatar from "$lib/components/ui/avatar";
	import { Button } from "$lib/components/ui/button";
	import { Calendar } from "$lib/components/ui/calendar";
	import { pageState } from "$lib/pageSystem.svelte";
	import { authState } from "$lib/authState.svelte";
	import {
		today,
		getLocalTimeZone,
		isToday,
		type DateValue,
	} from "@internationalized/date";
	import SettingsIcon from "@lucide/svelte/icons/settings";
	import CalendarIcon from "@lucide/svelte/icons/calendar";
	import UserIcon from "@lucide/svelte/icons/user";
	import ChevronUp from "@lucide/svelte/icons/chevron-up";
	import LogOutIcon from "@lucide/svelte/icons/log-out";
	import { dayState } from "$lib/dayState.svelte";
	import { syncState } from "$lib/syncState.svelte";
	import RefreshCwIcon from "@lucide/svelte/icons/refresh-cw";
	import CheckIcon from "@lucide/svelte/icons/check";
	import ChevronDown from "@lucide/svelte/icons/chevron-down";
	import EyeIcon from "@lucide/svelte/icons/eye";
	import EyeOffIcon from "@lucide/svelte/icons/eye-off";
	import Pipette from "@lucide/svelte/icons/pipette";
	import GripVertical from "@lucide/svelte/icons/grip-vertical";
	import { calendarVisibilityState } from "$lib/calendarVisibilityState.svelte";
	import { dragState, type DragTaskPayload } from "$lib/dragState.svelte";
	import { fly, fade, slide } from "svelte/transition";

	const CALENDAR_STORAGE_KEY = "lasso_sidebar_calendar_open";
	const TASKS_STORAGE_KEY = "lasso_sidebar_tasks_open";
	const GOOGLE_CALENDARS_STORAGE_KEY = "lasso_sidebar_google_calendars_open";
	const STORAGE_COLLAPSED_KEY = "lasso_sidebar_collapsed_calendars";

	import { dataState } from "$lib/dataState/dataState.svelte";
	import {
		getTasksByCourse,
		parseTaskCalendarId,
		sortTasksChronological,
		sortTasksCompleted,
		type TaskGroup,
	} from "$lib/dataState/taskQueries.svelte";
	import {
		resolveCalendarColor,
		getCourseworkCalendars,
		getGoogleCalendars,
	} from "$lib/dataState/calendarQueries.svelte";
	import {
		type CalendarRecord,
		type TaskRecord,
	} from "$lib/dataState/dataRecordInterfaces";

	type PocketBaseCalendar = CalendarRecord;
	type PocketBaseTask = TaskRecord;

	let isCalendarOpen = $state(
		typeof window !== "undefined"
			? localStorage.getItem(CALENDAR_STORAGE_KEY) !== "false"
			: true,
	);

	let isTasksOpen = $state(
		typeof window !== "undefined"
			? localStorage.getItem(TASKS_STORAGE_KEY) !== "false"
			: true,
	);

	let isGoogleCalendarsOpen = $state(
		typeof window !== "undefined"
			? localStorage.getItem(GOOGLE_CALENDARS_STORAGE_KEY) !== "false"
			: true,
	);

	let currentTime = $state(
		new Date().toLocaleTimeString([], { hour: "numeric", minute: "2-digit" }),
	);

	let collapsedCalendars = $state<Record<string, boolean>>(
		loadCollapsedCalendars(),
	);
	let expandedCompleted = $state<Record<string, boolean>>({});
	let expandedUpcoming = $state<Record<string, boolean>>({});
	let isScrolled = $state(false);

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

	const isAnySyncing = $derived(syncState.isAnySyncing);

	const isAnyConnected = $derived(
		syncState.isCanvasConnected || syncState.isGoogleConnected,
	);

	const oldestSyncDate = $derived.by(() => {
		const timestamps: number[] = [];
		if (syncState.isCanvasConnected && dataState.canvasSyncedAt) {
			const d = parseSyncDate(dataState.canvasSyncedAt);
			if (d) timestamps.push(d.getTime());
		}
		if (syncState.isGoogleConnected) {
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

	const syncTooltip = $derived.by(() => {
		if (isAnySyncing) return "Syncing in progress...";
		if (!isAnyConnected) return "No sync services connected";
		if (oldestSyncTimeFormatted) {
			return `Last sync: ${oldestSyncTimeFormatted} — Click to sync all`;
		}
		return "Connected — Click to sync all";
	});

	function handleFooterSyncClick() {
		if (isAnyConnected && !isAnySyncing) {
			syncState.syncAll().catch(() => {});
		}
	}

	function toggleCalendar() {
		isCalendarOpen = !isCalendarOpen;
		if (typeof window !== "undefined") {
			localStorage.setItem(CALENDAR_STORAGE_KEY, String(isCalendarOpen));
		}
	}

	function toggleTasks() {
		isTasksOpen = !isTasksOpen;
		if (typeof window !== "undefined") {
			localStorage.setItem(TASKS_STORAGE_KEY, String(isTasksOpen));
		}
	}

	function toggleGoogleCalendars() {
		isGoogleCalendarsOpen = !isGoogleCalendarsOpen;
		if (typeof window !== "undefined") {
			localStorage.setItem(
				GOOGLE_CALENDARS_STORAGE_KEY,
				String(isGoogleCalendarsOpen),
			);
		}
	}

	function toggleCalendarCollapse(calId: string) {
		collapsedCalendars = {
			...collapsedCalendars,
			[calId]: !collapsedCalendars[calId],
		};
		try {
			localStorage.setItem(
				STORAGE_COLLAPSED_KEY,
				JSON.stringify(collapsedCalendars),
			);
		} catch (e) {}
	}

	function toggleUpcoming(calId: string) {
		expandedUpcoming[calId] = !expandedUpcoming[calId];
	}

	function toggleCompleted(calId: string) {
		expandedCompleted[calId] = !expandedCompleted[calId];
	}

	function handleScroll(e: UIEvent & { currentTarget: HTMLDivElement }) {
		isScrolled = e.currentTarget.scrollTop > 2;
	}

	function loadCollapsedCalendars(): Record<string, boolean> {
		if (typeof window === "undefined") return {};
		try {
			const saved = localStorage.getItem(STORAGE_COLLAPSED_KEY);
			return saved ? JSON.parse(saved) : {};
		} catch {
			return {};
		}
	}

	async function setTaskState(task: PocketBaseTask) {
		const calId = parseTaskCalendarId(task);
		const cal = calId ? dataState.calendars.find((c) => c.id === calId) : null;
		if (
			cal?.source === "canvas" ||
			task.expand?.calendar?.source === "canvas"
		) {
			return; // Canvas task state is managed by Canvas
		}

		const nextStatus = task.status === "done" ? "todo" : "done";
		try {
			await dataState.updateTask(task.id, { status: nextStatus });
		} catch (err) {
			console.error("Failed to update task:", err);
		}
	}

	function formatDueDate(dueStr?: string): string {
		if (!dueStr) return "";
		try {
			const d = new Date(dueStr);
			if (isNaN(d.getTime())) return "";
			const now = new Date();
			const isSameDay =
				d.getFullYear() === now.getFullYear() &&
				d.getMonth() === now.getMonth() &&
				d.getDate() === now.getDate();
			const tomorrow = new Date(now);
			tomorrow.setDate(tomorrow.getDate() + 1);
			const isTomorrow =
				d.getFullYear() === tomorrow.getFullYear() &&
				d.getMonth() === tomorrow.getMonth() &&
				d.getDate() === tomorrow.getDate();

			const timeStr = d.toLocaleTimeString([], {
				hour: "numeric",
				minute: "2-digit",
			});

			if (isSameDay) {
				return `Today ${timeStr}`;
			}
			if (isTomorrow) {
				return `Tomorrow ${timeStr}`;
			}
			const monthDay = d.toLocaleDateString(undefined, {
				month: "short",
				day: "numeric",
			});
			return `${monthDay}`;
		} catch {
			return "";
		}
	}


	function getCalendarColor(cal?: PocketBaseCalendar | null): string {
		return resolveCalendarColor(cal);
	}

	function handleDragStart(
		e: DragEvent,
		task: PocketBaseTask,
		cal: PocketBaseCalendar,
	) {
		if (!e.dataTransfer) return;
		const calId = parseTaskCalendarId(task) || cal.id;
		const color = getCalendarColor(cal) || task.expand?.calendar?.color || "#3b82f6";
		const courseName =
			cal.nickname ||
			cal.name ||
			task.expand?.calendar?.nickname ||
			task.expand?.calendar?.name ||
			"Coursework";

		const payload: DragTaskPayload = {
			taskId: task.id,
			taskName: task.name,
			calendarId: calId,
			color,
			courseName,
		};

		dragState.setTask(payload);
		e.dataTransfer.setData("application/json", JSON.stringify(payload));
		e.dataTransfer.setData("text/plain", task.name);
		e.dataTransfer.effectAllowed = "copy";
	}

	function handleDragEnd() {
		dragState.clear();
	}

	// Group tasks by calendar with tasks sorted chronologically by due_date
	const canvasCalendars = $derived(getTasksByCourse());


	const visibleCanvasCalendars = $derived.by(() => {
		return canvasCalendars.filter(
			(g) => !calendarVisibilityState.isHiddenInSidebar(g.calendar.id),
		);
	});

	const googleCalendars = $derived(getGoogleCalendars());

	const visibleGoogleCalendars = $derived.by(() => {
		if (!authState.record?.google_connected) return [];
		return googleCalendars.filter(
			(cal) => !calendarVisibilityState.isHiddenInSidebar(cal.id, cal.nickname || cal.name),
		);
	});
</script>

<aside
	class="flex h-screen w-60 flex-col border-r border-border bg-sidebar text-sidebar-foreground shrink-0 select-none relative"
>
	<!-- Top header with branding and settings button -->
	<div
		class="flex h-14 items-center justify-between gap-1 border-b border-sidebar-border pl-1.5 pr-2 relative z-10 bg-sidebar shrink-0 overflow-visible"
	>
		<div class="flex items-center min-w-0 select-none overflow-visible">
			<img
				src="/lasso.svg"
				alt="Lasso"
				class="h-[68px] max-w-none w-auto object-contain shrink-0 invert dark:invert-0 -my-2"
			/>
		</div>

		<Button
			variant={pageState.current === "settings" ? "secondary" : "ghost"}
			size="icon"
			class="size-8 text-sidebar-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground cursor-pointer shrink-0"
			onclick={() =>
				pageState.current === "settings"
					? pageState.goBack()
					: pageState.setPage("settings")}
			aria-label={pageState.current === "settings"
				? "Return to previous page"
				: "Open settings"}
		>
			<SettingsIcon class="size-4" />
		</Button>
	</div>

	<!-- Pop-up Today button when away, or Clock & Sync indicator when viewing Today -->
	<div
		class="relative w-full shrink-0 flex items-center justify-center bg-sidebar z-10 overflow-hidden"
		style="height: var(--calendar-header-offset);"
	>
		{#if !isToday(dayState.value, getLocalTimeZone())}
			<div
				class="absolute inset-0 flex justify-center w-full -mt-[1px]"
				transition:fly={{ y: -24, duration: 200 }}
			>
				<button
					type="button"
					style="height: var(--today-button-height);"
					class="w-[90%] flex items-center justify-center gap-1 rounded-b-md rounded-t-none border-b border-x border-sidebar-border bg-sidebar-accent text-sidebar-foreground hover:bg-sidebar-accent/80 hover:text-primary text-[11px] font-medium cursor-pointer transition-colors shadow-xs"
					onclick={() => dayState.today()}
					aria-label="Return to today"
				>
					<CalendarIcon class="size-3" />
					<span>Today</span>
				</button>
			</div>
		{:else}
			<div
				class="absolute inset-0 flex justify-center items-center w-full"
				style="height: var(--today-button-height);"
				transition:fade={{ duration: 180 }}
			>
				<button
					type="button"
					class="flex items-center justify-center w-full px-4 text-muted-foreground select-none hover:text-foreground transition-colors cursor-pointer"
					title={syncTooltip}
					onclick={handleFooterSyncClick}
				>
					<!-- Left: Live Clock -->
					<div class="flex-1 text-right pr-2">
						<span
							class="text-[11px] font-medium text-foreground/80 tabular-nums"
						>
							{currentTime}
						</span>
					</div>

					<!-- Center: Sync Status Dot -->
					{#if isAnySyncing}
						<RefreshCwIcon
							class="size-2.5 animate-spin text-primary shrink-0"
						/>
					{:else if isAnyConnected}
						<span
							class="size-1.5 rounded-full bg-emerald-500 shadow-[0_0_5px_rgba(16,185,129,0.5)] shrink-0"
						></span>
					{:else}
						<span class="size-1.5 rounded-full bg-muted-foreground/40 shrink-0"
						></span>
					{/if}

					<!-- Right: Last Sync Time / Status -->
					<div class="flex-1 text-left pl-2 truncate">
						<span class="text-[10px] text-muted-foreground/80 tabular-nums">
							{#if isAnySyncing}
								syncing...
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
			</div>
		{/if}
	</div>

	<!-- Subtle top fade gradient when scrolled down -->
	<div
		class="pointer-events-none absolute left-0 right-[8px] z-20 h-6 bg-gradient-to-b from-sidebar via-sidebar/80 to-transparent transition-opacity duration-200 {isScrolled
			? 'opacity-100'
			: 'opacity-0'}"
		style="top: calc(3.5rem + var(--calendar-header-offset));"
	></div>

	<!-- Middle scrollable section: Calendar, Tasks, Google Calendars sorted chronologically -->
	<div
		class="flex-1 min-h-0 sidebar-scrollbar pl-1.5 pr-[2px] pt-1 pb-2 space-y-2 text-sidebar-foreground select-none"
		onscroll={handleScroll}
	>
		<!-- Calendar underneath the branding section, shifted down to align with 12AM marker and scaled down -->
		<div class="w-full">
			<!-- Divider with centered close / toggle badge button -->
			<div class="relative flex w-full my-1 items-center">
				<button
					type="button"
					onclick={toggleCalendar}
					class="shrink-0 cursor-pointer text-left"
				>
					<span
						class="text-[10px] pr-1 font-semibold uppercase tracking-wider text-muted-foreground hover:text-foreground transition-colors"
					>
						Calendar
					</span>
				</button>
				<div class="w-full h-full border-t border-sidebar-border"></div>
				<div class="w-8 flex items-center justify-center shrink-0">
					<button
						type="button"
						onclick={toggleCalendar}
						class="relative flex items-center justify-center rounded-full border border-sidebar-border bg-sidebar-accent text-sidebar-foreground hover:bg-sidebar-accent/80 hover:text-primary transition-all cursor-pointer shadow-2xs h-4.5 px-2 gap-1"
						aria-label={isCalendarOpen ? "Close calendar" : "Open calendar"}
						title={isCalendarOpen ? "Close calendar" : "Open calendar"}
					>
						<ChevronUp
							class="size-2.5 transition-transform duration-200 {isCalendarOpen
								? ''
								: 'rotate-180'}"
						/>
					</button>
				</div>
			</div>
		</div>
		{#if isCalendarOpen}
			<div transition:slide={{ duration: 200 }}>
				<Calendar
					type="single"
					bind:value={dayState.value}
					preventDeselect={true}
					class="w-full border-none bg-transparent p-0 shadow-none [--cell-size:--spacing(6)] text-xs"
				/>
			</div>
		{/if}

		{#if dataState.loading && dataState.tasks.length === 0}
			<div
				class="flex items-center justify-center py-6 text-xs text-muted-foreground gap-2"
			>
				<RefreshCwIcon class="size-3 animate-spin" />
				<span>Loading tasks...</span>
			</div>
		{:else if visibleCanvasCalendars.length === 0 && visibleGoogleCalendars.length === 0}
			<div
				class="px-3 py-6 text-center text-xs text-muted-foreground space-y-1"
			>
				<p class="font-medium text-foreground/80">No calendars</p>
				<p class="text-[11px] opacity-75">
					Connect Canvas or Google Calendar in Settings to sync your schedule.
				</p>
			</div>
		{:else}
			{#if visibleCanvasCalendars.length > 0}
				<!-- Divider with close / toggle badge button just like calendar -->
				<div class="relative flex w-full mt-1 mb-1 items-center">
					<button
						type="button"
						onclick={toggleTasks}
						class="shrink-0 cursor-pointer text-left"
					>
						<span
							class="text-[10px] pr-1 font-semibold uppercase tracking-wider text-muted-foreground hover:text-foreground transition-colors"
						>
							Canvas: Tasks by Course
						</span>
					</button>
					<div class="w-full h-full border-t border-sidebar-border"></div>
					<div class="w-8 flex items-center justify-center shrink-0">
						<button
							type="button"
							onclick={toggleTasks}
							class="relative flex items-center justify-center rounded-full border border-sidebar-border bg-sidebar-accent text-sidebar-foreground hover:bg-sidebar-accent/80 hover:text-primary transition-all cursor-pointer shadow-2xs h-4.5 px-2 gap-1"
							aria-label={isTasksOpen ? "Close tasks" : "Open tasks"}
							title={isTasksOpen ? "Close tasks" : "Open tasks"}
						>
							<ChevronUp
								class="size-2.5 transition-transform duration-200 {isTasksOpen
									? ''
									: 'rotate-180'}"
							/>
						</button>
					</div>
				</div>

				{#if isTasksOpen}
					<div transition:slide={{ duration: 200 }} class="space-y-2.5">
						{#each visibleCanvasCalendars as group (group.calendar.id)}
							{@const isCalHidden = calendarVisibilityState.isHiddenOnCalendar(
								group.calendar.id,
							)}
							{@const isCalIsolated = calendarVisibilityState.isIsolated(
								group.calendar.id,
							)}
							<div
								class="space-y-1 {isCalHidden && !isCalIsolated
									? 'opacity-70'
									: ''}"
							>
								<!-- Course Header (Outlined button with course color) -->
								<div
									class="w-full grid grid-cols-[1fr_auto] items-center rounded-md text-xs font-medium text-sidebar-foreground bg-transparent select-none min-h-[34px] gap-1"
								>
									<button
										type="button"
										class="w-full min-w-0 flex items-center justify-between px-2.5 py-1 rounded-md text-xs font-medium text-sidebar-foreground border bg-transparent hover:bg-sidebar-accent/40 transition-colors cursor-pointer group select-none shadow-xs"
										style="border-color: {group.calendar.color || '#3b82f6'};"
										onclick={() => toggleCalendarCollapse(group.calendar.id)}
									>
										<div class="flex items-center gap-1.5 min-w-0">
											<span
												class="truncate font-medium text-[11px] tracking-tight {isCalHidden &&
												!isCalIsolated
													? 'text-muted-foreground line-through opacity-75'
													: ''}"
											>
												{group.calendar.nickname || group.calendar.name}
											</span>
										</div>

										<div
											class="flex items-center gap-0.5 shrink-0 text-muted-foreground"
										>
											<span
												class="size-5 flex items-center justify-center shrink-0"
											>
												<ChevronDown
													class="size-3.5 transition-transform duration-150 {collapsedCalendars[
														group.calendar.id
													]
														? '-rotate-90'
														: ''}"
												/>
											</span>
										</div>
									</button>
									<!-- Toggle visibility and isolate on main calendar (Hidden when in settings to avoid confusion) -->
									{#if pageState.current !== "settings"}
										<div class="col-start-2 flex items-center">
											<!-- Isolate button (eyedropper) to the left of the eye -->
											<button
												tabindex="0"
												class="size-5 rounded flex items-center justify-center transition-colors cursor-pointer {isCalIsolated
													? 'text-primary hover:text-primary/70'
													: 'text-muted-foreground/60 hover:text-foreground hover:bg-sidebar-accent/80'}"
												onclick={(e) => {
													e.stopPropagation();
													calendarVisibilityState.toggleIsolate(
														group.calendar.id,
													);
												}}
												onkeydown={(e) => {
													if (e.key === "Enter" || e.key === " ") {
														e.stopPropagation();
														calendarVisibilityState.toggleIsolate(
															group.calendar.id,
														);
													}
												}}
												title={isCalIsolated
													? "Un-isolate calendar (click to clear)"
													: "Isolate calendar"}
												aria-label={isCalIsolated
													? "Un-isolate calendar"
													: "Isolate calendar"}
											>
												<Pipette class="size-3.5" />
											</button>

											<!-- Toggle visibility on main calendar -->
											<button
												tabindex="0"
												class="size-5 rounded flex items-center justify-center transition-colors hover:text-foreground hover:bg-sidebar-accent/80 cursor-pointer {isCalHidden
													? 'text-muted-foreground/50 hover:text-foreground'
													: 'text-muted-foreground/80 hover:text-foreground'}"
												onclick={(e) => {
													e.stopPropagation();
													calendarVisibilityState.toggleCalendarVisibility(
														group.calendar.id,
													);
												}}
												onkeydown={(e) => {
													if (e.key === "Enter" || e.key === " ") {
														e.stopPropagation();
														calendarVisibilityState.toggleCalendarVisibility(
															group.calendar.id,
														);
													}
												}}
												title={isCalHidden ? "Show Calendar" : "Hide Calendar"}
												aria-label={isCalHidden
													? "Show Calendar"
													: "Hide Calendar"}
											>
												{#if isCalHidden}
													<EyeOffIcon class="size-3.5" />
												{:else}
													<EyeIcon class="size-3.5" />
												{/if}
											</button>
										</div>
									{/if}
								</div>

								<!-- Tasks under this calendar -->
								{#if !collapsedCalendars[group.calendar.id]}
									{@const visibleUpcoming = expandedUpcoming[group.calendar.id]
										? group.upcomingTasks
										: group.upcomingTasks.slice(0, 7)}
									{@const visibleCompleted = expandedCompleted[
										group.calendar.id
									]
										? group.completedTasks
										: group.completedTasks.slice(0, 3)}
									<div
										transition:slide={{ duration: 150 }}
										class="space-y-0.5 pt-0.5 px-0.5"
									>
										{#if group.tasks.length === 0}
											<div
												class="px-2 py-1 text-[10.5px] text-muted-foreground/60 italic"
											>
												No assignments
											</div>
										{:else}
											<!-- Upcoming assignments -->
											{#if group.upcomingTasks.length > 0}
												{#each visibleUpcoming as task (task.id)}
													<div
														role="button"
														tabindex="0"
														aria-label={`Drag ${task.name} to calendar`}
														class="group/task flex items-start gap-1.5 px-1 py-1 rounded hover:bg-sidebar-accent/50 transition-colors text-xs cursor-grab active:cursor-grabbing select-none"
														draggable="true"
														ondragstart={(e) => handleDragStart(e, task, group.calendar)}
														ondragend={handleDragEnd}
													>
														<!-- Drag cue on hover -->
														<div
															class="mt-1 opacity-0 group-hover/task:opacity-40 text-muted-foreground transition-opacity shrink-0 -mr-0.5"
															title="Drag onto calendar"
														>
															<GripVertical class="size-2.5" />
														</div>

														<!-- Status indicator / toggle -->
														{#if group.calendar.source === "canvas" || task.expand?.calendar?.source === "canvas"}
															<span
																class="mt-1.5 size-1.5 rounded-full shrink-0 mx-1 opacity-70 select-none"
																style="background-color: {group.calendar
																	.color || '#3b82f6'};"
																title="Due on Canvas"
															></span>
														{:else}
															<!-- Checkbox toggle button for manual tasks -->
															<button
																type="button"
																class="mt-0.5 size-3.5 rounded border flex items-center justify-center shrink-0 transition-colors cursor-pointer border-muted-foreground/40 hover:border-primary"
																onclick={(e) => {
																	e.stopPropagation();
																	setTaskState(task);
																}}
																aria-label="Mark as completed"
															>
															</button>
														{/if}

														<!-- Task info -->
														<div class="flex-1 min-w-0 pointer-events-none">
															<p
																class="text-[11px] leading-snug truncate text-sidebar-foreground"
																title={task.name}
															>
																{task.name}
															</p>

															{#if task.due_date}
																{@const formattedDue = formatDueDate(
																	task.due_date,
																)}
																{#if formattedDue}
																	<span
																		class="text-[9px] text-muted-foreground/75"
																	>
																		{formattedDue}
																	</span>
																{/if}
															{/if}
														</div>
													</div>
												{/each}

												{#if group.upcomingTasks.length > 7}
													<button
														type="button"
														class="w-full py-1 px-2 text-[10px] text-muted-foreground hover:text-foreground font-medium text-center hover:bg-sidebar-accent/40 rounded transition-colors cursor-pointer flex items-center justify-center gap-1"
														onclick={() => toggleUpcoming(group.calendar.id)}
													>
														{#if expandedUpcoming[group.calendar.id]}
															<span>Show less</span>
															<ChevronUp class="size-2.5" />
														{:else}
															<span
																>Show {group.upcomingTasks.length - 7} more</span
															>
															<ChevronDown class="size-2.5" />
														{/if}
													</button>
												{/if}
											{/if}

											<!-- Completed assignments -->
											{#if group.completedTasks.length > 0}
												<div
													class="pt-1.5 pb-0.5 px-1.5 flex items-center gap-1.5 text-[10px] font-medium text-muted-foreground/70 uppercase tracking-wider"
												>
													<span>Completed</span>
													<div class="h-px flex-1 bg-sidebar-border/40"></div>
												</div>

												{#each visibleCompleted as task (task.id)}
													<div
														role="button"
														tabindex="0"
														aria-label={`Drag ${task.name} to calendar`}
														class="group/task flex items-start gap-1.5 px-1 py-1 rounded hover:bg-sidebar-accent/50 transition-colors text-xs cursor-grab active:cursor-grabbing select-none"
														draggable="true"
														ondragstart={(e) => handleDragStart(e, task, group.calendar)}
														ondragend={handleDragEnd}
													>
														<!-- Drag cue on hover -->
														<div
															class="mt-1 opacity-0 group-hover/task:opacity-40 text-muted-foreground transition-opacity shrink-0 -mr-0.5"
															title="Drag onto calendar"
														>
															<GripVertical class="size-2.5" />
														</div>

														<!-- Status indicator / toggle -->
														{#if group.calendar.source === "canvas" || task.expand?.calendar?.source === "canvas"}
															<span
																class="mt-0.5 size-3.5 flex items-center justify-center text-emerald-500 shrink-0 select-none"
																title="Submitted on Canvas"
															>
																<CheckIcon class="size-3 stroke-[2.5]" />
															</span>
														{:else}
															<!-- Checkbox toggle button for manual tasks -->
															<button
																type="button"
																class="mt-0.5 size-3.5 rounded border flex items-center justify-center shrink-0 transition-colors cursor-pointer bg-primary border-primary text-primary-foreground"
																onclick={(e) => {
																	e.stopPropagation();
																	setTaskState(task);
																}}
																aria-label="Mark as incomplete"
															>
																<CheckIcon class="size-2.5 stroke-[3]" />
															</button>
														{/if}

														<!-- Task info -->
														<div class="flex-1 min-w-0 pointer-events-none">
															<div
																class="flex items-center justify-between gap-1.5 min-w-0"
															>
																<p
																	class="text-[11px] leading-snug truncate transition-colors line-through text-muted-foreground opacity-60 flex-1 min-w-0"
																	title={task.name}
																>
																	{task.name}
																</p>
																{#if task.grade}
																	<span
																		class="text-[9px] font-semibold font-mono text-emerald-600 dark:text-emerald-400 shrink-0 px-1.5 py-0.5 rounded bg-emerald-500/10 border border-emerald-500/20 leading-none"
																		title={`Grade: ${task.grade}`}
																	>
																		{task.grade}
																	</span>
																{/if}
															</div>

															{#if task.due_date}
																{@const formattedDue = formatDueDate(
																	task.due_date,
																)}
																{#if formattedDue}
																	<span
																		class="text-[9px] text-muted-foreground/75 font-mono"
																	>
																		{formattedDue}
																	</span>
																{/if}
															{/if}
														</div>
													</div>
												{/each}

												{#if group.completedTasks.length > 3}
													<button
														type="button"
														class="w-full py-1 px-2 text-[10px] text-muted-foreground hover:text-foreground font-medium text-center hover:bg-sidebar-accent/40 rounded transition-colors cursor-pointer flex items-center justify-center gap-1"
														onclick={() => toggleCompleted(group.calendar.id)}
													>
														{#if expandedCompleted[group.calendar.id]}
															<span>Show less</span>
															<ChevronUp class="size-2.5" />
														{:else}
															<span
																>Show {group.completedTasks.length - 3} more</span
															>
															<ChevronDown class="size-2.5" />
														{/if}
													</button>
												{/if}
											{/if}
										{/if}
									</div>
								{/if}
							</div>
						{/each}
					</div>
				{/if}
			{/if}

			<!-- Google Calendars Section underneath with dual visibility logic -->
			{#if visibleGoogleCalendars.length > 0}
				<!-- Divider with close / toggle badge button just like calendar -->
				<div class="relative flex w-full my-1 items-center">
					<button
						type="button"
						onclick={toggleGoogleCalendars}
						class="shrink-0 cursor-pointer text-left"
					>
						<span
							class="text-[10px] pr-1 font-semibold uppercase tracking-wider text-muted-foreground hover:text-foreground transition-colors"
						>
							Google Calendars
						</span>
					</button>
					<div class="w-full h-full border-t border-sidebar-border"></div>
					<div class="w-8 flex items-center justify-center shrink-0">
						<button
							type="button"
							onclick={toggleGoogleCalendars}
							class="relative flex items-center justify-center rounded-full border border-sidebar-border bg-sidebar-accent text-sidebar-foreground hover:bg-sidebar-accent/80 hover:text-primary transition-all cursor-pointer shadow-2xs h-4.5 px-2 gap-1"
							aria-label={isGoogleCalendarsOpen
								? "Close Google calendars"
								: "Open Google calendars"}
							title={isGoogleCalendarsOpen
								? "Close Google calendars"
								: "Open Google calendars"}
						>
							<ChevronUp
								class="size-2.5 transition-transform duration-200 {isGoogleCalendarsOpen
									? ''
									: 'rotate-180'}"
							/>
						</button>
					</div>
				</div>

				{#if isGoogleCalendarsOpen}
					<div transition:slide={{ duration: 200 }} class="space-y-1">
						{#each visibleGoogleCalendars as gcal (gcal.id)}
							{@const isCalHidden = calendarVisibilityState.isHiddenOnCalendar(
								gcal.id,
							)}
							{@const isCalIsolated = calendarVisibilityState.isIsolated(
								gcal.id,
							)}
							<div
								class="space-y-1 {isCalHidden && !isCalIsolated
									? 'opacity-70'
									: ''}"
							>
								<div
									class="w-full grid grid-cols-[1fr_auto] items-center rounded-md text-xs font-medium text-sidebar-foreground bg-transparent select-none min-h-[34px] gap-1"
								>
									<div
										class="w-full min-w-0 flex items-center justify-between px-2.5 py-1 rounded-md text-xs font-medium text-sidebar-foreground border bg-transparent hover:bg-sidebar-accent/40 transition-colors select-none min-h-[34px]"
										style="border-color: {gcal.color ||
											'#3b82f6'};"
									>
										<div class="flex items-center gap-1.5 min-w-0">
											<span
												class="size-2 rounded-xs shrink-0 shadow-2xs"
												style="background-color: {gcal.color ||
													'#3b82f6'};"
											></span>
											<span
												class="truncate font-medium text-[11px] tracking-tight {isCalHidden &&
												!isCalIsolated
													? 'text-muted-foreground line-through opacity-75'
													: ''}"
											>
												{gcal.nickname || gcal.name}
											</span>
										</div>
									</div>
									{#if pageState.current !== "settings"}
										<div class="col-start-2 flex items-center">
											<!-- Isolate button (eyedropper) to the left of the eye -->
											<button
												tabindex="0"
												class="size-5 rounded flex items-center justify-center transition-colors cursor-pointer {isCalIsolated
													? 'text-primary hover:text-primary/70'
													: 'text-muted-foreground/60 hover:text-foreground hover:bg-sidebar-accent/80'}"
												onclick={(e) => {
													e.stopPropagation();
													calendarVisibilityState.toggleIsolate(gcal.id);
												}}
												onkeydown={(e) => {
													if (e.key === "Enter" || e.key === " ") {
														e.stopPropagation();
														calendarVisibilityState.toggleIsolate(gcal.id);
													}
												}}
												title={isCalIsolated
													? "Un-isolate calendar"
													: "Isolate calendar"}
												aria-label={isCalIsolated
													? "Un-isolate calendar"
													: "Isolate calendar"}
											>
												<Pipette class="size-3.5" />
											</button>

											<!-- Toggle visibility on main calendar -->
											<button
												type="button"
												class="size-5 rounded flex items-center justify-center transition-colors hover:text-foreground hover:bg-sidebar-accent/80 cursor-pointer {isCalHidden
													? 'text-muted-foreground/50 hover:text-foreground'
													: 'text-muted-foreground/80 hover:text-foreground'}"
												onclick={(e) => {
													e.stopPropagation();
													calendarVisibilityState.toggleCalendarVisibility(
														gcal.id,
													);
												}}
												title={isCalHidden
													? "Hidden on calendar grid (click to show)"
													: "Visible on calendar grid (click to hide)"}
												aria-label={isCalHidden
													? "Show on calendar grid"
													: "Hide on calendar grid"}
											>
												{#if isCalHidden}
													<EyeOffIcon class="size-3.5" />
												{:else}
													<EyeIcon class="size-3.5" />
												{/if}
											</button>
										</div>
									{/if}
								</div>
							</div>
						{/each}
					</div>
				{/if}
			{/if}
		{/if}
	</div>

	<!-- User display at the bottom -->
	<div class="border-t border-sidebar-border p-2.5 shrink-0">
		{#if authState.isAuthenticated && authState.user}
			<div class="flex items-center justify-between gap-2 p-1">
				<button
					type="button"
					class="flex items-center gap-2.5 min-w-0 text-left cursor-pointer group"
					onclick={() => pageState.setPage("settings")}
					aria-label="Account settings"
				>
					<Avatar.Root
						class="size-8 rounded-full border border-sidebar-border bg-sidebar-accent group-hover:border-primary/40 transition-colors"
					>
						{#if authState.user.avatarUrl}
							<Avatar.Image
								src={authState.user.avatarUrl}
								alt={authState.user.name}
							/>
						{/if}
						<Avatar.Fallback
							class="text-xs font-medium text-sidebar-foreground"
						>
							<UserIcon class="size-4" />
						</Avatar.Fallback>
					</Avatar.Root>
					<div class="flex flex-col min-w-0">
						<span
							class="truncate text-sm font-medium text-sidebar-foreground group-hover:text-primary transition-colors"
						>
							{authState.user.name}
						</span>
						<span class="truncate text-xs text-muted-foreground">
							{authState.user.email}
						</span>
					</div>
				</button>

				<Button
					variant="ghost"
					size="icon"
					class="size-7 text-muted-foreground hover:text-destructive hover:bg-destructive/10 cursor-pointer"
					onclick={() => authState.logout()}
					title="Sign out"
				>
					<LogOutIcon class="size-3.5" />
				</Button>
			</div>
		{:else}
			<div
				class="flex items-center justify-between p-1 text-xs text-muted-foreground"
			>
				<div class="flex items-center gap-2">
					<span class="size-2 rounded-full bg-amber-500/80"></span>
					<span>Not signed in</span>
				</div>
			</div>
		{/if}
	</div>
</aside>

<style>
	:global(.sidebar-scrollbar) {
		overflow-y: auto;
		scrollbar-gutter: stable;
		scrollbar-width: thin;
		scrollbar-color: color-mix(
				in srgb,
				var(--sidebar-foreground) 25%,
				transparent
			)
			transparent;
	}

	:global(.sidebar-scrollbar::-webkit-scrollbar) {
		width: 6px;
		background: transparent;
	}

	:global(.sidebar-scrollbar::-webkit-scrollbar-track) {
		background: transparent;
	}

	:global(.sidebar-scrollbar::-webkit-scrollbar-thumb) {
		background-color: color-mix(
			in srgb,
			var(--sidebar-foreground) 25%,
			transparent
		);
		border-radius: 9999px;
	}

	:global(.sidebar-scrollbar::-webkit-scrollbar-thumb:hover) {
		background-color: color-mix(
			in srgb,
			var(--sidebar-foreground) 50%,
			transparent
		);
	}
</style>
