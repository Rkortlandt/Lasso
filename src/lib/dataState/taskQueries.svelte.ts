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

function areTaskArraysEqual(a: TaskRecord[], b: TaskRecord[]): boolean {
	if (a.length !== b.length) return false;
	for (let i = 0; i < a.length; i++) {
		if (
			a[i].id !== b[i].id ||
			a[i].status !== b[i].status ||
			a[i].name !== b[i].name ||
			a[i].due_date !== b[i].due_date
		) {
			return false;
		}
	}
	return true;
}

const cachedCourseGroupsMap = new Map<string, TaskGroup>();
let lastCourseGroupsArray: TaskGroup[] = [];

/**
 * Groups tasks by coursework calendar with upcoming & completed subsets.
 * Pure reactive derivation with stable reference caching to prevent unnecessary DOM re-renders.
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
	let allGroupsUnchanged =
		lastCourseGroupsArray.length === calMap.size && calMap.size > 0;

	for (const cal of calMap.values()) {
		const calTasks = (taskMap.get(cal.id) || []).sort(sortTasksChronological);
		const prev = cachedCourseGroupsMap.get(cal.id);

		if (
			prev &&
			areTaskArraysEqual(calTasks, prev.tasks) &&
			cal.color === prev.calendar.color &&
			cal.nickname === prev.calendar.nickname &&
			cal.name === prev.calendar.name
		) {
			groups.push(prev);
		} else {
			allGroupsUnchanged = false;
			const upcomingTasks = calTasks.filter((t) => t.status !== "done");
			const completedTasks = calTasks
				.filter((t) => t.status === "done")
				.sort(sortTasksCompleted);
			const newGroup: TaskGroup = {
				calendar: cal,
				tasks: calTasks,
				upcomingTasks,
				completedTasks,
				pendingCount: upcomingTasks.length,
			};
			cachedCourseGroupsMap.set(cal.id, newGroup);
			groups.push(newGroup);
		}
	}

	if (allGroupsUnchanged) {
		return lastCourseGroupsArray;
	}

	lastCourseGroupsArray = groups;
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

let lastTodoTasks: TaskRecord[] = [];
let lastTodoCalId: string | undefined = undefined;
let lastTodoReturnObj: {
	calendar?: CalendarRecord;
	tasks: TaskRecord[];
	upcomingTasks: TaskRecord[];
	completedTasks: TaskRecord[];
	pendingCount: number;
} | null = null;

/**
 * Derives tasks that belong to the user's personal To Do calendar.
 * Caches return reference if tasks belonging to To Do haven't changed.
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
		.filter((t) => t.source === "todo" || parseTaskCalendarId(t) === todoCal.id)
		.sort(sortTasksChronological);

	if (
		lastTodoReturnObj &&
		lastTodoCalId === todoCal.id &&
		areTaskArraysEqual(tasks, lastTodoTasks)
	) {
		return lastTodoReturnObj;
	}

	const upcomingTasks = tasks.filter((t) => t.status !== "done");
	const completedTasks = tasks.filter((t) => t.status === "done").sort(sortTasksCompleted);

	lastTodoTasks = tasks;
	lastTodoCalId = todoCal.id;
	lastTodoReturnObj = {
		calendar: todoCal,
		tasks,
		upcomingTasks,
		completedTasks,
		pendingCount: upcomingTasks.length,
	};

	return lastTodoReturnObj;
}
