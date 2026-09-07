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
 * Returns the sorting key for a calendar: nickname if present, otherwise canonical name.
 */
export function getCalendarSortKey(c: CalendarRecord): string {
	return (c.nickname?.trim() || c.name || "").trim().toLowerCase();
}

/**
 * Sorts calendars by nickname if present, otherwise by name (case-insensitive natural sort).
 */
export function sortCalendarsByNameOrNickname(a: CalendarRecord, b: CalendarRecord): number {
	const keyA = getCalendarSortKey(a);
	const keyB = getCalendarSortKey(b);
	return keyA.localeCompare(keyB, undefined, { numeric: true, sensitivity: "base" });
}

/**
 * Checks whether a course has ended/concluded based on its end_date.
 */
export function isCourseEnded(course: CalendarRecord): boolean {
	if (!course.end_date) return false;
	const raw = course.end_date.trim();
	if (!raw) return false;
	const normalized = raw.includes("T") ? raw : raw.replace(" ", "T");
	const end = new Date(normalized);
	if (isNaN(end.getTime())) return false;
	return end.getTime() < Date.now();
}

/**
 * Derives calendars that belong to coursework / Canvas courses.
 * Filters out personal Google calendars and the user's personal To Do calendar.
 * Sorted by nickname if present, otherwise by name.
 */
export function getCourseworkCalendars(): CalendarRecord[] {
	return dataState.calendars
		.filter(
			(c) =>
				c.source !== "google" &&
				!c.calendar_id &&
				c.source !== "todo" &&
				c.source !== "internal" &&
				(c.name || "").toLowerCase().trim() !== "to do",
		)
		.sort(sortCalendarsByNameOrNickname);
}

/**
 * Derives current (active) coursework calendars whose end_date has not passed.
 */
export function getCurrentCourseworkCalendars(): CalendarRecord[] {
	return getCourseworkCalendars().filter((c) => !isCourseEnded(c));
}

/**
 * Derives previous (concluded) coursework calendars whose end_date has passed.
 */
export function getPreviousCourseworkCalendars(): CalendarRecord[] {
	return getCourseworkCalendars().filter((c) => isCourseEnded(c));
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
