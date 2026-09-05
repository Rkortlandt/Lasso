import { dataState } from "$lib/dataState/dataState.svelte";
import { type CalendarRecord } from "$lib/dataState/dataRecordInterfaces";

import { pb } from "$lib/pocketbase";

/**
 * Derives a fast lookup Map of calendar ID -> CalendarRecord.
 * Automatically reactive when dataState.calendars changes.
 */
export function getCalendarMap(): Map<string, CalendarRecord> {
	const map = new Map<string, CalendarRecord>();
	for (const cal of dataState.calendars) {
		map.set(cal.id, cal);
	}
	return map;
}

/**
 * Derives calendars that belong to coursework / Canvas courses.
 * Filters out personal Google calendars and the user's personal To Do calendar.
 */
export function getCourseworkCalendars(): CalendarRecord[] {
	return dataState.calendars.filter(
		(c) =>
			c.source !== "google" &&
			!c.calendar_id &&
			c.source !== "todo" &&
			c.source !== "internal" &&
			(c.name || "").toLowerCase().trim() !== "to do",
	);
}

/**
 * Finds the user's personal To Do calendar.
 */
export function getTodoCalendar(): CalendarRecord | undefined {
	return dataState.calendars.find(
		(c) =>
			c.source === "todo" ||
			c.source === "internal" ||
			(c.name || "").toLowerCase().trim() === "to do",
	);
}

let isEnsuringTodoCalendar = false;

/**
 * Ensures a To Do calendar exists for the user. Creates one optimistically if not present.
 */
export async function ensureTodoCalendar(): Promise<CalendarRecord | null> {
	const existing = getTodoCalendar();
	if (existing) return existing;

	const userId = pb.authStore.record?.id;
	if (!userId || isEnsuringTodoCalendar) return existing || null;

	isEnsuringTodoCalendar = true;
	try {
		const created = await dataState.addCalendar({
			user: userId,
			name: "To Do",
			source: "todo",
			color: "#10b981",
			visible: true,
		});
		return created;
	} catch (err) {
		console.error("Failed to create todo calendar:", err);
		return null;
	} finally {
		isEnsuringTodoCalendar = false;
	}
}

/**
 * Derives personal Google Calendars stored in PocketBase.
 * Excludes the dedicated outbound "Lasso" coursework calendar.
 */
export function getGoogleCalendars(): CalendarRecord[] {
	return dataState.calendars.filter(
		(c) =>
			(c.source === "google" || Boolean(c.calendar_id)) &&
			(c.nickname || c.name || "").toLowerCase().trim() !== "lasso",
	);
}

/**
 * Finds the dedicated outbound "Lasso" Google Calendar if it exists.
 */
export function getDedicatedLassoCalendar(): CalendarRecord | undefined {
	return dataState.calendars.find(
		(c) =>
			(c.source === "google" || Boolean(c.calendar_id)) &&
			(c.nickname || c.name || "").toLowerCase().trim() === "lasso",
	);
}

/**
 * Resolves the display color of a calendar with fallbacks.
 */
export function resolveCalendarColor(cal?: CalendarRecord | null, fallback = "#3b82f6"): string {
	if (!cal) return fallback;
	return cal.color || fallback;
}
