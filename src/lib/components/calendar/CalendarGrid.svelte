<script lang="ts">
	import { onMount } from "svelte";
	import { fade } from "svelte/transition";
	import CalendarEventItem from "./CalendarEventItem.svelte";
	import CalendarAnnouncementItem from "./CalendarAnnouncementItem.svelte";
	import CalendarDeadlineItem from "./CalendarDeadlineItem.svelte";
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
						class="absolute left-0 right-0 h-[1px] bg-foreground/20 dark:bg-white/20 pointer-events-none z-0"
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
									class="absolute left-0 right-0 z-0 pointer-events-none flex items-center"
									style="top: {currentDayTimePercent}%; transform: translateY(-50%);"
									transition:fade={{ duration: 150 }}
								>
									<div
										class="w-[2.5px] h-3.5 bg-foreground dark:bg-white shrink-0 rounded-full translate-x-[-75%] shadow-xs"
									></div>
									<div
										class="flex-1 h-[2px] bg-foreground dark:bg-white translate-x-[-1.5px] shadow-xs"
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
								<CalendarEventItem
									event={evt}
									visibleColIndex={visibleIdx}
									onSelect={(item, colIdx) =>
										onSelectItem?.(item, "event", colIdx)}
									onDelete={onDeleteEvent}
								/>
							{/each}

							<!-- Announcement lines -->
							{#each announcements as ann (ann.id)}
								<CalendarAnnouncementItem
									announcement={ann}
									visibleColIndex={visibleIdx}
									onSelect={(item, colIdx) =>
										onSelectItem?.(item, "announcement", colIdx)}
								/>
							{/each}

							<!-- Deadline lines -->
							{#each deadlines as dl (dl.id)}
								<CalendarDeadlineItem
									deadline={dl}
									visibleColIndex={visibleIdx}
									onSelect={(item, colIdx) =>
										onSelectItem?.(item, "deadline", colIdx)}
								/>
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
