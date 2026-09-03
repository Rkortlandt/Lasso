import { dataState } from "$lib/dataState/dataState.svelte";
import { type CalendarRecord } from "$lib/dataState/dataRecordInterfaces";

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
 * Filters out personal Google calendars.
 */
export function getCourseworkCalendars(): CalendarRecord[] {
	return dataState.calendars.filter(
		(c) => c.source !== "google" && !c.calendar_id,
	);
}

/**
 * Derives personal Google Calendars stored in PocketBase.
 */
export function getGoogleCalendars(): CalendarRecord[] {
	return dataState.calendars.filter(
		(c) => c.source === "google" || Boolean(c.calendar_id),
	);
}

/**
 * Resolves the display color of a calendar with fallbacks.
 */
export function resolveCalendarColor(cal?: CalendarRecord | null, fallback = "#3b82f6"): string {
	if (!cal) return fallback;
	return cal.color || fallback;
}
