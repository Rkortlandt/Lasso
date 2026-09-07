import type { CalendarDate, DateValue } from "@internationalized/date";
import type { EventRecord, TaskRecord } from "$lib/pocketbaseActions";

export interface DayAllDayEvent {
	id: string;
	title: string;
	color?: string;
}

export interface FormattedDeadline {
	id: string;
	task?: TaskRecord;
	event?: EventRecord;
	name: string;
	dueTimeStr: string;
	status: string;
	priority?: string;
	color: string;
	courseName: string;
	isEndOfDay: boolean;
	topPercent: number;
	leftPercent: number;
	widthPercent: number;
}

export interface FormattedAnnouncement {
	id: string;
	event: EventRecord;
	title: string;
	timeStr: string;
	color: string;
	courseName: string;
	description?: string;
	isEndOfDay: boolean;
	topPercent: number;
	leftPercent: number;
	widthPercent: number;
}

export interface FormattedTimedEvent {
	id: string;
	rawId?: string;
	title: string;
	timeStr: string;
	calendarName: string;
	color: string;
	calendarColor?: string;
	topPercent: number;
	heightPercent: number;
	startMin: number;
	endMin: number;
	colIndex: number;
	leftPercent: number;
	widthPercent: number;
	description?: string;
	isTaskBlock?: boolean;
	isTaskDone?: boolean;
	taskId?: string;
	source: "google" | "canvas" | "internal";
	rawEvent?: EventRecord;
}

export interface DayItem {
	date: CalendarDate;
	weekday: string;
	dayNumber: number;
	monthName: string;
	isSelected: boolean;
	isToday: boolean;
}
