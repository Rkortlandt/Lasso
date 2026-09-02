export interface EventRecord {
	id: string;
	calendar?: string;
	task?: string;
	title: string;
	start: string;
	end?: string;
	allday?: boolean;
	deadline?: boolean;
	announcement?: boolean;
	color?: string;
	description?: string;
	google_event_id?: string;
	expand?: {
		calendar?: CalendarRecord;
		task?: TaskRecord;
	};
}

export interface CalendarRecord {
	id: string;
	name: string;
	color?: string;
	source?: string;
	nickname?: string;
	course_id?: string;
	calendar_id?: string;
}

export interface TaskRecord {
	id: string;
	user?: string;
	calendar?: string | string[];
	name: string;
	status?: string;
	priority?: string;
	due_date?: string;
	fake_due_date?: string;
	expand?: {
		calendar?: CalendarRecord;
	};
}



export type CreateEventPayload = Omit<EventRecord, "id"> & { id?: string };
export type CreateTaskPayload = Omit<TaskRecord, "id"> & { id?: string };

import { pb } from "./pocketbase";

export async function addEventRecord(payload: CreateEventPayload) {
	return await pb.collection("events").create<EventRecord>(payload, { expand: "calendar,task" });
}

export async function addTaskRecord(payload: CreateTaskPayload) {
	return await pb.collection("tasks").create<TaskRecord>(payload, { expand: "calendar" });
}

export async function getCalendarRecords() {
	return await pb
		.collection("calendars")
		.getFullList<CalendarRecord>({
			sort: "name",
			requestKey: null,
		})
		.catch(() => [])
}

export async function getTaskRecords() {
	return await pb
		.collection("tasks")
		.getFullList<TaskRecord>({
			sort: "due_date",
			expand: "calendar",
			requestKey: null,
		})
		.catch(() => [])
}

export async function getEventRecords() {
	return await pb
		.collection("events")
		.getFullList<EventRecord>({
			expand: "calendar,task",
			requestKey: null,
		})
		.catch(() => [])
}
