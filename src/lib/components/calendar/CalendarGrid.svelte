<script lang="ts">
	import { onMount } from "svelte";
	import { fade } from "svelte/transition";
	import MegaphoneIcon from "@lucide/svelte/icons/megaphone";
	import X from "@lucide/svelte/icons/x";
	import {
		isToday,
		getLocalTimeZone,
		type CalendarDate,
		type DateValue,
	} from "@internationalized/date";
	import { dragState } from "$lib/dragState.svelte";
	import type {
		DayItem,
		FormattedTimedEvent,
		FormattedDeadline,
		FormattedAnnouncement,
	} from "./calendarTypes";

	interface DragHoverState {
		colDateKey: string;
		startHour: number;
		startMin: number;
		topPercent: number;
		heightPercent: number;
		taskTitle: string;
		color: string;
		courseName: string;
	}

	interface Props {
		days: DayItem[];
		totalCount: number;
		visibleCount: number;
		buffer: number;
		slideAnimationDelta: number;
		transformPercent: number;
		isAnimating: boolean;
		currentDayTimePercent: number;
		hours: string[];
		getDayTimedEvents: (date: DateValue) => FormattedTimedEvent[];
		getDayDeadlines: (date: DateValue) => FormattedDeadline[];
		getDayAnnouncements: (date: DateValue) => FormattedAnnouncement[];
		onDropTask: (
			payload: any,
			date: CalendarDate,
			startHour: number,
			startMin: number,
		) => void;
		onDeleteEvent: (evt: FormattedTimedEvent) => void;
		onSelectItem?: (
			item: any,
			type: "event" | "announcement" | "deadline",
			visibleColIndex: number,
		) => void;
	}

	let {
		days,
		totalCount,
		visibleCount,
		buffer,
		slideAnimationDelta,
		transformPercent,
		isAnimating,
		currentDayTimePercent,
		hours,
		getDayTimedEvents,
		getDayDeadlines,
		getDayAnnouncements,
		onDropTask,
		onDeleteEvent,
		onSelectItem,
	}: Props = $props();

	const isTodayVisible = $derived.by(() => {
		return days.some((item, index) => {
			if (!item.isToday) return false;
			const currentIdx = index - buffer - slideAnimationDelta;
			if (currentIdx >= 0 && currentIdx < visibleCount) return true;
			if (isAnimating) {
				const prevIdx = index - buffer;
				if (prevIdx >= 0 && prevIdx < visibleCount) return true;
			}
			return false;
		});
	});

	let dragHoverState = $state<DragHoverState | null>(null);
	let scrollContainer = $state<HTMLDivElement | null>(null);

	onMount(() => {
		if (scrollContainer) {
			// Slight delay ensures the flex layout has fully calculated its height
			setTimeout(() => {
				if (scrollContainer) {
					scrollContainer.scrollTop = scrollContainer.scrollHeight;
				}
			}, 10);
		}
	});

	function formatTimeFromHourMin(hour: number, min: number): string {
		const d = new Date();
		d.setHours(hour, min, 0, 0);
		return d.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" });
	}

	function getSnappedTimeFromY(clientY: number, columnEl: HTMLElement) {
		const rect = columnEl.getBoundingClientRect();
		const y = clientY - rect.top;
		const ratio = Math.max(0, Math.min(1, y / rect.height));
		// Row 0 is Hour -1 (prev 11 PM), Row 1 starts at 12 AM (0 hours)
		const gridHour = ratio * 25 - 1;
		// Snap to 15-minute intervals
		const totalMinutes = Math.round(gridHour * 4) * 15;
		const clampedMinutes = Math.max(0, Math.min(23 * 60, totalMinutes));
		const startHour = Math.floor(clampedMinutes / 60);
		const startMin = clampedMinutes % 60;
		const topPercent = Math.min(
			95,
			Math.max(4.0, ((1 + clampedMinutes / 60) / 25) * 100),
		);
		const heightPercent = (60 / 60 / 25) * 100; // 4% (1 hour duration)
		return { startHour, startMin, topPercent, heightPercent };
	}

	function handleDragEnter(e: DragEvent, date: DateValue) {
		if (!dragState.activeTask) return;
		e.preventDefault();
	}

	function handleDragOver(e: DragEvent, date: DateValue) {
		if (!dragState.activeTask) return;
		e.preventDefault();
		if (e.dataTransfer) {
			e.dataTransfer.dropEffect = "copy";
		}
		const columnEl = e.currentTarget as HTMLElement;
		const { startHour, startMin, topPercent, heightPercent } =
			getSnappedTimeFromY(e.clientY, columnEl);
		dragHoverState = {
			colDateKey: date.toString(),
			startHour,
			startMin,
			topPercent,
			heightPercent,
			taskTitle: dragState.activeTask.taskName,
			color: dragState.activeTask.color,
			courseName: dragState.activeTask.courseName,
		};
	}

	function handleDragLeave(e: DragEvent, date: DateValue) {
		const currentTarget = e.currentTarget as HTMLElement;
		const relatedTarget = e.relatedTarget as Node | null;
		if (relatedTarget && currentTarget.contains(relatedTarget)) {
			return;
		}
		if (dragHoverState?.colDateKey === date.toString()) {
			dragHoverState = null;
		}
	}

	function handleDrop(e: DragEvent, date: CalendarDate) {
		e.preventDefault();
		dragHoverState = null;

		let payload = dragState.activeTask;
		if (!payload && e.dataTransfer) {
			try {
				const raw = e.dataTransfer.getData("application/json");
				if (raw) payload = JSON.parse(raw);
			} catch {}
		}
		dragState.clear();

		if (!payload) return;

		const columnEl = e.currentTarget as HTMLElement;
		const { startHour, startMin } = getSnappedTimeFromY(e.clientY, columnEl);
		onDropTask(payload, date, startHour, startMin);
	}
