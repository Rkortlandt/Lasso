import { rrulestr, RRuleSet } from "rrule";
import type { EventRecord } from "$lib/dataState/dataRecordInterfaces";

/**
 * Expands recurring events within a given date window into concrete virtual EventRecords.
 * Skips dates listed in master.exdate and original slot dates replaced by child exceptions.
 */
export function expandRecurringEvents(
	events: readonly EventRecord[] | EventRecord[],
	windowStart: Date,
	windowEnd: Date,
): EventRecord[] {
	const expanded: EventRecord[] = [];

	// 1. Group child exceptions by master event ID
	const exceptionsByMasterId = new Map<string, EventRecord[]>();
	const masterEvents: EventRecord[] = [];
	const regularAndExceptionEvents: EventRecord[] = [];

	for (const e of events) {
		if (e.recurr && e.recurr.trim() !== "") {
			masterEvents.push(e);
		} else {
			regularAndExceptionEvents.push(e);
			if (e.recurr_event_id) {
				let list = exceptionsByMasterId.get(e.recurr_event_id);
				if (!list) {
					list = [];
					exceptionsByMasterId.set(e.recurr_event_id, list);
				}
				list.push(e);
			}
		}
	}

	// 2. Add all regular and exception events directly
	for (const e of regularAndExceptionEvents) {
		expanded.push(e);
	}

	// 3. Expand each master recurring event
	for (const master of masterEvents) {
		if (!master.start) continue;

		const masterStartIso = master.start.includes(" ") ? master.start.replace(" ", "T") : master.start;
		const masterEndIso = master.end ? (master.end.includes(" ") ? master.end.replace(" ", "T") : master.end) : "";
		const masterStart = new Date(masterStartIso);
		if (isNaN(masterStart.getTime())) continue;

		const masterEnd = masterEndIso ? new Date(masterEndIso) : new Date(masterStart.getTime() + 60 * 60 * 1000);
		const durationMs = Math.max(0, masterEnd.getTime() - masterStart.getTime());

		try {
			const ruleSet = new RRuleSet();

			// Parse RRULE string
			let cleanRruleStr = master.recurr!.trim();
			if (!cleanRruleStr.toUpperCase().startsWith("RRULE:") && !cleanRruleStr.toUpperCase().startsWith("FREQ=")) {
				cleanRruleStr = `RRULE:${cleanRruleStr}`;
			}

			// Ensure DTSTART is supplied
			const rule = rrulestr(cleanRruleStr, {
				dtstart: masterStart,
			});
			ruleSet.rrule(rule);

			// Exclude cancelled dates from master's exdate
			let exdateList: string[] = [];
			if (Array.isArray(master.exdate)) {
				exdateList = master.exdate;
			} else if (typeof master.exdate === "string") {
				try {
					exdateList = JSON.parse(master.exdate);
				} catch {
					if (master.exdate.trim() !== "") {
						exdateList = [master.exdate];
					}
				}
			}

			for (const ex of exdateList) {
				const exIso = ex.includes(" ") ? ex.replace(" ", "T") : ex;
				const exDate = new Date(exIso);
				if (!isNaN(exDate.getTime())) {
					ruleSet.exdate(exDate);
				}
			}

			// Exclude original dates replaced by child exceptions
			const childExceptions = exceptionsByMasterId.get(master.id) || [];
			for (const child of childExceptions) {
				if (child.recurr_og_date) {
					const childOgIso = child.recurr_og_date.includes(" ")
						? child.recurr_og_date.replace(" ", "T")
						: child.recurr_og_date;
					const childOgDate = new Date(childOgIso);
					if (!isNaN(childOgDate.getTime())) {
						ruleSet.exdate(childOgDate);
					}
				}
			}

			// Generate occurrences within window
			const occurrences = ruleSet.between(windowStart, windowEnd, true);

			for (const occ of occurrences) {
				const occEnd = new Date(occ.getTime() + durationMs);
				const virtualId = `${master.id}_occ_${occ.getTime()}`;

				expanded.push({
					...master,
					id: virtualId,
					start: occ.toISOString(),
					end: occEnd.toISOString(),
					// Virtual occurrence is concrete
					recurr: undefined,
					recurr_event_id: master.id,
					recurr_og_date: occ.toISOString(),
				});
			}
		} catch (err) {
			console.warn(`[rruleExpansion] Failed to expand recurring event "${master.title}" (${master.id}):`, err);
			// Fallback: render the master event itself
			expanded.push(master);
		}
	}

	return expanded;
}
