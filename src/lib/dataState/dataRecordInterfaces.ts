export interface EventRecord {
	id: string;
	_clientId?: string;
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
	recurr?: string;
	recurr_event_id?: string;
	recurr_og_date?: string;
	exdate?: string[] | string;
	timezone?: string;
	source?: "canvas" | "google" | "internal" | string;
	google_event_id?: string;
	event_label_id?: string;
	label_name?: string;
	expand?: {
		calendar?: CalendarRecord;
		task?: TaskRecord;
	};
}

export interface LabelRecord {
	id: string;
	user?: string;
	name: string;
	color: string;
	created?: string;
	updated?: string;
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
	google_label_id?: string;
	start_date?: string;
	end_date?: string;
	label?: string;
	created?: string;
	updated?: string;
	expand?: {
		label?: LabelRecord;
	};
}

export interface TaskRecord {
	id: string;
	_clientId?: string;
	user?: string;
	calendar?: string | string[] | Record<string, any>;
	calendar_id?: string | string[];
	name: string;
	source: "todo" | "canvas" | "google";
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

export type SyncOperationStatus = "idle" | "running" | "success" | "error";

export interface SyncHistoryEntry {
	id: string;
	service: "canvas" | "google_import" | "google_export";
	status: "success" | "error";
	timestamp: string;
	duration_ms?: number;
	feedback?: string;
	count?: number;
	error?: string;
}

export interface SyncStatusRecord {
	id: string;
	user: string;

	// Canvas Sync
	canvas_status: SyncOperationStatus;
	canvas_synced_at?: string;
	canvas_error?: string;
	canvas_feedback?: string;

	// Google Import (Inbound personal events)
	google_import_status: SyncOperationStatus;
	google_import_synced_at?: string;
	google_import_error?: string;
	google_import_feedback?: string;

	// Google Export (Outbound coursework push)
	google_export_status: SyncOperationStatus;
	google_export_synced_at?: string;
	google_export_error?: string;
	google_export_feedback?: string;
	google_export_enabled: boolean;

	// Chronological sync history log (capped to last 20-50 entries)
	history?: SyncHistoryEntry[];

	created?: string;
	updated?: string;
}

