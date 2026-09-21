import type { DateValue } from "@internationalized/date";
import { dataState } from "$lib/dataState/dataState.svelte";
import { getCalendarMap, resolveCalendarColor } from "$lib/dataState/calendarQueries.svelte";
import { calendarVisibilityState } from "$lib/calendarVisibilityState.svelte";
import type {
	DayAllDayEvent,
	FormattedDeadline,
	FormattedAnnouncement,
	FormattedTimedEvent,
} from "$lib/components/calendar/calendarTypes";
import type { EventRecord, TaskRecord } from "$lib/dataState/dataRecordInterfaces";

export function getTaskLocalDate(
	dateStr: string,
	isAllDay?: boolean,
): { year: number; month: number; day: number } | null {
	if (!dateStr) return null;
	const trimmed = dateStr.trim();
	if (
		isAllDay ||
		/^\d{4}-\d{2}-\d{2}$/.test(trimmed) ||
		(isAllDay && /^\d{4}-\d{2}-\d{2}[ T]00:00:00/.test(trimmed))
	) {
		const datePart = trimmed.slice(0, 10);
		const [y, m, d] = datePart.split("-").map(Number);
		return { year: y, month: m, day: d };
	}
	const normalized = trimmed.includes(" ") ? trimmed.replace(" ", "T") : trimmed;
	const d = new Date(normalized);
	if (isNaN(d.getTime())) return null;
	return {
		year: d.getFullYear(),
		month: d.getMonth() + 1,
		day: d.getDate(),
	};
}

export const ISOIsolateStart = (e: EventRecord | { start: string }) =>
	e.start.includes(" ") ? e.start.replace(" ", "T") : e.start;

export const ISOIsolateEnd = (e: EventRecord | { end?: string }) =>
	e.end ? (e.end.includes(" ") ? e.end.replace(" ", "T") : e.end) : "";

/**
 * Pre-indexes announcement titles for fast O(1) membership checks.
 */
const announcementTitlesSet = $derived.by(() => {
	const set = new Set<string>();
	for (const e of dataState.events) {
		if (e.announcement && e.title) {
			set.add(e.title.toLowerCase().trim());
		}
	}
	return set;
});

const EMPTY_DEADLINES: FormattedDeadline[] = [];
const cachedDeadlinesByDate = new Map<string, { hash: string; items: FormattedDeadline[] }>();

/**
 * Pre-indexes deadlines by date key ("YYYY-M-D").
 */