</script>

<!-- Viewport wrapper that DOES NOT scroll, sized to the window remaining height, masked at top and bottom -->
<div
	class="relative flex-1 min-h-0 w-full overflow-hidden calendar-viewport-mask z-10"
>
	<!-- Scrollable container inside the masked viewport (scrollbar hidden to match left and right padding) -->
	<div
		bind:this={scrollContainer}
		class="h-full w-full overflow-y-auto overflow-x-hidden [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
	>
		<div class="flex gap-2 px-2 w-full h-[140%] min-h-[750px] pt-0 pb-0">
			<!-- Part 1: Left time column (1 col, 25 rows) -->
			<div class="w-10 shrink-0 flex flex-col h-full select-none">
				{#each hours as hour, index}
					<div class="flex-1 min-h-0 flex items-start justify-end pr-1">
						<span
							class={`text-[10px] font-medium text${index == 7 || index == 13 || index == 19 ? "" : "-muted"}-foreground -translate-y-1/2`}
						>
							{hour}
						</span>
					</div>
				{/each}
			</div>

			<!-- Part 2: Middle grid with right fade across 25% of the 7th column -->
			<div
				class="flex-1 min-w-0 h-full calendar-right-mask relative overflow-hidden"
			>
				<!-- Static horizontal hour grid rows (25 rows) -->
				<div
					class="absolute inset-0 flex flex-col h-full divide-y divide-border/30 pointer-events-none"
				>
					{#each hours as _}
						<div class="flex-1 min-h-0 w-full"></div>
					{/each}
				</div>

				<!-- Static current time subtle guide line across grid (visible only when Today is in view) -->
				{#if isTodayVisible}
					<div
						class="absolute left-0 right-0 h-[1px] bg-white/20 pointer-events-none z-20"
						style="top: {currentDayTimePercent}%; transform: translateY(-50%);"
						transition:fade={{ duration: 150 }}
					></div>
				{/if}

				<!-- Sliding Days Track (39 columns animated in sync with header) -->
				<div
					class="h-full flex flex-row relative z-10"
					style="width: calc({totalCount} / {visibleCount} * 100%); transform: translate3d(-{transformPercent}%, 0, 0); {isAnimating
						? 'transition: transform 370ms cubic-bezier(0.16, 1, 0.3, 1);'
						: 'transition: none;'}"
				>
					{#each days as item, index (item.date.toString())}
						{@const colDate = item.date}
						{@const timedEvents = getDayTimedEvents(colDate)}
						{@const deadlines = getDayDeadlines(colDate)}
						{@const announcements = getDayAnnouncements(colDate)}
						{@const isColToday = item.isToday}
						{@const visibleIdx = index - buffer - slideAnimationDelta}

						<div
							role="region"
							aria-label={`Calendar column for ${colDate.toString()}`}
							style="flex: 0 0 calc(100% / {totalCount});"
							class="h-full relative border-r border-border/30 last:border-r-0 min-w-0 overflow-visible hover:z-50 {dragHoverState &&
							dragHoverState.colDateKey === colDate.toString()
								? 'bg-primary/5 ring-1 ring-inset ring-primary/20'
								: ''}"
							ondragenter={(e) => handleDragEnter(e, colDate)}
							ondragover={(e) => handleDragOver(e, colDate)}
							ondragleave={(e) => handleDragLeave(e, colDate)}
							ondrop={(e) => handleDrop(e, colDate)}
						>
							<!-- Prominent Today Current Time Indicator -->
							{#if isColToday && isTodayVisible}
								<div
									class="absolute left-0 right-0 z-30 pointer-events-none flex items-center"
									style="top: {currentDayTimePercent}%; transform: translateY(-50%);"
									transition:fade={{ duration: 150 }}
								>
									<div
										class="w-[2px] h-3 bg-white shrink-0 rounded-full translate-x-[-75%]"
									></div>
									<div
										class="flex-1 h-[1.5px] bg-white translate-x-[-1.5px]"
									></div>
								</div>
							{/if}

							<!-- Ghost preview when dragging a task over this column -->
							{#if dragHoverState && dragHoverState.colDateKey === colDate.toString()}
								<div
									class="absolute pointer-events-none rounded-md px-2 py-1 border-2 border-dashed z-40 flex flex-col justify-start overflow-hidden shadow-lg"
									style="
										left: 2px;
										width: calc(100% - 4px);
										top: {dragHoverState.topPercent}%;
										height: {dragHoverState.heightPercent}%;
										min-height: 24px;
										background-color: {dragHoverState.color}25;
										border-color: {dragHoverState.color};
									"
								>
									<div class="flex items-center gap-1 min-w-0">
										<span
											class="size-1.5 rounded-full shrink-0"
											style="background-color: {dragHoverState.color};"
										></span>
										<span
											class="truncate font-semibold text-[10px] text-foreground leading-tight"
										>
											{dragHoverState.taskTitle}
										</span>
									</div>
									<span
										class="text-[9px] text-muted-foreground font-mono mt-0.5"
									>
										{formatTimeFromHourMin(
											dragHoverState.startHour,
											dragHoverState.startMin,
										)} – {formatTimeFromHourMin(
											dragHoverState.startHour + 1,
											dragHoverState.startMin,
										)}
									</span>
								</div>
							{/if}

							<!-- Timed blocks: Google personal calendar events & Canvas task work sessions -->
							{#each timedEvents as evt (evt.id)}
								{@const isCompletedWorkSession = Boolean(
									evt.isTaskBlock && evt.isTaskDone,
								)}
								<div
									role="button"
									tabindex="0"
									class="absolute pointer-events-auto group/gev select-none cursor-pointer overflow-visible z-10 hover:z-50 text-left"
									style="
										left: calc({evt.leftPercent}% + 1.5px);
										width: calc({evt.widthPercent}% - 3px);
										top: {evt.topPercent}%;
										height: {evt.heightPercent}%;
										min-height: 20px;
									"
									onclick={(e) => {
										e.stopPropagation();
										onSelectItem?.(evt, "event", visibleIdx);
									}}
									onkeydown={(e) => {
										if (e.key === "Enter" || e.key === " ") {
											e.stopPropagation();
											onSelectItem?.(evt, "event", visibleIdx);
										}
									}}
								>
									<!-- Inner event pill -->
									<div
										class="w-full h-full rounded-md px-1.5 py-0.5 overflow-hidden transition-all duration-150 border text-[10px] font-medium leading-tight flex flex-col justify-start hover:ring-1 hover:ring-primary/40 shadow-xs relative border-3"
										style="
											background-color: {isCompletedWorkSession
											? `color-mix(in srgb, ${evt.calendarColor || evt.color || '#3b82f6'} 70%, black)`
											: evt.color || '#3b82f6'};
											border-color: {isCompletedWorkSession
											? evt.calendarColor || evt.color || '#3b82f6'
											: evt.color || '#3b82f6'};
											color: white;
										"
									>
										<!-- Line 1: Title & Delete button for work sessions -->
										<div
											class="flex items-center justify-between gap-1 min-w-0 w-full"
										>
											<span
												class="truncate font-semibold text-[10px] text-white drop-shadow-xs leading-tight flex-1 min-w-0"
											>
												{evt.title}
											</span>
										</div>
										<div
											class="flex items-center justify-between gap-1 min-w-0 w-full"
										>
											<!-- Line 2: Time on new line if big enough (omitted if small) -->
											{#if evt.heightPercent >= 3.0 && evt.widthPercent >= 28}
												<span
													class="truncate text-[9px] text-white font-medium leading-tight mt-0.5"
												>
													{evt.widthPercent > 60
														? evt.timeStr
														: evt.timeStr.split("-")[0].trim()}
												</span>
											{/if}
											{#if evt.isTaskBlock}
												<button
													type="button"
													class="opacity-0 group-hover/gev:opacity-100 hover:bg-black/30 rounded size-3.5 flex items-center justify-center text-white/90 hover:text-white transition-all cursor-pointer shrink-0 z-20"
													title="Delete work session"
													onclick={(e) => {
														e.stopPropagation();
														onDeleteEvent(evt);
													}}
												>
													<X class="size-2.5" />
												</button>
											{/if}
										</div>
									</div>

									<!-- Hover Details Card (solid, zero transparency) -->
									<div
										class="absolute hidden group-hover/gev:flex flex-col gap-1.5 z-[100] pointer-events-none p-2.5 rounded-lg bg-popover border border-border shadow-2xl text-popover-foreground text-xs min-w-[260px] max-w-[300px] animate-in fade-in zoom-in-95 duration-150 {evt.topPercent >
										70
											? 'bottom-full mb-2'
											: 'top-full mt-2'} {visibleIdx === 0
											? 'left-0'
											: visibleIdx === 6
												? 'right-0'
												: 'left-1/2 -translate-x-1/2'}"
									>
										<div
											class="flex items-center justify-between gap-1.5 font-semibold text-[11px]"
											style="color: {evt.color};"
										>
											<div class="flex items-center gap-1.5 min-w-0 truncate">
												<span
													class="size-2 rounded-full shrink-0"
													style="background-color: {evt.color};"
												></span>
												<span class="truncate">{evt.calendarName}</span>
											</div>
											{#if evt.isTaskBlock}
												<span
													class="text-[9px] px-1.5 py-0.5 rounded {isCompletedWorkSession
														? 'bg-emerald-500/10 text-emerald-500'
														: 'bg-primary/10 text-primary'} shrink-0 font-medium"
													>{isCompletedWorkSession
														? "Completed Work Session"
														: "Work Session"}</span
												>
											{:else if evt.source === "google"}
												<span
													class="text-[9px] px-1 py-0.5 rounded bg-muted text-muted-foreground shrink-0 font-mono"
													>Google</span
												>
											{:else}
												<span
													class="text-[9px] px-1 py-0.5 rounded bg-muted text-muted-foreground shrink-0 font-mono"
													>Event</span
												>
											{/if}
										</div>
										<p class="font-medium text-xs text-foreground leading-snug">
											{evt.title}
										</p>
										<div
											class="flex items-center justify-between gap-1 text-[10px] text-muted-foreground font-mono"
										>
											<span>{evt.timeStr}</span>
											{#if evt.isTaskBlock}
												<button
													type="button"
													class="text-[10px] text-destructive hover:underline cursor-pointer flex items-center gap-0.5"
													onclick={(e) => {
														e.stopPropagation();
														onDeleteEvent(evt);
													}}
												>
													Delete
												</button>
											{/if}
										</div>
										{#if evt.description}
											<p
												class="text-[10px] text-muted-foreground border-t border-border/40 pt-1 line-clamp-3 leading-relaxed whitespace-pre-line"
											>
												{evt.description}
											</p>
										{/if}
									</div>
								</div>
							{/each}

							<!-- Announcement lines -->
							{#each announcements as ann (ann.id)}
								<div
									role="button"
									tabindex="0"
									class="absolute pointer-events-auto group/announcement select-none flex items-center cursor-pointer z-10 hover:z-50 overflow-visible text-left {ann.isEndOfDay
										? 'py-2 items-end'
										: 'py-2.5 items-center'}"
									style="
										left: calc({ann.leftPercent}% + 1.5px);
										width: calc({ann.widthPercent}% - 3px);
										{ann.isEndOfDay
										? 'bottom: 0px;'
										: `top: ${ann.topPercent}%; transform: translateY(-50%);`}
									"
									onclick={(e) => {
										e.stopPropagation();
										onSelectItem?.(ann, "announcement", visibleIdx);
									}}
									onkeydown={(e) => {
										if (e.key === "Enter" || e.key === " ") {
											e.stopPropagation();
											onSelectItem?.(ann, "announcement", visibleIdx);
										}
									}}
								>
									<!-- Left line: thinner than deadline on sides (h-[2px] vs h-1) -->
									<div
										class="flex-1 h-[2px] rounded-full transition-opacity duration-150 opacity-80 group-hover/announcement:opacity-100"
										style="
											background-color: {ann.color};
											box-shadow: 0 0 4px {ann.color}70, 0 1px 2px rgba(0,0,0,0.1);
										"
									></div>

									<!-- Middle pill with megaphone icon -->
									<div
										class="px-2 py-0.5 rounded-full flex items-center justify-center gap-1 shadow-xs transition-transform duration-150 group-hover/announcement:scale-110 shrink-0 z-10 mx-1"
										style="
											background-color: {ann.color};
											box-shadow: 0 0 6px {ann.color}80, 0 1px 2px rgba(0,0,0,0.2);
										"
									>
										<MegaphoneIcon
											class="size-2.5 sm:size-3 text-white fill-white/20"
										/>
									</div>

									<!-- Right line: thinner than deadline on sides (h-[2px] vs h-1) -->
									<div
										class="flex-1 h-[2px] rounded-full transition-opacity duration-150 opacity-80 group-hover/announcement:opacity-100"
										style="
											background-color: {ann.color};
											box-shadow: 0 0 4px {ann.color}70, 0 1px 2px rgba(0,0,0,0.1);
										"
									></div>

									<!-- Hover Details Card -->
									<div
										class="absolute hidden group-hover/announcement:flex flex-col gap-1.5 z-[100] pointer-events-none p-2.5 rounded-lg bg-popover border border-border shadow-2xl text-popover-foreground text-xs min-w-[200px] max-w-[260px] animate-in fade-in zoom-in-95 duration-150
											{ann.isEndOfDay || ann.topPercent > 70 ? 'bottom-full mb-2' : 'top-full mt-2'}
											{visibleIdx === 0
											? 'left-0'
											: visibleIdx === 6
												? 'right-0'
												: 'left-1/2 -translate-x-1/2'}"
									>
										<!-- Course Header -->
										<div class="flex items-center justify-between gap-2">
											<div class="flex items-center gap-1.5 min-w-0">
												<span
													class="size-2 rounded-full shrink-0"
													style="background-color: {ann.color};"
												></span>
												<span
													class="font-semibold text-[11px] truncate"
													style="color: {ann.color};"
												>
													{ann.courseName}
												</span>
											</div>
											<span
												class="text-[10px] text-muted-foreground font-mono shrink-0"
											>
												{ann.timeStr}
											</span>
										</div>

										<!-- Announcement Title -->
										<p class="font-medium text-xs leading-snug text-foreground">
											{ann.title}
										</p>

										{#if ann.description}
											<p
												class="text-[11px] text-muted-foreground line-clamp-2 leading-relaxed"
											>
												{ann.description.replace(/<[^>]*>?/gm, "")}
											</p>
										{/if}

										<!-- Status & Tag Footer -->
										<div
											class="flex items-center justify-between pt-1 border-t border-border/40 text-[10px] text-muted-foreground"
										>
											<span
												class="inline-flex items-center gap-1 font-medium text-primary"
											>
												<MegaphoneIcon class="size-3 text-primary" />
												Announcement
											</span>
										</div>
									</div>
								</div>
							{/each}

							<!-- Deadline lines -->
							{#each deadlines as dl (dl.id)}
								<div
									role="button"
									tabindex="0"
									class="absolute pointer-events-auto group/deadline select-none flex cursor-pointer z-10 hover:z-50 overflow-visible text-left {dl.isEndOfDay
										? 'py-2 items-end'
										: 'py-2.5 items-center'}"
									style="
										left: calc({dl.leftPercent}% + 1.5px);
										width: calc({dl.widthPercent}% - 3px);
										{dl.isEndOfDay
										? 'bottom: 0px;'
										: `top: ${dl.topPercent}%; transform: translateY(-50%);`}
									"
									onclick={(e) => {
										e.stopPropagation();
										onSelectItem?.(dl, "deadline", visibleIdx);
									}}
									onkeydown={(e) => {
										if (e.key === "Enter" || e.key === " ") {
											e.stopPropagation();
											onSelectItem?.(dl, "deadline", visibleIdx);
										}
									}}
								>
									<!-- Colored / Greyed Deadline Line (stable, no vertical movement on hover) -->
									<div
										class="w-full h-1 rounded-full transition-opacity duration-150 {dl.status ===
										'done'
											? 'opacity-40 group-hover/deadline:opacity-85'
											: 'group-hover/deadline:opacity-100'}"
										style="
											background-color: {dl.status === 'done' ? '#9ca3af' : dl.color};
											box-shadow: {dl.status === 'done'
											? '0 1px 2px rgba(0,0,0,0.1)'
											: `0 0 5px ${dl.color}80, 0 1px 2px rgba(0,0,0,0.15)`};
										"
									></div>

									{#if !dl.isEndOfDay && dl.status !== "done"}
										<!-- Tag-like marker on right edge (triangley-square shape facing left) -->
										<div
											class="absolute right-0 top-1/2 -translate-y-1/2 flex items-center justify-center pointer-events-none z-10"
										>
											<svg
												class="w-2.5 h-2.5 transition-transform duration-150 group-hover/deadline:scale-125"
												viewBox="0 0 10 10"
												fill="none"
												style="filter: drop-shadow(0 1px 2px rgba(0,0,0,0.35));"
											>
												<path
													d="M0 5 L4.5 0 L9 0 C9.55 0 10 0.45 10 1 L10 9 C10 9.55 9.55 10 9 10 L4.5 10 Z"
													fill={dl.color}
												/>
											</svg>
										</div>
									{/if}

									<!-- Hover Details Card (solid, zero transparency) -->
									<div
										class="absolute hidden group-hover/deadline:flex flex-col gap-1.5 z-[100] pointer-events-none p-2.5 rounded-lg bg-popover border border-border shadow-2xl text-popover-foreground text-xs min-w-[200px] max-w-[260px] animate-in fade-in zoom-in-95 duration-150
											{dl.isEndOfDay || dl.topPercent > 70 ? 'bottom-full mb-2' : 'top-full mt-2'}
											{visibleIdx === 0
											? 'left-0'
											: visibleIdx === 6
												? 'right-0'
												: 'left-1/2 -translate-x-1/2'}"
									>
										<!-- Course Header -->
										<div class="flex items-center justify-between gap-2">
											<div class="flex items-center gap-1.5 min-w-0">
												<span
													class="size-2 rounded-full shrink-0"
													style="background-color: {dl.color};"
												></span>
												<span
													class="font-semibold text-[11px] truncate"
													style="color: {dl.color};"
												>
													{dl.courseName}
												</span>
											</div>
											<span
												class="text-[10px] text-muted-foreground font-mono shrink-0"
											>
												{dl.dueTimeStr}
											</span>
										</div>

										<!-- Assignment Name -->
										<p
											class="font-medium text-xs leading-snug text-foreground {dl.status ===
											'done'
												? 'line-through text-muted-foreground opacity-75'
												: ''}"
										>
											{dl.name}
										</p>

										<!-- Status & Priority -->
										<div
											class="flex items-center justify-between pt-1 border-t border-border/40 text-[10px] text-muted-foreground"
										>
											<span
												class="capitalize font-medium {dl.status === 'done'
													? 'text-emerald-500'
													: 'text-amber-500'}"
											>
												{dl.status === "done" ? "✓ Completed" : "Due Deadline"}
											</span>
											{#if dl.priority}
												<span
													class="uppercase tracking-wider text-[9px] px-1 py-0.2 bg-muted rounded font-mono"
												>
													{dl.priority}
												</span>
											{/if}
										</div>
									</div>
								</div>
							{/each}
						</div>
					{/each}
				</div>
			</div>

			<!-- Part 3: Right spacer (same width as left) -->
			<div class="w-10 shrink-0"></div>
		</div>
	</div>
</div>

<style>
	/* Fade out top edge only slightly, keeping bottom edge content visible */
	.calendar-viewport-mask {
		mask-image: linear-gradient(
			to bottom,
			transparent 0%,
			black 2%,
			black 100%
		);
		-webkit-mask-image: linear-gradient(
			to bottom,
			transparent 0%,
			black 2%,
			black 100%
		);
	}

	/* Fade out 25% of the width of the rightmost box in the 7-col grid (100% / 28 = 3.57%) */
	.calendar-right-mask {
		mask-image: linear-gradient(
			to right,
			black 0%,
			black calc(100% - 3.57%),
			transparent 100%
		);
		-webkit-mask-image: linear-gradient(
			to right,
			black 0%,
			black calc(100% - 3.57%),
			transparent 100%
		);
	}
</style>
