# System Architecture & Database Specification: Calendar & Task Engine

This document outlines the architecture, entity relationships, database schema, and operational rules for the unified calendar and task management engine.

---

## 1. System Overview

The system unifies time management across three distinct layers:
1. Calendars Layer (calendars): Logical grouping, color customization, visibility toggles, and multi-source ownership (Internal, Google Calendar via OAuth2, Canvas LMS via iCal).
2. Time-Grid Events Layer (events): Discrete time allocations on the calendar grid, recurring events (RFC 5545 RRULE), exception dates, and task work blocks.
3. Actionable Tasks Layer (tasks): Work items, priorities, completion lifecycles, and deadlines (both hard deadlines and early target/fake deadlines).

---

## 2. Entity Relationship Diagram

+-------------------------------------------------------------+
|                            users                            |
+------------------------------+------------------------------+
                               | 1:N                          | 1:N
                               v                              v
                     +-------------------+          +-------------------+
                     |     calendars     |          |       tasks       |
                     +---------+---------+          +---------+---------+
                               | 1:N                          | 1:N (Time blocks)
                               v                              |
                     +----------------------------------+     |
                     |              events              |<----+
                     |  (FK: calendar_id, FK: task_id)  |
                     +----------------------------------+

---

## 3. Database Schema
### `users` (Auth)
- `id` (PK)
- `email` (unique), `emailVisibility` (bool), `verified` (bool)
- `name`, `avatar`, `password`, `tokenKey`
- **Canvas Auth:** `canvas_url`, `canvas_token`, `canvas_connected` (bool), `canvas_student_name`
- **Google Auth:** `google_email`, `google_access_token`, `google_refresh_token`, `google_token_expiry`, `google_connected` (bool)
- `created`, `updated`

### `calendars`
- `id` (PK)
- `user` (FK -> users)
- `name`, `color`, `source` (text)
- `nickname` (text, user display nickname stored directly on calendar object)
- `course_id` (text, optional Canvas course ID)
- `calendar_id` (text, optional Google Calendar ID)
- `visible` (bool)
- `created`, `updated`

### `tasks`
- `id` (PK)
- `user` (FK -> users)
- `name` (text)
- `status` (text), `priority` (text)
- `due_date`, `fake_due_date` (text ISO)
- `created`, `updated`

### `events`
- `id` (PK)
- `calendar` (FK -> calendars)
- `task` (FK -> tasks, optional)
- `title`, `start`, `end` (text ISO)
- `allday` (bool), `deadline` (bool), `announcement` (bool)
- `recurr` (RRULE text), `exdate` (json), `timezone` (text)
- `created`, `updated`

### `sync_status` (Singleton 1:1 per user)
- `id` (PK)
- `user` (FK -> users, unique index)
- **Canvas Sync:** `canvas_status` ("idle" | "running" | "success" | "error"), `canvas_synced_at` (text ISO), `canvas_error` (text), `canvas_feedback` (text, e.g. "Synced 4 courses, 18 assignments, 3 announcements")
- **Google Import (Inbound personal events):** `google_import_status` ("idle" | "running" | "success" | "error"), `google_import_synced_at` (text ISO), `google_import_error` (text), `google_import_feedback` (text, e.g. "Imported 42 events across 3 calendars")
- **Google Export (Outbound coursework push):** `google_export_status` ("idle" | "running" | "success" | "error"), `google_export_synced_at` (text ISO), `google_export_error` (text), `google_export_feedback` (text, e.g. "Exported 24 upcoming deadlines to Lasso calendar")
- **Sync History Log:** `history` (json array of `SyncHistoryEntry`, capped at last 20-50 operations for chronological audit & activity log)
- `created`, `updated`

---

## 4. State Management Architecture: Single Source of Truth (`dataState`)

To eliminate scattered PocketBase fetches, duplicate WebSocket/SSE connections, and memory desynchronization across components:

1. **`dataState` is the Sole Source of Truth**:
   - Holds the master in-memory collections: `calendars`, `tasks`, `events`, and `syncStatus`.
   - Components MUST NOT perform raw `pb.collection(...).getFullList()` or subscribe directly to PocketBase collections.
   - All state is strictly exposed via **read-only reactive getters** (`dataState.calendars`, `dataState.tasks`, etc.).
2. **Built-in Optimistic Mutations**:
   - `dataState.addEvent()`, `updateTask()`, `deleteCalendar()`, etc. apply changes to local memory synchronously (0ms latency), then persist asynchronously.
   - Uses a **Trailing Edge Worker** queue for updates to serialize rapid user clicks and avoid race conditions.
   - Uses **creationFingerprints** to prevent double-submit bounces.
   - Implements **tombstones** and **deferred patches** so rapid edits or deletions on freshly created `temp_` records succeed seamlessly.
3. **Single Sync Subscription (`sync_status`)**:
   - The frontend does NOT subscribe to individual `events/*`, `tasks/*`, or `calendars/*` SSE channels.
   - It listens strictly to `sync_status/*`. When a background sync completes (`status === "success"`), `dataState.refresh()` pulls a clean, atomic snapshot.

---

## 5. Query Modules: Derived State Pattern

To keep `dataState` thin (<200 lines) and avoid monolithic "god objects", specialized domain filtering and formatting MUST NOT be added as methods on `dataState`.

Instead, create dedicated, modular **Query Files** that derive reactive state from `dataState`:

```
                  ┌──────────────────────┐
                  │      dataState       │  <-- Single Source of Truth
                  │  (Only raw records & │      (calendars, tasks, events, sync)
                  │   optimistic CRUD)   │
                  └──────────┬───────────┘
                             │
        ┌────────────────────┼────────────────────┐
        ▼                    ▼                    ▼
┌──────────────────┐ ┌──────────────────┐ ┌──────────────────┐
│ calendarQueries  │ │   taskQueries    │ │   gridQueries    │  <-- Modular Query
│ .svelte.ts       │ │   .svelte.ts     │ │   .svelte.ts     │      Files (Pure/Derived)
│ - googleCalendars│ │ - tasksByCourse  │ │ - collisionCols  │
│ - canvasCourses  │ │ - upcomingTasks  │ │ - allDayBuckets  │
└──────────────────┘ └──────────────────┘ └──────────────────┘
```

### Operational Rules for Query Modules:
1. **Pure & Tree-Shakable**: Query modules export `$derived` values or functions that read from `dataState`.
2. **Reactivity Preservation**: Because Svelte 5 tracks signals through function calls, any function or `$derived` reading `dataState` getters automatically updates the UI when records change.
3. **Separation of Concerns**:
   - Place calendar grouping/filtering in `src/lib/dataState/calendarQueries.svelte.ts`.
   - Place task grouping/status filtering in `src/lib/dataState/taskQueries.svelte.ts`.
   - Place time grid math/clustering in `src/lib/components/calendar/gridQueries.svelte.ts`.
4. **No Direct Writes**: Query modules are strictly read-only views; all mutations must flow through `dataState.add*`, `update*`, or `delete*`.