const deadlinesByDate = $derived.by(() => {
	const map = new Map<string, FormattedDeadline[]>();
	const calendarMap = getCalendarMap();

	const tasksGrouped = new Map<string, TaskRecord[]>();
	for (const t of dataState.tasks) {
		const rawDue = t.due_date || t.fake_due_date;
		if (!rawDue) continue;
		if (announcementTitlesSet.has(t.name.toLowerCase().trim())) continue;

		let calId = "";
		if (t.expand?.calendar?.id) {
			calId = t.expand.calendar.id;
		} else if (Array.isArray(t.calendar)) {
			calId = t.calendar[0];
		} else if (typeof t.calendar === "string") {
			calId = t.calendar;
		}

		const effectiveCalId = calId || "unassigned";
		const cal = calId ? calendarMap.get(calId) : null;
		const calName = cal?.name || t.expand?.calendar?.name || "Other Tasks";
		if (calendarVisibilityState.isHiddenFromGrid(effectiveCalId, calName)) {
			continue;
		}

		const taskDate = getTaskLocalDate(rawDue);
		if (!taskDate) continue;
		const key = `${taskDate.year}-${taskDate.month}-${taskDate.day}`;
		let list = tasksGrouped.get(key);
		if (!list) {
			list = [];
			tasksGrouped.set(key, list);
		}
		list.push(t);
	}

	const eventsGrouped = new Map<string, EventRecord[]>();
	for (const e of dataState.events) {
		if (!e.deadline || e.announcement || !e.start) continue;

		let calId = e.calendar || e.expand?.calendar?.id || "";
		const cal = calId ? calendarMap.get(calId) : null;
		const calName = cal?.name || e.expand?.calendar?.name || "Other Tasks";
		if (calendarVisibilityState.isHiddenFromGrid(calId || "unassigned", calName)) {
			continue;
		}

		const evtDate = getTaskLocalDate(e.start, false);
		if (!evtDate) continue;
		const key = `${evtDate.year}-${evtDate.month}-${evtDate.day}`;
		let list = eventsGrouped.get(key);
		if (!list) {
			list = [];
			eventsGrouped.set(key, list);
		}
		list.push(e);
	}

	const allKeys = new Set([...tasksGrouped.keys(), ...eventsGrouped.keys()]);

	for (const key of allKeys) {
		const dayTasks = tasksGrouped.get(key) || [];
		const dayDeadlineEvents = eventsGrouped.get(key) || [];

		const dateHash =
			dayTasks.map((t) => `${t.id}:${t.status}:${t.name}:${t.due_date || t.fake_due_date}`).join("|") +
			"~" +
			dayDeadlineEvents.map((e) => `${e.id}:${e.start}:${e.title}`).join("|");

		const cached = cachedDeadlinesByDate.get(key);
		if (cached && cached.hash === dateHash) {
			map.set(key, cached.items);
			continue;
		}

		const seenNames = new Set<string>();
		const uniqueTasks = dayTasks.filter((t) => {
			const nameKey = t.name.toLowerCase().trim();
			if (seenNames.has(nameKey)) return false;
			seenNames.add(nameKey);
			return true;
		});

		const parsedTasks: FormattedDeadline[] = uniqueTasks.map((t) => {
			const rawDue = (t.due_date || t.fake_due_date)!;
			const d = new Date(rawDue);
			const hours = d.getHours();
			const minutes = d.getMinutes();
			const totalMinutes = hours * 60 + minutes;

			const isEndOfDay = hours === 23 && minutes >= 45;
			const topPercent = isEndOfDay
				? 98.5
				: Math.min(98.5, Math.max(4.0, ((1 + totalMinutes / 60) / 25) * 100));

			let calId = "";
			if (t.expand?.calendar?.id) {
				calId = t.expand.calendar.id;
			} else if (Array.isArray(t.calendar)) {
				calId = t.calendar[0];
			} else if (typeof t.calendar === "string") {
				calId = t.calendar;
			}

			const cal = calId ? calendarMap.get(calId) : undefined;
			let color = resolveCalendarColor(cal || t.expand?.calendar);
			let courseName =
				cal?.nickname ||
				cal?.name ||
				t.expand?.calendar?.nickname ||
				t.expand?.calendar?.name ||
				"Assignment";

			const dueTimeStr = d.toLocaleTimeString([], {
				hour: "numeric",
				minute: "2-digit",
			});

			return {
				id: t.id,
				task: t,
				name: t.name,
				dueTimeStr,
				status: t.status || "todo",
				priority: t.priority,
				color,
				courseName,
				isEndOfDay,
				topPercent,
				leftPercent: 0,
				widthPercent: 100,
			};
		});

		const parsedEvents: FormattedDeadline[] = dayDeadlineEvents
			.filter((e) => {
				const nameKey = e.title.toLowerCase().trim();
				if (seenNames.has(nameKey)) return false;
				seenNames.add(nameKey);
				return true;
			})
			.map((e) => {
				const rawDue = ISOIsolateStart(e);
				const d = new Date(rawDue);
				const hours = d.getHours();
				const minutes = d.getMinutes();
				const totalMinutes = hours * 60 + minutes;
				const isEndOfDay = hours === 23 && minutes >= 45;
				const topPercent = isEndOfDay
					? 98.5
					: Math.min(98.5, Math.max(4.0, ((1 + totalMinutes / 60) / 25) * 100));

				let calId = e.calendar || e.expand?.calendar?.id || "";
				const cal = calId ? calendarMap.get(calId) : undefined;
				let color = e.color || resolveCalendarColor(cal || e.expand?.calendar);
				let courseName =
					cal?.nickname ||
					cal?.name ||
					e.expand?.calendar?.nickname ||
					e.expand?.calendar?.name ||
					"Assignment";

				const dueTimeStr = d.toLocaleTimeString([], {
					hour: "numeric",
					minute: "2-digit",
				});

				return {
					id: e.id,
					event: e,
					name: e.title,
					dueTimeStr,
					status: "todo",
					priority: undefined,
					color,
					courseName,
					isEndOfDay,
					topPercent,
					leftPercent: 0,
					widthPercent: 100,
				};
			});

		const parsed: FormattedDeadline[] = [...parsedTasks, ...parsedEvents];

		const endOfDayItems = parsed.filter((p) => p.isEndOfDay);
		const daytimeItems = parsed.filter((p) => !p.isEndOfDay);

		if (endOfDayItems.length > 0) {
			const n = endOfDayItems.length;
			const width = 100 / n;
			endOfDayItems.forEach((item, idx) => {
				item.widthPercent = width;
				item.leftPercent = idx * width;
			});
		}

		daytimeItems.sort((a, b) => a.topPercent - b.topPercent);

		const daytimeClusters: FormattedDeadline[][] = [];
		let currentCluster: FormattedDeadline[] = [];

		for (const item of daytimeItems) {
			if (currentCluster.length === 0) {
				currentCluster.push(item);
			} else {
				const prev = currentCluster[currentCluster.length - 1];
				if (Math.abs(item.topPercent - prev.topPercent) <= 1.2) {
					currentCluster.push(item);
				} else {
					daytimeClusters.push(currentCluster);
					currentCluster = [item];
				}
			}
		}
		if (currentCluster.length > 0) {
			daytimeClusters.push(currentCluster);
		}

		for (const cluster of daytimeClusters) {
			const n = cluster.length;
			const width = 100 / n;
			cluster.forEach((item, idx) => {
				item.widthPercent = width;
				item.leftPercent = idx * width;
			});
		}

		const result = [...daytimeItems, ...endOfDayItems];
		cachedDeadlinesByDate.set(key, { hash: dateHash, items: result });
		map.set(key, result);
	}

	return map;
});

