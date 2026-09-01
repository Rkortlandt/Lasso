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
- `allday` (bool), `deadline` (bool)
- `recurr` (RRULE text), `exdate` (json), `timezone` (text)
- `created`, `updated`
