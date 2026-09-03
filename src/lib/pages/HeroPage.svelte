<script lang="ts">
	import TopDayBar from "$lib/components/calendar/TopDayBar.svelte";
	import AllDayEventsBar from "$lib/components/calendar/AllDayEventsBar.svelte";
	import CalendarGrid from "$lib/components/calendar/CalendarGrid.svelte";
	import type {
		DayAllDayEvent,
		FormattedDeadline,
		FormattedAnnouncement,
		FormattedTimedEvent,
		DayItem,
	} from "$lib/components/calendar/calendarTypes";
	import { dayState } from "$lib/dayState.svelte";
	import { canvasState } from "$lib/canvasState.svelte";
	import { pb } from "$lib/pocketbase";
	import { authState } from "$lib/authState.svelte";
	import { calendarVisibilityState } from "$lib/calendarVisibilityState.svelte";
	import { googleCalendarState } from "$lib/googleCalendarState.svelte";
	import { themeState } from "$lib/themeState.svelte";
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

	let pbTasks = $state<TaskRecord[]>([]);
	let pbCalendars = $state<CalendarRecord[]>([]);
	let pbEvents = $state<EventRecord[]>([]);
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

	async function onDeleteEvent(evt: FormattedTimedEvent) {
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

	// Watch dayState.value changes
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
		// Only react to transform transitions on the track itself
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

<div class="h-full w-full flex flex-col overflow-hidden">
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
	/>
</div>