export function getDayDeadlines(date: DateValue): FormattedDeadline[] {
	const key = `${date.year}-${date.month}-${date.day}`;
	return deadlinesByDate.get(key) || EMPTY_DEADLINES;
}

const EMPTY_ANNOUNCEMENTS: FormattedAnnouncement[] = [];
const cachedAnnouncementsByDate = new Map<string, { hash: string; items: FormattedAnnouncement[] }>();

/**
 * Pre-indexes announcements by date key ("YYYY-M-D").
 */
const announcementsByDate = $derived.by(() => {
	const map = new Map<string, FormattedAnnouncement[]>();
	const calendarMap = getCalendarMap();

	const announcementsGrouped = new Map<string, EventRecord[]>();
	for (const e of dataState.events) {
		if (!e.announcement || !e.start) continue;

		let calId = e.calendar || e.expand?.calendar?.id || "";
		const cal = calId ? calendarMap.get(calId) : null;
		const calName = cal?.name || e.expand?.calendar?.name || "Coursework";
		if (calendarVisibilityState.isHiddenFromGrid(calId || "unassigned", calName)) {
			continue;
		}

		const evtDate = getTaskLocalDate(e.start, false);
		if (!evtDate) continue;
		const key = `${evtDate.year}-${evtDate.month}-${evtDate.day}`;
		let list = announcementsGrouped.get(key);
		if (!list) {
			list = [];
			announcementsGrouped.set(key, list);
		}
		list.push(e);
	}

	for (const [key, dayAnnouncements] of announcementsGrouped.entries()) {
		const dateHash = dayAnnouncements.map((e) => `${e.id}:${e.start}:${e.title}:${e.description || ""}`).join("|");
		const cached = cachedAnnouncementsByDate.get(key);
		if (cached && cached.hash === dateHash) {
			map.set(key, cached.items);
			continue;
		}

		const seenTitles = new Set<string>();
		const uniqueAnnouncements = dayAnnouncements.filter((e) => {
			const titleKey = e.title.toLowerCase().trim();
			if (seenTitles.has(titleKey)) return false;
			seenTitles.add(titleKey);
			return true;
		});

		const parsed: FormattedAnnouncement[] = uniqueAnnouncements.map((e) => {
			const rawStart = ISOIsolateStart(e);
			const d = new Date(rawStart);
			const hours = d.getHours();
			const minutes = d.getMinutes();
			const totalMinutes = hours * 60 + minutes;

			const isEndOfDay = hours === 23 && minutes >= 45;
			const topPercent = isEndOfDay
				? 98.5
				: Math.min(98.5, Math.max(4.0, ((1 + totalMinutes / 60) / 25) * 100));

			let calId = e.calendar || e.expand?.calendar?.id || "";
			const cal = calId ? calendarMap.get(calId) : undefined;
			let color = e.color || resolveCalendarColor(cal || e.expand?.calendar);
			let courseName =
				cal?.nickname ||
				cal?.name ||
				e.expand?.calendar?.nickname ||
				e.expand?.calendar?.name ||
				"Announcement";

			const timeStr = d.toLocaleTimeString([], {
				hour: "numeric",
				minute: "2-digit",
			});

			return {
				id: e.id,
				event: e,
				title: e.title,
				timeStr,
				color,
				courseName,
				description: e.description,
				isEndOfDay,
				topPercent,
				leftPercent: 0,
				widthPercent: 100,
			};
		});

		const endOfDayItems = parsed.filter((p) => p.isEndOfDay);
		const daytimeItems = parsed.filter((p) => !p.isEndOfDay);

		if (endOfDayItems.length > 0) {
			const n = endOfDayItems.length;
			const width = 100 / n;
			endOfDayItems.forEach((item, idx) => {
				item.widthPercent = width;
				item.leftPercent = idx * width;
			});
		}

		daytimeItems.sort((a, b) => a.topPercent - b.topPercent);

		const daytimeClusters: FormattedAnnouncement[][] = [];
		let currentCluster: FormattedAnnouncement[] = [];

		for (const item of daytimeItems) {
			if (currentCluster.length === 0) {
				currentCluster.push(item);
			} else {
				const prev = currentCluster[currentCluster.length - 1];
				if (Math.abs(item.topPercent - prev.topPercent) <= 1.2) {
					currentCluster.push(item);
				} else {
					daytimeClusters.push(currentCluster);
					currentCluster = [item];
				}
			}
		}
		if (currentCluster.length > 0) {
			daytimeClusters.push(currentCluster);
		}

		for (const cluster of daytimeClusters) {
			const n = cluster.length;
			const width = 100 / n;
			cluster.forEach((item, idx) => {
				item.widthPercent = width;
				item.leftPercent = idx * width;
			});
		}

		const result = [...daytimeItems, ...endOfDayItems];
		cachedAnnouncementsByDate.set(key, { hash: dateHash, items: result });
		map.set(key, result);
	}

	return map;
});

