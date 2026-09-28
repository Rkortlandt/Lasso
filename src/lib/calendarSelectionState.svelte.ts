import { CalendarDate, type DateValue } from "@internationalized/date";
import { dayState } from "$lib/dayState.svelte";
import { themeState } from "$lib/themeState.svelte";
import { pageState } from "$lib/pageSystem.svelte";
import { dataState } from "$lib/dataState/dataState.svelte";
import { getTaskLocalDate, ISOIsolateStart } from "$lib/components/calendar/gridQueries.svelte";
import type {
	CalendarRecord,
	EventRecord,
	TaskRecord,
} from "$lib/dataState/dataRecordInterfaces";

export interface SelectedOverlayItem {
	id?: string;
	taskId?: string;
	title: string;
	side: "left" | "right";
	itemType: "event" | "announcement" | "deadline";
	courseId: string;
	description?: string;
	source_link?: string;
	date?: DateValue;
}

export interface AssociatedTaskItem {
	id: string;
	taskId: string;
	title: string;
	itemType: "event" | "announcement" | "deadline";
	courseId: string;
	description?: string;
	source_link?: string;
	dateTime: Date;
	calDate: CalendarDate;
}

class CalendarSelectionState {
	selectedItem = $state<SelectedOverlayItem | null>(null);

	selectItem(item: SelectedOverlayItem | null) {
		this.selectedItem = item;
	}

	close() {
		this.selectedItem = null;
	}

	private getDayDiff(to: DateValue, from: DateValue): number {
		const utcTo = Date.UTC(to.year, to.month - 1, to.day);
		const utcFrom = Date.UTC(from.year, from.month - 1, from.day);
		return Math.round((utcTo - utcFrom) / 86400000);
	}

	/**
	 * Finds all calendar events and deadlines associated with a task,
	 * sorted chronologically (ascending).
	 */
	getAssociatedItemsForTask(
		task: TaskRecord,
		calendar?: CalendarRecord,
	): AssociatedTaskItem[] {
		const items: AssociatedTaskItem[] = [];
		const seenIds = new Set<string>();

		let defaultCourseId =
			calendar?.course_id ||
			(typeof task.calendar === "object" && !Array.isArray(task.calendar) && task.calendar !== null
				? (task.calendar as Record<string, any>).course_id
				: "") ||
			task.expand?.calendar?.course_id ||
			"";

		// 1. Task deadline (due_date or fake_due_date)
		const rawDue = task.due_date || task.fake_due_date;
		if (rawDue) {
			const parsed = getTaskLocalDate(rawDue);
			const dt = new Date(rawDue.includes(" ") ? rawDue.replace(" ", "T") : rawDue);
			if (parsed && !isNaN(dt.getTime())) {
				const deadlineId = `dl_${task.id}`;
				seenIds.add(deadlineId);
				items.push({
					id: deadlineId,
					taskId: task.id,
					title: task.name,
					itemType: "deadline",
					courseId: defaultCourseId,
					description: "",
					source_link: task.source_link || "",
					dateTime: dt,
					calDate: new CalendarDate(parsed.year, parsed.month, parsed.day),
				});
			}
		}

		// 2. Events associated with this task in dataState.events
		for (const evt of dataState.events) {
			if (evt.task === task.id || evt.expand?.task?.id === task.id) {
				if (!evt.start) continue;
				const parsed = getTaskLocalDate(evt.start, evt.allday);
				const dt = new Date(ISOIsolateStart(evt));
				if (!parsed || isNaN(dt.getTime())) continue;

				const evtId = `evt_${evt.id}`;
				if (seenIds.has(evtId)) continue;
				seenIds.add(evtId);

				let calId = evt.calendar || evt.expand?.calendar?.id || "";
				const cal = calId ? dataState.calendars.find((c) => c.id === calId) : undefined;
				const courseId = cal?.course_id || defaultCourseId;
				const itemType = evt.deadline
					? "deadline"
					: evt.announcement
						? "announcement"
						: "event";

				items.push({
					id: evtId,
					taskId: task.id,
					title: evt.title || task.name,
					itemType,
					courseId,
					description: evt.description || "",
					source_link: task.source_link || "",
					dateTime: dt,
					calDate: new CalendarDate(parsed.year, parsed.month, parsed.day),
				});
			}
		}

		// Sort chronologically (ascending)
		items.sort((a, b) => a.dateTime.getTime() - b.dateTime.getTime());
		return items;
	}

	/**
	 * Opens or cycles through associated calendar items for a given task.
	 */
	openTaskEvent(task: TaskRecord, calendar?: CalendarRecord) {
		// Ensure we are on the hero (calendar) page
		if (pageState.current !== "hero") {
			pageState.setPage("hero");
		}

		const associated = this.getAssociatedItemsForTask(task, calendar);
		if (associated.length === 0) {
			return;
		}

		let targetItem: AssociatedTaskItem;

		const current = this.selectedItem;
		const isViewingAssociated =
			current &&
			(current.taskId === task.id ||
				associated.some((a) => a.id === current.id || (current.id && a.id.includes(current.id))));

		if (isViewingAssociated) {
			// Find current index and cycle to next
			const currentIndex = associated.findIndex(
				(a) => a.id === current.id || (current.id && a.id.includes(current.id)),
			);
			if (currentIndex !== -1) {
				const nextIndex = (currentIndex + 1) % associated.length;
				targetItem = associated[nextIndex];
			} else {
				targetItem = associated[0];
			}
		} else {
			// Go to next event in the future or first event in the past
			const nowTime = Date.now();
			const futureItem = associated.find((a) => a.dateTime.getTime() >= nowTime);
			if (futureItem) {
				targetItem = futureItem;
			} else {
				targetItem = associated[0]; // first event in past
			}
		}

		// Viewport calculation
		const base = dayState.value as CalendarDate;
		const offset = themeState.calendarOffset;
		const startVisibleDate = base.subtract({ days: offset });
		const diff = this.getDayDiff(targetItem.calDate, startVisibleDate);

		let visibleColIndex: number;
		if (diff >= 0 && diff < 7) {
			// Inside current 7-day viewport: do not move calendar
			visibleColIndex = diff;
		} else {
			// Out of current viewport: change day to the date of the event
			dayState.set(targetItem.calDate);
			visibleColIndex = offset;
		}

		const side: "left" | "right" = visibleColIndex < 4 ? "right" : "left";

		this.selectedItem = {
			id: targetItem.id,
			taskId: task.id,
			title: targetItem.title,
			side,
			itemType: targetItem.itemType,
			courseId: targetItem.courseId,
			description: targetItem.description,
			source_link: targetItem.source_link,
			date: targetItem.calDate,
		};
	}
}

export const calendarSelectionState = new CalendarSelectionState();
