<script lang="ts">
	import TopDayBar from "$lib/components/calendar/TopDayBar.svelte";
	import AllDayEventsBar from "$lib/components/calendar/AllDayEventsBar.svelte";
	import CalendarGrid from "$lib/components/calendar/CalendarGrid.svelte";
	import CalendarItemOverlay from "$lib/components/calendar/CalendarItemOverlay.svelte";
	import type { FormattedTimedEvent } from "$lib/components/calendar/calendarTypes";
	import { dayState } from "$lib/dayState.svelte";
	import { themeState } from "$lib/themeState.svelte";
	import { syncState } from "$lib/syncState.svelte";
	import { dataState } from "$lib/dataState/dataState.svelte";
	import { parseTaskCalendarId } from "$lib/dataState/taskQueries.svelte";
	import {
		getDayDeadlines,
		getDayAnnouncements,
		getAllDayEventsForDate,
		getDayTimedEvents,
	} from "$lib/components/calendar/gridQueries.svelte";
	import {
		isToday,
		getLocalTimeZone,
		type CalendarDate,
		type DateValue,
	} from "@internationalized/date";

	const BUFFER = 16;
	const VISIBLE_COUNT = 7;
	const TOTAL_COUNT = BUFFER + VISIBLE_COUNT + BUFFER;

	// 25 rows: Row 0 is unlabeled (previous 11 PM), Row 1 starts at 12 AM through 11 PM at Row 24
	const HOURS = Array.from({ length: 25 }, (_, i) => {
		if (i === 0) return "";
		const hourIndex = i - 1; // 0 to 23
		if (hourIndex === 0) return "12 AM";
		if (hourIndex < 12) return `${hourIndex} AM`;
		if (hourIndex === 12) return "12 PM";
		return `${hourIndex - 12} PM`;
	});

	function getDayDiff(to: DateValue, from: DateValue): number {
		const utcTo = Date.UTC(to.year, to.month - 1, to.day);
		const utcFrom = Date.UTC(from.year, from.month - 1, from.day);
		return Math.round((utcTo - utcFrom) / 86400000);
	}

	let now = $state(new Date());
	let baseDate = $state<DateValue>(dayState.value);
	let prevDate = $state<DateValue>(dayState.value);
	let slideAnimationDelta = $state(0);
	let isAnimating = $state(false);

	$effect(() => {
		const timer = setInterval(() => {
			now = new Date();
		}, 1000); // 1-second interval to follow exact live time
		return () => clearInterval(timer);
	});

	const currentDayTimePercent = $derived.by(() => {
		const totalHours = now.getHours() + now.getMinutes() / 60;
		return ((1 + totalHours) / 25) * 100;
	});

	async function handleGlobalSync() {
		if (syncState.isAnySyncing) return;
		try {
			await syncState.syncAll();
		} catch (e) {
			console.warn("Global sync error:", e);
		}
	}

	async function onDropTask(
		payload: any,
		date: CalendarDate,
		startHour: number,
		startMin: number,
	) {
		const startDate = new Date(
			date.year,
			date.month - 1,
			date.day,
			startHour,
			startMin,
			0,
		);
		const endDate = new Date(startDate.getTime() + 60 * 60 * 1000);

		let targetCalId = payload.calendarId;
		if (
			!targetCalId ||
			targetCalId === "unassigned" ||
			!dataState.calendars.some((c) => c.id === targetCalId)
		) {
			const fallbackCal =
				dataState.calendars.find((c) => c.id !== "unassigned") ||
				dataState.calendars[0];
			if (fallbackCal) {
				targetCalId = fallbackCal.id;
			}
		}

		await dataState.addEvent({
			calendar: targetCalId,
			task: payload.taskId,
			title: payload.taskName,
			start: startDate.toISOString(),
			end: endDate.toISOString(),
			allday: false,
			deadline: false,
			color: "",
			description: `Work session for ${payload.taskName}`,
		});
	}

	async function onDeleteEvent(evt: FormattedTimedEvent) {
		if (!evt.rawId || !evt.isTaskBlock) return;
		await dataState.deleteEvent(evt.rawId);
	}

	interface SelectedOverlayItem {
		title: string;
		side: "left" | "right";
		itemType: "event" | "announcement" | "deadline";
		courseId: string;
		description?: string;
		source_link?: string;
	}

	let selectedOverlayItem = $state<SelectedOverlayItem | null>(null);

	function handleSelectItem(
		item: any,
		type: "event" | "announcement" | "deadline",
		visibleColIndex: number,
	) {
		const side: "left" | "right" = visibleColIndex < 4 ? "right" : "left";
		let courseId = "";

		if (item.task) {
			const calId = parseTaskCalendarId(item.task);
			const cal = dataState.calendars.find((c) => c.id === calId);
			courseId = cal?.course_id || "";
		} else if (item.event) {
			const calId = item.event.calendar;
			const cal = dataState.calendars.find((c) => c.id === calId);
			courseId = cal?.course_id || "";
		} else if (item.rawEvent) {
			const calId = item.rawEvent.calendar;
			const cal = dataState.calendars.find((c) => c.id === calId);
			courseId = cal?.course_id || "";
		} else if (item.calendarId) {
			const cal = dataState.calendars.find((c) => c.id === item.calendarId);
			courseId = cal?.course_id || "";
		}

		let sourceLink = item.task?.source_link || item.source_link || "";
		if (!sourceLink && item.taskId) {
			const tsk = dataState.tasks.find((t) => t.id === item.taskId);
			if (tsk) {
				sourceLink = tsk.source_link || "";
			}
		}

		selectedOverlayItem = {
			title: item.name || item.title || "",
			side,
			itemType: type,
			courseId,
			description: item.description,
			source_link: sourceLink,
		};
	}

	function handleSelectAllDayEvent(title: string, visibleColIndex: number) {
		const side: "left" | "right" = visibleColIndex < 4 ? "right" : "left";
		const evt = dataState.events.find((e) => e.title === title && e.allday);
		let courseId = "";
		let sourceLink = "";
		if (evt?.calendar) {
			const cal = dataState.calendars.find((c) => c.id === evt.calendar);
			courseId = cal?.course_id || "";
		}
		if (evt?.task) {
			const tsk = dataState.tasks.find((t) => t.id === evt.task);
			if (tsk) {
				sourceLink = tsk.source_link || "";
			}
		}
		selectedOverlayItem = {
			title,
			side,
			itemType: "event",
			courseId,
			description: evt?.description,
			source_link: sourceLink,
		};
	}

	// Watch dayState.value changes for carousel sliding
	$effect(() => {
		const targetDate = dayState.value;
		const delta = getDayDiff(targetDate, prevDate);
		prevDate = targetDate;

		if (delta === 0) return;

		if (Math.abs(delta) > 15) {
			isAnimating = false;
			slideAnimationDelta = 0;
			baseDate = targetDate;
			return;
		}

		const newOffset = slideAnimationDelta + delta;

		if (Math.abs(newOffset) > BUFFER - 2) {
			isAnimating = false;
			slideAnimationDelta = 0;
			baseDate = targetDate;
			return;
		}

		isAnimating = true;
		slideAnimationDelta = newOffset;
	});

	function handleTransitionEnd(e: TransitionEvent) {
		if (e.target !== e.currentTarget || e.propertyName !== "transform") return;

		if (slideAnimationDelta !== 0) {
			isAnimating = false;
			baseDate = (baseDate as CalendarDate).add({ days: slideAnimationDelta });
			slideAnimationDelta = 0;
		}
	}

	const days = $derived(
		Array.from({ length: TOTAL_COUNT }, (_, i) => {
			const dayOffset = i - (BUFFER + themeState.calendarOffset);
			const date = (baseDate as CalendarDate).add({ days: dayOffset });
			const jsDate = new Date(date.year, date.month - 1, date.day);
			const weekday = jsDate.toLocaleDateString(undefined, {
				weekday: "short",
			});
			const monthName = jsDate.toLocaleDateString(undefined, {
				month: "short",
			});
			const isSelected =
				date.year === dayState.value.year &&
				date.month === dayState.value.month &&
				date.day === dayState.value.day;
			const isCurrentToday = isToday(date, getLocalTimeZone());

			return {
				date,
				weekday,
				dayNumber: date.day,
				monthName,
				isSelected,
				isToday: isCurrentToday,
			};
		}),
	);

	const transformPercent = $derived(
		((BUFFER + slideAnimationDelta) * 100) / TOTAL_COUNT,
	);