export function getDayAnnouncements(date: DateValue): FormattedAnnouncement[] {
	const key = `${date.year}-${date.month}-${date.day}`;
	return announcementsByDate.get(key) || EMPTY_ANNOUNCEMENTS;
}

const EMPTY_ALL_DAY_EVENTS: DayAllDayEvent[] = [];
const cachedAllDayEventsByDate = new Map<string, { hash: string; items: DayAllDayEvent[] }>();

/**
 * Pre-indexes all-day events by date key ("YYYY-M-D").
 */
const allDayEventsByDate = $derived.by(() => {
	const map = new Map<string, DayAllDayEvent[]>();
	const calendarMap = getCalendarMap();
	const grouped = new Map<string, EventRecord[]>();

	for (const e of dataState.events) {
		if (!e.start || !e.allday || e.deadline || e.announcement) continue;

		let calColor: string | undefined;
		if (e.calendar) {
			const cal = calendarMap.get(e.calendar);
			calColor = resolveCalendarColor(cal || e.expand?.calendar);
			if (calendarVisibilityState.isHiddenFromGrid(e.calendar, cal?.name)) {
				continue;
			}
		}
		const evtDate = getTaskLocalDate(e.start, true);
		if (!evtDate) continue;
		const key = `${evtDate.year}-${evtDate.month}-${evtDate.day}`;
		let list = grouped.get(key);
		if (!list) {
			list = [];
			grouped.set(key, list);
		}
		list.push(e);
	}

	for (const [key, evts] of grouped.entries()) {
		const hash = evts.map((e) => `${e.id}:${e.title}:${e.color}`).join("|");
		const cached = cachedAllDayEventsByDate.get(key);
		if (cached && cached.hash === hash) {
			map.set(key, cached.items);
			continue;
		}

		const items: DayAllDayEvent[] = [];
		for (const e of evts) {
			const dedupeKey = `evt_${e.id}`;
			if (!items.some((item) => item.id === dedupeKey)) {
				const cal = e.calendar ? calendarMap.get(e.calendar) : undefined;
				const calColor = resolveCalendarColor(cal || e.expand?.calendar);
				items.push({
					id: dedupeKey,
					title: e.title,
					color: e.color || calColor || "#3b82f6",
				});
			}
		}

		cachedAllDayEventsByDate.set(key, { hash, items });
		map.set(key, items);
	}

	return map;
});

