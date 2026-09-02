<script lang="ts">
	import ChevronLeft from "@lucide/svelte/icons/chevron-left";
	import ChevronRight from "@lucide/svelte/icons/chevron-right";
	import RefreshCwIcon from "@lucide/svelte/icons/refresh-cw";
	import MegaphoneIcon from "@lucide/svelte/icons/megaphone";
	import X from "@lucide/svelte/icons/x";
	import Button from "$lib/components/ui/button/button.svelte";
	import { dayState } from "$lib/dayState.svelte";
	import { canvasState } from "$lib/canvasState.svelte";
	import { pb } from "$lib/pocketbase";
	import { authState } from "$lib/authState.svelte";
	import { calendarVisibilityState } from "$lib/calendarVisibilityState.svelte";
	import { googleCalendarState } from "$lib/googleCalendarState.svelte";
	import { themeState } from "$lib/themeState.svelte";
	import { dragState } from "$lib/dragState.svelte";
	import { syncState } from "$lib/syncState.svelte";
	import {
		type EventRecord,
		type CalendarRecord,
		type TaskRecord,
		getCalendarRecords,
		getTaskRecords,
		getEventRecords,
		addEventRecord,
	} from "$lib/pocketbaseActions";
	import {
		isToday,
		getLocalTimeZone,
		type CalendarDate,
		type DateValue,
	} from "@internationalized/date";

	function getDayDiff(to: DateValue, from: DateValue): number {
		const utcTo = Date.UTC(to.year, to.month - 1, to.day);
		const utcFrom = Date.UTC(from.year, from.month - 1, from.day);
		return Math.round((utcTo - utcFrom) / 86400000);
	}

	let pbTasks = $state<TaskRecord[]>([]);
	let pbCalendars = $state<CalendarRecord[]>([]);
	let pbEvents = $state<EventRecord[]>([]);
	let now = $state(new Date());
	let dragHoverState = $state<DragHoverState | null>(null);
	let baseDate = $state<DateValue>(dayState.value);
	let prevDate = $state<DateValue>(dayState.value);
	let offsetDays = $state(0);
	let isAnimating = $state(false);

	$effect(() => {
		const timer = setInterval(() => {
			now = new Date();
		}, 1000); // 1-second interval to follow exact live time
		return () => clearInterval(timer);
	});

	const currentDayTimePercent = $derived.by(() => {
		const hours = now.getHours();
		const minutes = now.getMinutes();
		const seconds = now.getSeconds();
		const totalMinutes = hours * 60 + minutes + seconds / 60;
		// 25 rows in grid; Row 1 starts at 12 AM (0 minutes)
		return ((1 + totalMinutes / 60) / 25) * 100;
	});

	const todayColumnIndex = $derived.by(() => {
		const offset = themeState.calendarOffset;
		for (let i = 0; i < 7; i++) {
			const colDate = (baseDate as CalendarDate).add({ days: i - offset });
			if (isToday(colDate, getLocalTimeZone())) {
				return i;
			}
		}
		return -1;
	});

	async function loadCalendarData() {
		if (!pb.authStore.isValid) return;
		try {
			const [cals, tks, evts] = await Promise.all([
				getCalendarRecords(),
				getTaskRecords(),
				getEventRecords(),
			]);
			pbCalendars = cals;
			pbTasks = tks;
			pbEvents = evts;
		} catch (err) {
			console.warn("Failed to load calendar data:", err);
		}
	}

	let reloadDebounceTimer: ReturnType<typeof setTimeout> | null = null;
	function debouncedLoadCalendarData() {
		if (reloadDebounceTimer) clearTimeout(reloadDebounceTimer);
		reloadDebounceTimer = setTimeout(() => {
			loadCalendarData();
		}, 250);
	}

	$effect(() => {
		if (authState.isAuthenticated) {
			loadCalendarData();
		}
	});

	$effect(() => {
		if (canvasState.lastSynced) {
			debouncedLoadCalendarData();
		}
	});

	$effect(() => {
		if (typeof window === "undefined" || !pb.authStore.isValid) return;

		const unsubTasks = pb
			.collection("tasks")
			.subscribe("*", () => {
				debouncedLoadCalendarData();
			})
			.catch(() => () => {});
		const unsubCals = pb
			.collection("calendars")
			.subscribe("*", () => {
				debouncedLoadCalendarData();
				googleCalendarState.loadFromPocketBase();
			})
			.catch(() => () => {});
		const unsubEvts = pb
			.collection("events")
			.subscribe("*", () => {
				debouncedLoadCalendarData();
				googleCalendarState.loadFromPocketBase();
			})
			.catch(() => () => {});
		return () => {
			if (reloadDebounceTimer) clearTimeout(reloadDebounceTimer);
			unsubTasks.then((unsub) => unsub && unsub());
			unsubCals.then((unsub) => unsub && unsub());
			unsubEvts.then((unsub) => unsub && unsub());
		};
	});

	// All personal Google Calendars to load in memory (hiding a calendar does not stop it from loading in)
	const allGoogleCalendarIds = $derived.by(() => {
		if (!googleCalendarState.isConnected) return [];
		return googleCalendarState.readOnlyCalendars.map((c) => c.id);
	});

	let lastFetchedGCalKey = "";
	$effect(() => {
		const key = allGoogleCalendarIds.slice().sort().join(",");
		if (googleCalendarState.isConnected && key) {
			if (key !== lastFetchedGCalKey) {
				lastFetchedGCalKey = key;
				googleCalendarState.fetchReadOnlyEvents(allGoogleCalendarIds);
			}
		}
	});

	async function handleGlobalSync() {
		if (syncState.isAnySyncing) return;
		try {
			await syncState.syncAll();
			await loadCalendarData();
		} catch (e) {
			console.warn("Global sync error:", e);
		}
	}

	const allTasks = $derived.by(() => {
		const map = new Map<string, TaskRecord>();
		for (const t of pbTasks) {
			map.set(t.id, t);
		}
		for (const t of canvasState.tasks) {
			if (!map.has(t.id)) {
				map.set(t.id, t as any);
			}
		}
		return Array.from(map.values());
	});

	const calendarMap = $derived.by(() => {
		const map = new Map<string, CalendarRecord>();
		for (const c of pbCalendars) {
			map.set(c.id, c);
		}
		for (const c of canvasState.calendars) {
			if (!map.has(c.id)) {
				map.set(c.id, c);
			}
		}
		for (const t of allTasks) {
			if (t.expand?.calendar && !map.has(t.expand.calendar.id)) {
				map.set(t.expand.calendar.id, t.expand.calendar);
			}
		}
		return map;
	});

	interface FormattedDeadline {
		id: string;
		task?: TaskRecord;
		event?: EventRecord;
		name: string;
		dueTimeStr: string;
		status: string;
		priority?: string;
		color: string;
		courseName: string;
		isEndOfDay: boolean;
		topPercent: number;
		leftPercent: number;
		widthPercent: number;
	}

	interface FormattedAnnouncement {
		id: string;
		event: EventRecord;
		title: string;
		timeStr: string;
		color: string;
		courseName: string;
		description?: string;
		isEndOfDay: boolean;
		topPercent: number;
		leftPercent: number;
		widthPercent: number;
	}

	function getTaskLocalDate(
		dateStr: string,
		isAllDay?: boolean,
	): { year: number; month: number; day: number } | null {
		if (!dateStr) return null;
		const trimmed = dateStr.trim();
		// If explicitly all-day OR date-only YYYY-MM-DD OR midnight representation: extract calendar date directly
		if (
			isAllDay ||
			/^\d{4}-\d{2}-\d{2}$/.test(trimmed) ||
			(isAllDay && /^\d{4}-\d{2}-\d{2}[ T]00:00:00/.test(trimmed))
		) {
			const datePart = trimmed.slice(0, 10);
			const [y, m, d] = datePart.split("-").map(Number);
			return { year: y, month: m, day: d };
		}
		const normalized = trimmed.includes(" ")
			? trimmed.replace(" ", "T")
			: trimmed;
		const d = new Date(normalized);
		if (isNaN(d.getTime())) return null;
		return {
			year: d.getFullYear(),
			month: d.getMonth() + 1,
			day: d.getDate(),
		};
	}

	// Pre-index announcement titles for O(1) membership check
	const announcementTitlesSet = $derived.by(() => {
		const set = new Set<string>();
		for (const e of pbEvents) {
			if (e.announcement && e.title) {
				set.add(e.title.toLowerCase().trim());
			}
		}
		return set;
	});

	// Pre-index deadlines by date key ("YYYY-M-D") - calculated ONCE when data changes, not on 39 columns per frame
	const deadlinesByDate = $derived.by(() => {
		const map = new Map<string, FormattedDeadline[]>();

		const tasksGrouped = new Map<string, TaskRecord[]>();
		for (const t of allTasks) {
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
			if (
				calendarVisibilityState.isHiddenFromGrid(
					effectiveCalId,
					cal?.calendar_id || calName,
				)
			) {
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
		for (const e of pbEvents) {
			if (!e.deadline || e.announcement || !e.start) continue;

			let calId = e.calendar || e.expand?.calendar?.id || "";
			const cal = calId ? calendarMap.get(calId) : null;
			const calName = cal?.name || e.expand?.calendar?.name || "Other Tasks";
			if (
				calendarVisibilityState.isHiddenFromGrid(
					calId || "unassigned",
					cal?.calendar_id || calName,
				)
			) {
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
				let color = cal?.color || t.expand?.calendar?.color;
				let courseName =
					cal?.nickname ||
					cal?.name ||
					t.expand?.calendar?.nickname ||
					t.expand?.calendar?.name;

				const courseMatch = canvasState.courses.find(
					(c) =>
						(cal?.course_id && String(c.id) === String(cal.course_id)) ||
						String(c.id) === calId ||
						c.name === courseName ||
						(c.original_name && c.original_name === courseName),
				);
				if (courseMatch) {
					if (courseMatch.color || courseMatch.backgroundColor) {
						color = courseMatch.color || courseMatch.backgroundColor;
					}
					if (!courseName) courseName = courseMatch.name;
				}

				if (!color) color = "#3b82f6";
				if (!courseName) courseName = "Assignment";

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
						: Math.min(
								98.5,
								Math.max(4.0, ((1 + totalMinutes / 60) / 25) * 100),
							);

					let calId = e.calendar || e.expand?.calendar?.id || "";
					const cal = calId ? calendarMap.get(calId) : undefined;
					let color = e.color || cal?.color || e.expand?.calendar?.color;
					let courseName =
						cal?.nickname ||
						cal?.name ||
						e.expand?.calendar?.nickname ||
						e.expand?.calendar?.name;

					const dueTimeStr = d.toLocaleTimeString([], {
						hour: "numeric",
						minute: "2-digit",
					});

					if (!color) color = "#3b82f6";
					if (!courseName) courseName = "Assignment";

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
			// REFACTOR
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

			map.set(key, [...daytimeItems, ...endOfDayItems]);
		}

		return map;
	});

	function getDayDeadlines(date: DateValue): FormattedDeadline[] {
		const key = `${date.year}-${date.month}-${date.day}`;
		return deadlinesByDate.get(key) || [];
	}
	const ISOIsolateStart = (e: EventRecord) =>
		e.start.includes(" ") ? e.start.replace(" ", "T") : e.start;
	const ISOIsolateEnd = (e: EventRecord) =>
		e.end ? (e.end.includes(" ") ? e.end.replace(" ", "T") : e.end) : "";
	// Pre-index announcements by date key ("YYYY-M-D") - calculated ONCE when data changes, not on 39 columns per frame
	const announcementsByDate = $derived.by(() => {
		const map = new Map<string, FormattedAnnouncement[]>();

		const announcementsGrouped = new Map<string, EventRecord[]>();
		for (const e of pbEvents) {
			if (!e.announcement || !e.start) continue;

			let calId = e.calendar || e.expand?.calendar?.id || "";
			const cal = calId ? calendarMap.get(calId) : null;
			const calName = cal?.name || e.expand?.calendar?.name || "Coursework";
			if (
				calendarVisibilityState.isHiddenFromGrid(
					calId || "unassigned",
					cal?.calendar_id || calName,
				)
			) {
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
				let color = e.color || cal?.color || e.expand?.calendar?.color;
				let courseName =
					cal?.nickname ||
					cal?.name ||
					e.expand?.calendar?.nickname ||
					e.expand?.calendar?.name;

				const courseMatch = canvasState.courses.find(
					(c) =>
						(cal?.course_id && String(c.id) === String(cal.course_id)) ||
						String(c.id) === calId ||
						c.name === courseName ||
						(c.original_name && c.original_name === courseName),
				);
				if (courseMatch) {
					if (courseMatch.color || courseMatch.backgroundColor) {
						color = courseMatch.color || courseMatch.backgroundColor;
					}
					if (!courseName) courseName = courseMatch.name;
				}

				if (!color) color = "#3b82f6";
				if (!courseName) courseName = "Announcement";

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
			//Refactor
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

			map.set(key, [...daytimeItems, ...endOfDayItems]);
		}

		return map;
	});

	function getDayAnnouncements(date: DateValue): FormattedAnnouncement[] {
		const key = `${date.year}-${date.month}-${date.day}`;
		return announcementsByDate.get(key) || [];
	}

	interface DayAllDayEvent {
		id: string;
		title: string;
		color?: string;
	}

	const allDayEventsByDate = $derived.by(() => {
		const map = new Map<string, DayAllDayEvent[]>();

		// 1. Process pbEvents
		for (const e of pbEvents) {
			if (!e.start || !e.allday || e.deadline || e.announcement) continue;
			let calColor: string | undefined;
			if (e.calendar) {
				const cal = calendarMap.get(e.calendar);
				calColor = cal?.color || e.expand?.calendar?.color;
				if (
					calendarVisibilityState.isHiddenFromGrid(
						e.calendar,
						cal?.calendar_id || cal?.name,
					)
				) {
					continue;
				}
				if (cal?.source === "canvas" || cal?.course_id) {
					const courseMatch = canvasState.courses.find(
						(c) =>
							(cal?.course_id && String(c.id) === String(cal.course_id)) ||
							String(c.id) === e.calendar ||
							c.name === cal?.name ||
							(c.original_name && c.original_name === cal?.name),
					);
					if (
						courseMatch &&
						(courseMatch.color || courseMatch.backgroundColor)
					) {
						calColor = courseMatch.color || courseMatch.backgroundColor;
					}
				}
			}
			const evtDate = getTaskLocalDate(e.start, true);
			if (!evtDate) continue;
			const key = `${evtDate.year}-${evtDate.month}-${evtDate.day}`;
			let list = map.get(key);
			if (!list) {
				list = [];
				map.set(key, list);
			}
			const dedupeKey = `pb_${e.id}`;
			if (!list.some((item) => item.id === dedupeKey)) {
				list.push({
					id: dedupeKey,
					title: e.title,
					color: calColor || "#3b82f6",
				});
			}
		}

		// 2. Process googleCalendarState.readOnlyEvents
		if (googleCalendarState.isConnected) {
			for (const ge of googleCalendarState.readOnlyEvents) {
				if (!ge.allday || !ge.start) continue;
				if (
					calendarVisibilityState.isHiddenFromGrid(
						ge.calendarId,
						(ge as any).pbCalendarId,
					)
				)
					continue;
				const evtDate = getTaskLocalDate(ge.start, true);
				if (!evtDate) continue;
				const key = `${evtDate.year}-${evtDate.month}-${evtDate.day}`;
				let list = map.get(key);
				if (!list) {
					list = [];
					map.set(key, list);
				}
				const dedupeKey = `gcal_${ge.id}`;
				if (!list.some((item) => item.id === dedupeKey)) {
					const gCal = googleCalendarState.calendars.find(
						(c) => c.id === ge.calendarId,
					);
					list.push({
						id: dedupeKey,
						title: ge.title,
						color: ge.color || gCal?.backgroundColor || "#3b82f6",
					});
				}
			}
		}

		return map;
	});

	function getAllDayEventsForDate(date: DateValue): DayAllDayEvent[] {
		const key = `${date.year}-${date.month}-${date.day}`;
		return allDayEventsByDate.get(key) || [];
	}

	interface FormattedTimedEvent {
		id: string;
		rawId?: string;
		title: string;
		timeStr: string;
		calendarName: string;
		color: string;
		topPercent: number;
		heightPercent: number;
		colIndex: number;
		leftPercent: number;
		widthPercent: number;
		description?: string;
		isTaskBlock?: boolean;
		taskId?: string;
		source?: "google" | "canvas" | "internal";
		rawEvent?: EventRecord;
	}

	const timedEventsByDate = $derived.by(() => {
		const map = new Map<string, FormattedTimedEvent[]>();

		const dateItemsMap = new Map<
			string,
			{
				geItems: (typeof googleCalendarState.readOnlyEvents)[number][];
				peItems: EventRecord[];
			}
		>();

		// 1. Google personal calendar timed events
		if (
			googleCalendarState.isConnected &&
			googleCalendarState.readOnlyEvents.length > 0
		) {
			for (const ge of googleCalendarState.readOnlyEvents) {
				if (ge.allday || !ge.start) continue;
				if (
					calendarVisibilityState.isHiddenFromGrid(
						ge.calendarId,
						(ge as any).pbCalendarId,
					)
				) {
					continue;
				}
				const evtDate = getTaskLocalDate(ge.start, false);
				if (!evtDate) continue;
				const key = `${evtDate.year}-${evtDate.month}-${evtDate.day}`;
				let entry = dateItemsMap.get(key);
				if (!entry) {
					entry = { geItems: [], peItems: [] };
					dateItemsMap.set(key, entry);
				}
				entry.geItems.push(ge);
			}
		}

		// 2. PocketBase timed events
		for (const pe of pbEvents) {
			if (pe.allday || !pe.start || pe.deadline || pe.announcement) continue;
			if (pe.google_event_id && googleCalendarState.isConnected) continue;

			let calId = pe.calendar || pe.expand?.calendar?.id || "";
			const cal = calId ? calendarMap.get(calId) : undefined;
			const calName =
				cal?.nickname ||
				cal?.name ||
				pe.expand?.calendar?.nickname ||
				pe.expand?.calendar?.name ||
				"Coursework";

			if (
				calId &&
				calendarVisibilityState.isHiddenFromGrid(
					calId,
					cal?.calendar_id || calName,
				)
			) {
				continue;
			}

			const evtDate = getTaskLocalDate(pe.start, false);
			if (!evtDate) continue;
			const key = `${evtDate.year}-${evtDate.month}-${evtDate.day}`;
			let entry = dateItemsMap.get(key);
			if (!entry) {
				entry = { geItems: [], peItems: [] };
				dateItemsMap.set(key, entry);
			}
			entry.peItems.push(pe);
		}

		// Format and cluster each date with timed items ONCE
		for (const [key, { geItems, peItems }] of dateItemsMap.entries()) {
			const seenKeys = new Set<string>();
			interface RawTimedItem extends FormattedTimedEvent {
				startMin: number;
				endMin: number;
				colIndex: number;
			}
			const items: RawTimedItem[] = [];

			for (const ge of geItems) {
				const dedupeKey = `gcal_${ge.id}_${ge.start}`;
				if (seenKeys.has(dedupeKey)) continue;
				seenKeys.add(dedupeKey);

				const startIso = ISOIsolateStart(ge);
				const endIso = ISOIsolateEnd(ge);
				const startD = new Date(startIso);
				const endD = endIso
					? new Date(endIso)
					: new Date(startD.getTime() + 60 * 60 * 1000);
				const startMin = startD.getHours() * 60 + startD.getMinutes();
				const durationMin = Math.max(
					25,
					(endD.getTime() - startD.getTime()) / 60000,
				);
				const endMin = startMin + durationMin;

				const topPercent = Math.min(
					95,
					Math.max(4.0, ((1 + startMin / 60) / 25) * 100),
				);
				const heightPercent = Math.max(
					2.0,
					Math.min(25, (durationMin / 60 / 25) * 100),
				);

				const cal = googleCalendarState.calendars.find(
					(c) => c.id === ge.calendarId,
				);
				const calName = cal?.nickname || cal?.summary || "Google Calendar";
				const color = ge.color || cal?.backgroundColor || "#3b82f6";

				const timeStr = `${startD.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" })} - ${endD.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" })}`;

				items.push({
					id: dedupeKey,
					title: ge.title,
					timeStr,
					calendarName: calName,
					color,
					topPercent,
					heightPercent,
					startMin,
					endMin,
					colIndex: 0,
					leftPercent: 0,
					widthPercent: 100,
					description: ge.description,
					isTaskBlock: false,
					source: "google",
				});
			}

			for (const pe of peItems) {
				let calId = pe.calendar || pe.expand?.calendar?.id || "";
				const cal = calId ? calendarMap.get(calId) : undefined;
				const calName =
					cal?.nickname ||
					cal?.name ||
					pe.expand?.calendar?.nickname ||
					pe.expand?.calendar?.name ||
					"Coursework";

				const dedupeKey = `pb_${pe.id}`;
				if (seenKeys.has(dedupeKey)) continue;
				seenKeys.add(dedupeKey);

				const startIso = ISOIsolateStart(pe);
				const endIso = ISOIsolateEnd(pe);
				const startD = new Date(startIso);
				const endD = endIso
					? new Date(endIso)
					: new Date(startD.getTime() + 60 * 60 * 1000);
				const startMin = startD.getHours() * 60 + startD.getMinutes();
				const durationMin = Math.max(
					25,
					(endD.getTime() - startD.getTime()) / 60000,
				);
				const endMin = startMin + durationMin;

				const topPercent = Math.min(
					95,
					Math.max(4.0, ((1 + startMin / 60) / 25) * 100),
				);
				const heightPercent = Math.max(
					2.0,
					Math.min(25, (durationMin / 60 / 25) * 100),
				);

				let color = "";
				if (cal?.source === "canvas" || cal?.course_id || pe.task) {
					const courseMatch = canvasState.courses.find(
						(c) =>
							(cal?.course_id && String(c.id) === String(cal.course_id)) ||
							c.name === cal?.name ||
							(c.original_name && c.original_name === cal?.name),
					);
					if (
						courseMatch &&
						(courseMatch.color || courseMatch.backgroundColor)
					) {
						color = (courseMatch.color || courseMatch.backgroundColor)!;
					} else if (cal?.color) {
						color = cal.color;
					} else if (pe.expand?.calendar?.color) {
						color = pe.expand.calendar.color;
					}
				}
				if (!color) {
					color =
						pe.color || cal?.color || pe.expand?.calendar?.color || "#3b82f6";
				}
				const timeStr = `${startD.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" })} - ${endD.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" })}`;

				items.push({
					id: dedupeKey,
					rawId: pe.id,
					title: pe.title,
					timeStr,
					calendarName: calName,
					color,
					topPercent,
					heightPercent,
					startMin,
					endMin,
					colIndex: 0,
					leftPercent: 0,
					widthPercent: 100,
					description: pe.description,
					isTaskBlock: Boolean(pe.task),
					taskId: pe.task,
					source: pe.task ? "canvas" : (cal?.source as any) || "internal",
					rawEvent: pe,
				});
			}

			if (items.length === 0) continue;

			items.sort(
				(a, b) =>
					a.startMin - b.startMin ||
					b.endMin - b.startMin - (a.endMin - a.startMin),
			);

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
							(other) =>
								item.startMin < other.endMin && other.startMin < item.endMin,
						);
						if (hasOverlap) break;
						colSpan++;
					}

					item.leftPercent = (item.colIndex / totalCols) * 100;
					item.widthPercent = (colSpan / totalCols) * 100;
				}
			}

			map.set(key, items);
		}

		return map;
	});

	function getDayTimedEvents(date: DateValue): FormattedTimedEvent[] {
		const key = `${date.year}-${date.month}-${date.day}`;
		return timedEventsByDate.get(key) || [];
	}

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

	async function handleDrop(e: DragEvent, date: DateValue) {
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
			!calendarMap.has(targetCalId)
		) {
			const fallbackCal =
				pbCalendars.find((c) => c.id !== "unassigned") || pbCalendars[0];
			if (fallbackCal) {
				targetCalId = fallbackCal.id;
			}
		}

		const tempId = `temp_${Date.now()}`;
		const calRecord = targetCalId ? calendarMap.get(targetCalId) : undefined;
		const taskRecord = allTasks.find((t) => t.id === payload.taskId);

		const optimisticRecord: EventRecord = {
			id: tempId,
			calendar: targetCalId,
			task: payload.taskId,
			title: payload.taskName,
			start: startDate.toISOString(),
			end: endDate.toISOString(),
			allday: false,
			deadline: false,
			color: "", // Inherits from calendar
			description: `Work session for ${payload.taskName}`,
			expand: {
				calendar: calRecord,
				task: taskRecord,
			},
		};

		// 1. Optimistic update
		pbEvents = [...pbEvents, optimisticRecord];

		// 2. Persist to PocketBase
		try {
			const created = await addEventRecord({
				calendar: targetCalId,
				task: payload.taskId,
				title: payload.taskName,
				start: startDate.toISOString(),
				end: endDate.toISOString(),
				allday: false,
				deadline: false,
				color: "", // Inherits from calendar
				description: `Work session for ${payload.taskName}`,
			});

			pbEvents = pbEvents.map((ev) => (ev.id === tempId ? created : ev));
		} catch (err) {
			console.error("Failed to create work session event:", err);
			// Rollback on failure
			pbEvents = pbEvents.filter((ev) => ev.id !== tempId);
		}
	}

	async function deleteEvent(evt: FormattedTimedEvent) {
		if (!evt.rawId || !evt.isTaskBlock) return;
		const idToDelete = evt.rawId;
		const prev = pbEvents;
		pbEvents = pbEvents.filter((e) => e.id !== idToDelete);

		if (idToDelete.startsWith("temp_")) return;

		try {
			await pb.collection("events").delete(idToDelete);
		} catch (err) {
			console.error("Failed to delete event:", err);
			pbEvents = prev;
		}
	}

	const BUFFER = 16;
	const VISIBLE_COUNT = 7;
	const TOTAL_COUNT = BUFFER + VISIBLE_COUNT + BUFFER;
	const ALL_OFFSETS = $derived(
		Array.from(
			{ length: TOTAL_COUNT },
			(_, i) => i - (BUFFER + themeState.calendarOffset),
		),
	);
	// Watch dayState.value changes
	$effect(() => {
		const targetDate = dayState.value;
		const delta = getDayDiff(targetDate, prevDate);
		prevDate = targetDate;

		if (delta === 0) return;

		// If a single user action jumped more than 4 days (e.g. from calendar picker):
		// Instantly snap without animation
		if (Math.abs(delta) > 15) {
			isAnimating = false;
			offsetDays = 0;
			baseDate = targetDate;
			return;
		}

		const newOffset = offsetDays + delta;

		if (Math.abs(newOffset) > BUFFER - 2) {
			isAnimating = false;
			offsetDays = 0;
			baseDate = targetDate;
			return;
		}

		isAnimating = true;
		offsetDays = newOffset;
	});

	function handleTransitionEnd(e: TransitionEvent) {
		// Only react to transform transitions on the track itself
		if (e.target !== e.currentTarget || e.propertyName !== "transform") return;

		if (offsetDays !== 0) {
			isAnimating = false;
			baseDate = (baseDate as CalendarDate).add({ days: offsetDays });
			offsetDays = 0;
		}
	}

	const days = $derived(
		ALL_OFFSETS.map((offset) => {
			const date = (baseDate as CalendarDate).add({ days: offset });
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
				offset,
				weekday,
				dayNumber: date.day,
				monthName,
				isSelected,
				isToday: isCurrentToday,
			};
		}),
	);

	const transformPercent = $derived(
		((BUFFER + offsetDays) * 100) / TOTAL_COUNT,
	);

	// 25 rows: Row 0 is unlabeled (previous 11 PM), Row 1 starts at 12 AM through 11 PM at Row 24
	const HOURS = Array.from({ length: 25 }, (_, i) => {
		if (i === 0) return "";
		const hourIndex = i - 1; // 0 to 23
		if (hourIndex === 0) return "12 AM";
		if (hourIndex < 12) return `${hourIndex} AM`;
		if (hourIndex === 12) return "12 PM";
		return `${hourIndex - 12} PM`;
	});
