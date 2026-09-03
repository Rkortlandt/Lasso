import { dataState } from "$lib/dataState/dataState.svelte";
import { type EventRecord } from "$lib/dataState/dataRecordInterfaces";

/**
 * Returns a fast lookup Set of announcement titles (lowercase trimmed)
 * to quickly filter out duplicate task/announcement entries.
 */
export function getAnnouncementTitlesSet(): Set<string> {
	const set = new Set<string>();
	for (const e of dataState.events) {
		if (e.announcement && e.title) {
			set.add(e.title.toLowerCase().trim());
		}
	}
	return set;
}

/**
 * Returns all deadline events.
 */
export function getDeadlineEvents(): EventRecord[] {
	return dataState.events.filter((e) => e.deadline);
}

/**
 * Returns discrete time-allocation grid events (non-announcements).
 */
export function getGridEvents(): EventRecord[] {
	return dataState.events.filter((e) => !e.announcement);
}

/**
 * Returns all events associated with a specific calendar ID.
 */
export function getEventsByCalendarId(calendarId: string): EventRecord[] {
	return dataState.events.filter((e) => e.calendar === calendarId);
}