export function getAllDayEventsForDate(date: DateValue): DayAllDayEvent[] {
	const key = `${date.year}-${date.month}-${date.day}`;
	return allDayEventsByDate.get(key) || EMPTY_ALL_DAY_EVENTS;
}

const EMPTY_TIMED_EVENTS: FormattedTimedEvent[] = [];
const cachedTimedEventsByDate = new Map<string, { hash: string; items: FormattedTimedEvent[] }>();

/**
 * Pre-indexes timed events with column clustering and layout geometry.
 */
const timedEventsByDate = $derived.by(() => {
	const map = new Map<string, FormattedTimedEvent[]>();
	const calendarMap = getCalendarMap();

	const taskMap = new Map<string, TaskRecord>();
	for (const t of dataState.tasks) {
		taskMap.set(t.id, t);
	}

	const dateItemsMap = new Map<string, EventRecord[]>();

	for (const pe of dataState.events) {
		if (pe.allday || !pe.start || pe.deadline || pe.announcement) continue;

		let calId = pe.calendar || pe.expand?.calendar?.id || "";
		const cal = calId ? calendarMap.get(calId) : undefined;
		const calName =
			cal?.name ||
			pe.expand?.calendar?.name ||
			"Coursework";

		if (calId && calendarVisibilityState.isHiddenFromGrid(calId, calName)) {
			continue;
		}

		const evtDate = getTaskLocalDate(pe.start, false);
		if (!evtDate) continue;
		const key = `${evtDate.year}-${evtDate.month}-${evtDate.day}`;
		let entry = dateItemsMap.get(key);
		if (!entry) {
			entry = [];
			dateItemsMap.set(key, entry);
		}
		entry.push(pe);
	}

	for (const [key, peItems] of dateItemsMap.entries()) {
		const dateHash = peItems
			.map((pe) => {
				const taskId = pe.task;
				const linkedTask = taskId ? (taskMap.get(taskId) || pe.expand?.task) : pe.expand?.task;
				return `${pe.id}:${pe.start}:${pe.end}:${pe.title}:${linkedTask?.status}`;
			})
			.join("|");

		const cached = cachedTimedEventsByDate.get(key);
		if (cached && cached.hash === dateHash) {
			map.set(key, cached.items);
			continue;
		}

		const seenKeys = new Set<string>();
		interface RawTimedItem extends FormattedTimedEvent {
			startMin: number;
			endMin: number;
			colIndex: number;
		}
		const items: RawTimedItem[] = [];

		for (const pe of peItems) {
			let calId = pe.calendar || pe.expand?.calendar?.id || "";
			const cal = calId ? calendarMap.get(calId) : undefined;
			const calName =
				cal?.nickname ||
				cal?.name ||
				pe.expand?.calendar?.nickname ||
				pe.expand?.calendar?.name ||
				(pe.google_event_id ? "Google Calendar" : "Coursework");

			const dedupeKey = `evt_${pe.id}`;
			if (seenKeys.has(dedupeKey)) continue;
			seenKeys.add(dedupeKey);

			const startIso = ISOIsolateStart(pe);
			const endIso = ISOIsolateEnd(pe);
			const startD = new Date(startIso);
			const endD = endIso ? new Date(endIso) : new Date(startD.getTime() + 60 * 60 * 1000);
			const startMin = startD.getHours() * 60 + startD.getMinutes();
			const durationMin = Math.max(25, (endD.getTime() - startD.getTime()) / 60000);
			const endMin = startMin + durationMin;

			const topPercent = Math.min(95, Math.max(4.0, ((1 + startMin / 60) / 25) * 100));
			const heightPercent = Math.max(2.0, Math.min(25, (durationMin / 60 / 25) * 100));

			const calColor = resolveCalendarColor(cal || pe.expand?.calendar);
			const color = pe.color || calColor;
			const timeStr = `${startD.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" })} - ${endD.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" })}`;

			const isTaskBlock = Boolean(pe.task || pe.expand?.task);
			const rawTaskVal: any = pe.task;
			let taskId = "";
			if (typeof rawTaskVal === "string") {
				taskId = rawTaskVal;
			} else if (Array.isArray(rawTaskVal) && rawTaskVal.length > 0) {
				taskId = typeof rawTaskVal[0] === "string" ? rawTaskVal[0] : rawTaskVal[0]?.id || "";
			} else if (rawTaskVal && typeof rawTaskVal === "object" && "id" in rawTaskVal) {
				taskId = rawTaskVal.id;
			} else if (pe.expand?.task?.id) {
				taskId = pe.expand.task.id;
			}

			const linkedTask = taskId ? (taskMap.get(taskId) || pe.expand?.task) : pe.expand?.task;
			const isTaskDone = Boolean(isTaskBlock && linkedTask && linkedTask.status === "done");

			const source: "google" | "canvas" | "internal" = pe.google_event_id
				? "google"
				: isTaskBlock
					? "canvas"
					: (cal?.source as any) || "internal";

			items.push({
				id: dedupeKey,
				rawId: pe.id,
				title: pe.title,
				timeStr,
				calendarName: calName,
				color,
				calendarColor: calColor,
				topPercent,
				heightPercent,
				startMin,
				endMin,
				colIndex: 0,
				leftPercent: 0,
				widthPercent: 100,
				description: pe.description,
				isTaskBlock,
				isTaskDone,
				taskId: taskId || pe.task,
				source,
				rawEvent: pe,
			});
		}

		if (items.length === 0) continue;

		items.sort((a, b) => a.startMin - b.startMin || b.endMin - b.startMin - (a.endMin - a.startMin));

		const clusters: RawTimedItem[][] = [];
		let currentCluster: RawTimedItem[] = [];
		let clusterEndMin = -1;

		for (const item of items) {
			if (currentCluster.length === 0) {
				currentCluster.push(item);
				clusterEndMin = item.endMin;
			} else if (item.startMin < clusterEndMin) {
				currentCluster.push(item);
				clusterEndMin = Math.max(clusterEndMin, item.endMin);
			} else {
				clusters.push(currentCluster);
				currentCluster = [item];
				clusterEndMin = item.endMin;
			}
		}
		if (currentCluster.length > 0) {
			clusters.push(currentCluster);
		}

		for (const cluster of clusters) {
			const columns: RawTimedItem[][] = [];
			for (const item of cluster) {
				let placed = false;
				for (let c = 0; c < columns.length; c++) {
					const lastInCol = columns[c][columns[c].length - 1];
					if (lastInCol.endMin <= item.startMin) {
						columns[c].push(item);
						item.colIndex = c;
						placed = true;
						break;
					}
				}
				if (!placed) {
					item.colIndex = columns.length;
					columns.push([item]);
				}
			}

			const totalCols = columns.length;
			for (let c = 0; c < totalCols; c++) {
				for (const item of columns[c]) {
					item.leftPercent = (c / totalCols) * 100;
					item.widthPercent = 100 / totalCols;
				}
			}

			for (const item of cluster) {
				let colSpan = 1;
				while (item.colIndex + colSpan < totalCols) {
					const targetCol = columns[item.colIndex + colSpan];
					const hasOverlap = targetCol.some(
						(other) => item.startMin < other.endMin && other.startMin < item.endMin,
					);
					if (hasOverlap) break;
					colSpan++;
				}

				item.leftPercent = (item.colIndex / totalCols) * 100;
				item.widthPercent = (colSpan / totalCols) * 100;
			}
		}

		cachedTimedEventsByDate.set(key, { hash: dateHash, items });
		map.set(key, items);
	}

	return map;
});

export function getDayTimedEvents(date: DateValue): FormattedTimedEvent[] {
	const key = `${date.year}-${date.month}-${date.day}`;
	return timedEventsByDate.get(key) || EMPTY_TIMED_EVENTS;
}