</script>

<div class="h-full w-full flex flex-col overflow-hidden">
	<!-- Header with day switcher -->
	<header class="flex h-14 items-center gap-3 px-4 shrink-0 z-10">
		<div class="w-10 shrink-0 flex items-center justify-center">
			<Button
				variant="ghost"
				size="icon"
				class="size-8 rounded-md text-muted-foreground hover:text-foreground hover:bg-muted cursor-pointer shrink-0"
				aria-label="Previous day"
				onclick={() => dayState.dec()}
			>
				<ChevronLeft class="size-4" />
			</Button>
		</div>

		<div class="flex-1 h-full overflow-hidden relative">
			<div
				class="h-full flex flex-row items-center"
				style="width: calc({TOTAL_COUNT} / {VISIBLE_COUNT} * 100%); transform: translate3d(-{transformPercent}%, 0, 0); {isAnimating
					? 'transition: transform 370ms cubic-bezier(0.16, 1, 0.3, 1);'
					: 'transition: none;'}"
				ontransitionend={handleTransitionEnd}
			>
				{#each days as item (item.date.toString())}
					<button
						type="button"
						style="flex: 0 0 calc(100% / {TOTAL_COUNT});"
						class="h-full flex flex-row items-center justify-center gap-2 text-center cursor-pointer select-none"
						onclick={() => dayState.set(item.date)}
						aria-label="{item.weekday}, {item.monthName} {item.dayNumber}"
						aria-current={item.isSelected ? "date" : undefined}
					>
						<span
							class="text-xs uppercase font-medium tracking-wide {item.isSelected
								? 'text-foreground font-semibold'
								: 'text-muted-foreground'}"
						>
							{item.weekday}
						</span>

						<div class="relative size-7 flex items-center justify-center">
							<!-- Expanding highlight from center -->
							<span
								class="absolute inset-0 rounded-md bg-primary shadow-xs origin-center pointer-events-none transition-all duration-250 ease-out {item.isSelected
									? 'scale-100 opacity-100'
									: 'scale-0 opacity-0'}"
							></span>

							<!-- Day number text -->
							<span
								class="relative z-10 text-sm font-semibold transition-colors duration-250 {item.isSelected
									? 'text-primary-foreground'
									: 'text-foreground'}"
							>
								{item.dayNumber}
							</span>

							{#if item.isToday}
								<span
									class="size-1 rounded-full absolute -top-1.5 left-1/2 -translate-x-1/2 z-10 transition-colors duration-150 {item.isSelected
										? 'bg-primary ring-1 ring-background'
										: 'bg-primary'}"
									title="Today"
								></span>
							{/if}
						</div>
					</button>
				{/each}
			</div>
		</div>

		<div class="w-10 shrink-0 flex items-center justify-center">
			<Button
				variant="ghost"
				size="icon"
				class="size-8 rounded-md text-muted-foreground hover:text-foreground hover:bg-muted cursor-pointer shrink-0"
				aria-label="Next day"
				onclick={() => dayState.inc()}
			>
				<ChevronRight class="size-4" />
			</Button>
		</div>
	</header>

	<!-- Floating All-Day Events Box (25% in upper nav, 75% in calendar, partially transparent) -->
	<div
		class="absolute top-14 left-0 right-0 z-30 flex gap-3 px-4 -translate-y-1/4 pointer-events-none"
	>
		<!-- Left: All-day label aligned with time column -->
		<div class="w-10 shrink-0 flex items-center justify-end pr-1 select-none">
			<span
				class="text-[9px] font-medium text-muted-foreground/60 uppercase tracking-tighter"
			>
				all-day
			</span>
		</div>

		<!-- Middle: 7-column neutral rounded box matching the 7 visible days -->
		<div
			class="flex-1 pointer-events-auto h-8 rounded-lg bg-muted/50 dark:bg-muted/35 backdrop-blur-md border border-border/50 shadow-xs overflow-hidden relative"
		>
			<div
				class="h-full flex flex-row"
				style="width: calc({TOTAL_COUNT} / {VISIBLE_COUNT} * 100%); transform: translate3d(-{transformPercent}%, 0, 0); {isAnimating
					? 'transition: transform 370ms cubic-bezier(0.16, 1, 0.3, 1);'
					: 'transition: none;'}"
			>
				{#each ALL_OFFSETS as offset (offset)}
					{@const colDate = (baseDate as CalendarDate).add({ days: offset })}
					{@const dayEvents = getAllDayEventsForDate(colDate)}
					<div
						style="flex: 0 0 calc(100% / {TOTAL_COUNT});"
						class="h-full border-r border-border/30 last:border-r-0 flex flex-col overflow-y-auto overflow-x-hidden [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
					>
						{#each dayEvents as evt (evt.id)}
							<div class="h-1/2 w-full shrink-0 px-1 py-[1px] box-border">
								<div
									class="h-full w-full rounded-full px-2 flex items-center justify-start gap-1 truncate text-[9px] font-medium border select-none cursor-pointer transition-all shadow-2xs"
									style="
										background-color: {evt.color || '#3b82f6'};
										border-color: {evt.color || '#3b82f6'};
									"
									title={evt.title}
								>
									<span
										class="truncate leading-none text-white drop-shadow-xs"
										style="font-weight: 600;">{evt.title}</span
									>
								</div>
							</div>
						{/each}
					</div>
				{/each}
			</div>
		</div>

		<!-- Right: Outline sync button in empty space -->
		<div
			class="w-10 shrink-0 flex items-center justify-center pointer-events-auto z-40"
		>
			<Button
				variant="outline"
				size="icon"
				class="size-8 rounded-lg border border-border bg-background hover:bg-muted text-foreground cursor-pointer shadow-sm transition-all"
				onclick={handleGlobalSync}
				disabled={syncState.isAnySyncing}
				title={syncState.isAnySyncing
					? "Syncing Canvas & Google Calendar..."
					: "Sync Canvas & Google Calendar"}
				aria-label="Sync Canvas & Google Calendar"
			>
				<RefreshCwIcon
					class="size-3.5 {syncState.isAnySyncing
						? 'animate-spin text-primary'
						: ''}"
				/>
			</Button>
		</div>
	</div>

	<!-- Viewport wrapper that DOES NOT scroll, sized to the window remaining height, masked at top and bottom -->
	<div
		class="relative flex-1 min-h-0 w-full overflow-hidden calendar-viewport-mask z-10"
	>
		<!-- Scrollable container inside the masked viewport (scrollbar hidden to match left and right padding) -->
		<div
			class="h-full w-full overflow-y-auto overflow-x-hidden [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
		>
			<div class="flex gap-3 px-4 w-full h-[140%] min-h-[750px] pt-0 pb-0">
				<!-- Part 1: Left time column (1 col, 25 rows) -->
				<div class="w-10 shrink-0 flex flex-col h-full select-none">
					{#each HOURS as hour, index}
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
						{#each HOURS as _}
							<div class="flex-1 min-h-0 w-full"></div>
						{/each}
					</div>

					<!-- Static current time subtle guide line across grid -->
					<div
						class="absolute left-0 right-0 h-[1px] bg-white/20 pointer-events-none z-20"
						style="top: {currentDayTimePercent}%; transform: translateY(-50%);"
					></div>

					<!-- Sliding Days Track (39 columns animated in sync with header) -->
					<div
						class="h-full flex flex-row relative z-10"
						style="width: calc({TOTAL_COUNT} / {VISIBLE_COUNT} * 100%); transform: translate3d(-{transformPercent}%, 0, 0); {isAnimating
							? 'transition: transform 370ms cubic-bezier(0.16, 1, 0.3, 1);'
							: 'transition: none;'}"
					>
						{#each ALL_OFFSETS as offset, index (offset)}
							{@const colDate = (baseDate as CalendarDate).add({
								days: offset,
							})}
							{@const timedEvents = getDayTimedEvents(colDate)}
							{@const deadlines = getDayDeadlines(colDate)}
							{@const announcements = getDayAnnouncements(colDate)}
							{@const isColToday = isToday(colDate, getLocalTimeZone())}
							{@const visibleIdx = index - BUFFER - offsetDays}

							<div
								role="region"
								aria-label={`Calendar column for ${colDate.toString()}`}
								style="flex: 0 0 calc(100% / {TOTAL_COUNT});"
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
								{#if isColToday}
									<div
										class="absolute left-0 right-0 z-30 pointer-events-none flex items-center"
										style="top: {currentDayTimePercent}%; transform: translateY(-50%);"
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
									<div
										class="absolute pointer-events-auto group/gev select-none cursor-pointer overflow-visible z-10 hover:z-50"
										style="
											left: calc({evt.leftPercent}% + 1.5px);
											width: calc({evt.widthPercent}% - 3px);
											top: {evt.topPercent}%;
											height: {evt.heightPercent}%;
											min-height: 20px;
										"
									>
										<!-- Inner event pill -->
										<div
											class="w-full h-full rounded-md px-1.5 py-0.5 overflow-hidden transition-all duration-150 border text-[10px] font-medium leading-tight flex flex-col justify-start hover:ring-1 hover:ring-primary/40 shadow-xs relative"
											style="
												background-color: {evt.color || '#3b82f6'};
												border-color: {evt.color || '#3b82f6'};
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

												{#if evt.isTaskBlock}
													<button
														type="button"
														class="opacity-0 group-hover/gev:opacity-100 hover:bg-black/30 rounded size-3.5 flex items-center justify-center text-white/90 hover:text-white transition-all cursor-pointer shrink-0 z-20"
														title="Delete work session"
														onclick={(e) => {
															e.stopPropagation();
															deleteEvent(evt);
														}}
													>
														<X class="size-2.5" />
													</button>
												{/if}
											</div>

											<!-- Line 2: Time on new line if big enough (omitted if small) -->
											{#if evt.heightPercent >= 3.0 && evt.widthPercent >= 28}
												<span
													class="truncate text-[9px] text-white/85 font-medium leading-tight mt-0.5"
												>
													{evt.widthPercent > 60
														? evt.timeStr
														: evt.timeStr.split("-")[0].trim()}
												</span>
											{/if}
										</div>

										<!-- Hover Details Card (solid, zero transparency) -->
										<div
											class="absolute hidden group-hover/gev:flex flex-col gap-1.5 z-[100] p-2.5 rounded-lg bg-popover border border-border shadow-2xl text-popover-foreground text-xs min-w-[260px] max-w-[300px] animate-in fade-in zoom-in-95 duration-150 {evt.topPercent >
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
														class="text-[9px] px-1.5 py-0.5 rounded bg-primary/10 text-primary shrink-0 font-medium"
														>Work Session</span
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
											<p
												class="font-medium text-xs text-foreground leading-snug"
											>
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
															deleteEvent(evt);
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
										class="absolute pointer-events-auto group/announcement select-none flex items-center cursor-pointer z-10 hover:z-50 overflow-visible {ann.isEndOfDay
											? 'py-2 items-end'
											: 'py-2.5 items-center'}"
										style="
											left: calc({ann.leftPercent}% + 1.5px);
											width: calc({ann.widthPercent}% - 3px);
											{ann.isEndOfDay
											? 'bottom: 0px;'
											: `top: ${ann.topPercent}%; transform: translateY(-50%);`}
										"
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
											<p
												class="font-medium text-xs leading-snug text-foreground"
											>
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
										class="absolute pointer-events-auto group/deadline select-none flex cursor-pointer z-10 hover:z-50 overflow-visible {dl.isEndOfDay
											? 'py-2 items-end'
											: 'py-2.5 items-center'}"
										style="
											left: calc({dl.leftPercent}% + 1.5px);
											width: calc({dl.widthPercent}% - 3px);
											{dl.isEndOfDay
											? 'bottom: 0px;'
											: `top: ${dl.topPercent}%; transform: translateY(-50%);`}
										"
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
													{dl.status === "done"
														? "✓ Completed"
														: "Due Deadline"}
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
