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
	visible?: boolean;
	user?: string;
	nickname?: string;
	course_id?: string;
	calendar_id?: string;
	created?: string;
	updated?: string;
}

export interface TaskRecord {
	id: string;
	user?: string;
	calendar?: string | string[] | Record<string, any>;
	calendar_id?: string | string[];
	name: string;
	status?: "todo" | "done" | string;
	priority?: "low" | "med" | "high" | string;
	grade?: string;
	source_link?: string;
	due_date?: string;
	fake_due_date?: string;
	created?: string;
	updated?: string;
	expand?: {
		calendar?: CalendarRecord;
	};
}

export type CreateEventPayload = Omit<EventRecord, "id"> & { id?: string };
export type CreateTaskPayload = Omit<TaskRecord, "id"> & { id?: string };

import { pb } from "./pocketbase";

export interface RecordFetchOptions {
	sort?: string;
	filter?: string;
	expand?: string;
	fields?: string;
	requestKey?: string | null;
}

export async function addEventRecord(payload: CreateEventPayload) {
	return await pb.collection("events").create<EventRecord>(payload, { expand: "calendar,task" });
}

export async function addTaskRecord(payload: CreateTaskPayload) {
	return await pb.collection("tasks").create<TaskRecord>(payload, { expand: "calendar" });
}

export async function updateTaskRecord(id: string, payload: Partial<TaskRecord>) {
	return await pb.collection("tasks").update<TaskRecord>(id, payload);
}

export async function deleteTaskRecord(id: string) {
	return await pb.collection("tasks").delete(id);
}

export async function deleteEventRecord(id: string) {
	return await pb.collection("events").delete(id);
}

export async function getCalendarRecords(options?: RecordFetchOptions): Promise<CalendarRecord[]> {
	return await pb
		.collection("calendars")
		.getFullList<CalendarRecord>({
			sort: "name",
			requestKey: null,
			...options,
		})
		.catch(() => []);
}

export async function getTaskRecords(options?: RecordFetchOptions): Promise<TaskRecord[]> {
	return await pb
		.collection("tasks")
		.getFullList<TaskRecord>({
			sort: "due_date",
			expand: "calendar",
			requestKey: null,
			...options,
		})
		.catch(() => []);
}

export async function getEventRecords(options?: RecordFetchOptions): Promise<EventRecord[]> {
	return await pb
		.collection("events")
		.getFullList<EventRecord>({
			expand: "calendar,task",
			requestKey: null,
			...options,
		})
		.catch(() => []);
}