</script>

<div class="h-full w-full flex flex-col overflow-hidden relative">
	<TopDayBar
		{days}
		totalCount={TOTAL_COUNT}
		visibleCount={VISIBLE_COUNT}
		{transformPercent}
		{isAnimating}
		onTransitionEnd={handleTransitionEnd}
	/>

	<AllDayEventsBar
		{days}
		totalCount={TOTAL_COUNT}
		visibleCount={VISIBLE_COUNT}
		{transformPercent}
		{isAnimating}
		isSyncing={syncState.isAnySyncing}
		{getAllDayEventsForDate}
		onSync={handleGlobalSync}
		onSelectEvent={handleSelectAllDayEvent}
	/>

	<CalendarGrid
		{days}
		totalCount={TOTAL_COUNT}
		visibleCount={VISIBLE_COUNT}
		buffer={BUFFER}
		{slideAnimationDelta}
		{transformPercent}
		{isAnimating}
		{currentDayTimePercent}
		hours={HOURS}
		{getDayTimedEvents}
		{getDayDeadlines}
		{getDayAnnouncements}
		{onDropTask}
		{onDeleteEvent}
		onSelectItem={handleSelectItem}
	/>

	{#if selectedOverlayItem}
		<CalendarItemOverlay
			title={selectedOverlayItem?.title ?? ""}
			side={selectedOverlayItem?.side ?? "right"}
			itemType={selectedOverlayItem?.itemType ?? "event"}
			courseId={selectedOverlayItem?.courseId ?? ""}
			description={selectedOverlayItem?.description}
			source_link={selectedOverlayItem?.source_link}
			onClose={() => {
				selectedOverlayItem = null;
			}}
		/>
	{/if}
</div>
