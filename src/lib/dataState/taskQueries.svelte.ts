import { dataState } from "$lib/dataState/dataState.svelte";
import { type TaskRecord, type CalendarRecord } from "$lib/dataState/dataRecordInterfaces";
import { getCourseworkCalendars, getTodoCalendar, resolveCalendarColor } from "./calendarQueries.svelte";

export interface TaskGroup {
	calendar: CalendarRecord;
	tasks: TaskRecord[];
	upcomingTasks: TaskRecord[];
	completedTasks: TaskRecord[];
	pendingCount: number;
}

/**
 * Extracts a normalized calendar ID string from a TaskRecord.
 */
export function parseTaskCalendarId(task: TaskRecord): string | null {
	let val: any = task.calendar || task.calendar_id;
	if (Array.isArray(val) && val.length > 0) {
		val = val[0];
	}
	if (typeof val === "object" && val !== null) {
		val = val.id;
	}
	return typeof val === "string" && val.trim() !== "" ? val.trim() : null;
}

/**
 * Sorts tasks ascending by due date (chronological), falling back to creation time.
 */
export function sortTasksChronological(a: TaskRecord, b: TaskRecord): number {
	if (!a.due_date && !b.due_date) {
		if (a.created && b.created) {
			return new Date(a.created).getTime() - new Date(b.created).getTime();
		}
		return 0;
	}
	if (!a.due_date) return 1;
	if (!b.due_date) return -1;
	return new Date(a.due_date).getTime() - new Date(b.due_date).getTime();
}

/**
 * Sorts completed tasks descending by due date (most recently finished/due first).
 */
export function sortTasksCompleted(a: TaskRecord, b: TaskRecord): number {
	if (!a.due_date && !b.due_date) return 0;
	if (!a.due_date) return 1;
	if (!b.due_date) return -1;
	return new Date(b.due_date).getTime() - new Date(a.due_date).getTime();
}

/**
 * Groups tasks by coursework calendar with upcoming & completed subsets.
 * Pure reactive derivation from dataState.tasks and dataState.calendars.
 */
export function getTasksByCourse(): TaskGroup[] {
	const courseworkCals = getCourseworkCalendars();
	const calMap = new Map<string, CalendarRecord>();

	for (const cal of courseworkCals) {
		calMap.set(cal.id, { ...cal, color: resolveCalendarColor(cal) });
	}

	// Also harvest any calendars expanded directly on tasks
	for (const task of dataState.tasks) {
		if (task.expand?.calendar && !calMap.has(task.expand.calendar.id)) {
			if (
				task.expand.calendar.source === "google" ||
				task.expand.calendar.calendar_id
			) {
				continue;
			}
			calMap.set(task.expand.calendar.id, {
				...task.expand.calendar,
				color: resolveCalendarColor(task.expand.calendar),
			});
		}
	}

	const taskMap = new Map<string, TaskRecord[]>();
	for (const task of dataState.tasks) {
		const calId = parseTaskCalendarId(task);
		if (calId && calMap.has(calId)) {
			const list = taskMap.get(calId) || [];
			list.push(task);
			taskMap.set(calId, list);
		}
	}

	const groups: TaskGroup[] = [];
	for (const cal of calMap.values()) {
		const calTasks = (taskMap.get(cal.id) || []).sort(sortTasksChronological);
		const upcomingTasks = calTasks.filter((t) => t.status !== "done");
		const completedTasks = calTasks.filter((t) => t.status === "done").sort(sortTasksCompleted);
		const pendingCount = upcomingTasks.length;

		groups.push({
			calendar: cal,
			tasks: calTasks,
			upcomingTasks,
			completedTasks,
			pendingCount,
		});
	}

	return groups;
}

/**
 * Returns all upcoming (uncompleted) tasks across all calendars.
 */
export function getUpcomingTasks(): TaskRecord[] {
	return dataState.tasks
		.filter((t) => t.status !== "done")
		.sort(sortTasksChronological);
}

/**
 * Returns all completed tasks across all calendars.
 */
export function getCompletedTasks(): TaskRecord[] {
	return dataState.tasks
		.filter((t) => t.status === "done")
		.sort(sortTasksCompleted);
}

/**
 * Derives tasks that belong to the user's personal To Do calendar.
 */
export function getTodoTasks(): {
	calendar?: CalendarRecord;
	tasks: TaskRecord[];
	upcomingTasks: TaskRecord[];
	completedTasks: TaskRecord[];
	pendingCount: number;
} {
	const todoCal = getTodoCalendar();
	if (!todoCal) {
		return {
			tasks: [],
			upcomingTasks: [],
			completedTasks: [],
			pendingCount: 0,
		};
	}

	const tasks = dataState.tasks
		.filter((t) => parseTaskCalendarId(t) === todoCal.id)
		.sort(sortTasksChronological);

	const upcomingTasks = tasks.filter((t) => t.status !== "done");
	const completedTasks = tasks.filter((t) => t.status === "done").sort(sortTasksCompleted);

	return {
		calendar: todoCal,
		tasks,
		upcomingTasks,
		completedTasks,
		pendingCount: upcomingTasks.length,
	};
}
