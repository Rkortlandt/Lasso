import { pb, POCKETBASE_URL } from './pocketbase';
import { authState } from './authState.svelte';

export interface CanvasCourse {
	id: number;
	name: string;
	course_code?: string;
	section_name?: string;
	nickname?: string;
	original_name?: string;
	workflow_state?: string;
	start_at?: string | null;
	end_at?: string | null;
	concluded?: boolean;
	access_restricted_by_date?: boolean;
	status?: 'previous' | 'current' | 'upcoming';
	term?: {
		id?: number;
		name?: string;
		start_at?: string | null;
		end_at?: string | null;
	};
	enrollments?: Array<{
		course_section_id?: number;
		enrollment_state?: string;
	}>;
	sections?: Array<{
		id: number;
		name: string;
		start_at?: string | null;
		end_at?: string | null;
	}>;
	color?: string;
	backgroundColor?: string;
}

export function isValidCourse(course: CanvasCourse | any): boolean {
	if (!course) return false;
	if (course.access_restricted_by_date) return false;
	if (!course.name || typeof course.name !== 'string' || course.name.trim().length === 0) return false;
	const state = (course.workflow_state || '').toLowerCase();
	if (state === 'deleted') return false;
	return true;
}

export function getCourseEndDate(course: CanvasCourse | any): Date | null {
	if (course.end_at) {
		const d = new Date(course.end_at);
		if (!isNaN(d.getTime())) return d;
	}
	if (course.term && course.term.end_at) {
		const d = new Date(course.term.end_at);
		if (!isNaN(d.getTime())) return d;
	}
	if (Array.isArray(course.sections)) {
		for (const s of course.sections) {
			if (s.end_at) {
				const d = new Date(s.end_at);
				if (!isNaN(d.getTime())) return d;
			}
		}
	}
	return null;
}

export function classifyCourse(course: CanvasCourse): 'previous' | 'current' | 'upcoming' {
	const now = Date.now();
	const state = (course.workflow_state || '').toLowerCase();

	// 1. Previous is based on end date (if available)
	const endDate = getCourseEndDate(course);
	if (endDate && endDate.getTime() < now) {
		return 'previous';
	}
	if (course.concluded === true || state === 'completed') {
		return 'previous';
	}
	if (Array.isArray(course.enrollments) && course.enrollments.some((e) => e.enrollment_state === 'completed')) {
		return 'previous';
	}

	// 2. Upcoming (ones not published)
	if (state === 'unpublished') {
		return 'upcoming';
	}

	// 3. Otherwise current
	return 'current';
}

export interface CanvasTask {
	id: string;
	user: string;
	calendar?: string;
	name: string;
	status: 'todo' | 'done' | string;
	priority: 'low' | 'med' | 'high' | string;
	due_date?: string;
	fake_due_date?: string;
	created?: string;
	updated?: string;
}

export interface CanvasCalendarRecord {
	id: string;
	user: string;
	name: string;
	color?: string;
	source?: string;
	visible?: boolean;
	nickname?: string;
	course_id?: string;
	calendar_id?: string;
	created?: string;
	updated?: string;
}

class CanvasState {
	isConnected = $state(false);
	isChecking = $state(true);
	canvasUrl = $state('https://canvas.instructure.com');
	studentName = $state('');
	courses = $state<CanvasCourse[]>([]);
	tasks = $state<CanvasTask[]>([]);
	calendars = $state<CanvasCalendarRecord[]>([]);
	isLoading = $state(false);
	isSyncing = $state(false);
	lastSynced = $state<Date | null>(null);
	syncSuccessMessage = $state<string | null>(null);
	error = $state<string | null>(null);
	skipped = $state(false);

	constructor() {
		// Clean up any legacy localStorage keys from earlier dev builds
		if (typeof window !== 'undefined') {
			localStorage.removeItem('lasso_canvas_connected');
			localStorage.removeItem('lasso_canvas_url');
			localStorage.removeItem('lasso_canvas_student');
			localStorage.removeItem('lasso_canvas_courses');
		}

		// Watch for user login / record changes
		$effect.root(() => {
			if (authState.record) {
				this.syncFromRecord(authState.record);
			} else {
				this.reset();
			}
		});

		// Check server on initial load if already authenticated and not yet connected
		if (typeof window !== 'undefined' && pb.authStore.isValid) {
			if (!this.isConnected) {
				this.checkServerStatus();
			}
		} else {
			this.isChecking = false;
		}
	}

	async checkServerStatus() {
		if (!pb.authStore.isValid || !pb.authStore.token || this.isConnected || this.isLoading) {
			this.isChecking = false;
			return;
		}

		try {
			const response = await fetch(`${POCKETBASE_URL}/api/canvas/verify`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					Authorization: `Bearer ${pb.authStore.token}`
				},
				body: JSON.stringify({ canvasToken: 'refresh' })
			});

			if (response.ok) {
				const data = await response.json();
				if (data.success && data.courses) {
					this.isConnected = true;
					this.canvasUrl = data.canvasUrl || this.canvasUrl;
					this.studentName = data.studentName || this.studentName;
					this.courses = (data.courses || []).filter(isValidCourse);
				}
			}
		} catch (e) {
			console.warn('Canvas status check failed:', e);
		} finally {
			this.isChecking = false;
		}
	}

	syncFromRecord(record: any) {
		const isConn = Boolean(record.canvas_connected);
		if (isConn) {
			this.isConnected = true;
			this.canvasUrl = record.canvas_url || 'https://canvas.instructure.com';
			this.studentName = record.canvas_student_name || '';
			this.isChecking = false;
			// If courses are not yet loaded in memory, fetch live from server
			if (this.courses.length === 0 && !this.isLoading) {
				this.refresh();
			}
		} else {
			// If local record in authStore doesn't show connection, check server directly
			this.checkServerStatus();
		}
	}

	get needsSetup(): boolean {
		return !this.isChecking && authState.isAuthenticated && !this.isConnected && !this.skipped;
	}

	get validCourses(): CanvasCourse[] {
		return this.courses.filter(isValidCourse);
	}

	get currentCourses(): CanvasCourse[] {
		return this.validCourses.filter((c) => classifyCourse(c) === 'current');
	}

	get upcomingCourses(): CanvasCourse[] {
		return this.validCourses.filter((c) => classifyCourse(c) === 'upcoming');
	}

	get previousCourses(): CanvasCourse[] {
		return this.validCourses.filter((c) => classifyCourse(c) === 'previous');
	}

	async connect(canvasUrl: string, canvasToken: string) {
		this.isLoading = true;
		this.error = null;

		try {
			const response = await fetch(`${POCKETBASE_URL}/api/canvas/verify`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					Authorization: pb.authStore.token ? `Bearer ${pb.authStore.token}` : ''
				},
				body: JSON.stringify({ canvasUrl, canvasToken })
			});

			const data = await response.json();

			if (!response.ok || !data.success) {
				throw new Error(data.message || data.error || 'Failed to verify Canvas token');
			}

			this.isConnected = true;
			this.canvasUrl = data.canvasUrl || canvasUrl;
			this.studentName = data.studentName || 'Student';
			this.courses = (data.courses || []).filter(isValidCourse);
			this.skipped = false;

			return data;
		} catch (err: any) {
			console.error('Canvas connection error:', err);
			this.error = err?.message || 'Could not connect to Canvas';
			throw err;
		} finally {
			this.isLoading = false;
		}
	}

	async fetchTasks(): Promise<CanvasTask[]> {
		if (!pb.authStore.isValid) return [];
		try {
			const records = await pb.collection('tasks').getFullList<CanvasTask>({
				sort: 'due_date',
				requestKey: null
			});
			this.tasks = records;
			return records;
		} catch (err: any) {
			console.warn('Failed to fetch tasks from PocketBase:', err);
			return [];
		}
	}

	async fetchCalendars(): Promise<CanvasCalendarRecord[]> {
		if (!pb.authStore.isValid) return [];
		try {
			const records = await pb.collection('calendars').getFullList<CanvasCalendarRecord>({
				requestKey: null
			});
			this.calendars = records;
			return records;
		} catch (err: any) {
			console.warn('Failed to fetch calendars from PocketBase:', err);
			return [];
		}
	}

	async syncCanvas(): Promise<{ success: boolean; coursesSynced: number; tasksSynced: number; message: string }> {
		if (!pb.authStore.token) {
			const err = 'Authentication required to sync Canvas';
			this.error = err;
			throw new Error(err);
		}

		this.isSyncing = true;
		this.isLoading = true;
		this.error = null;
		this.syncSuccessMessage = null;

		try {
			const response = await fetch(`${POCKETBASE_URL}/api/sync/canvas`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					Authorization: `Bearer ${pb.authStore.token}`
				},
				body: JSON.stringify({ canvasUrl: this.canvasUrl })
			});

			const data = await response.json();

			if (!response.ok || !data.success) {
				const errMsg = data.message || data.error || `Canvas sync failed (HTTP ${response.status})`;
				this.error = errMsg;
				throw new Error(errMsg);
			}

			this.lastSynced = new Date();
			this.syncSuccessMessage = data.message || `Successfully synced ${data.coursesSynced} courses and ${data.tasksSynced} tasks`;

			// Refresh courses, calendars, and tasks to immediately reflect updates in UI
			await Promise.all([
				this.refresh(),
				this.fetchTasks(),
				this.fetchCalendars()
			]);

			return data;
		} catch (err: any) {
			console.error('Canvas sync error:', err);
			this.error = err?.message || 'Failed to sync Canvas';
			throw err;
		} finally {
			this.isSyncing = false;
			this.isLoading = false;
		}
	}

	async sync() {
		return this.syncCanvas();
	}

	async refresh() {
		if (this.isLoading) return;
		this.isLoading = true;
		try {
			if (pb.authStore.token) {
				const response = await fetch(`${POCKETBASE_URL}/api/canvas/verify`, {
					method: 'POST',
					headers: {
						'Content-Type': 'application/json',
						Authorization: `Bearer ${pb.authStore.token}`
					},
					body: JSON.stringify({ canvasUrl: this.canvasUrl, canvasToken: 'refresh' })
				});
				const data = await response.json();
				if (data.success && data.courses) {
					this.courses = (data.courses || []).filter(isValidCourse);
				}
				await Promise.all([this.fetchTasks(), this.fetchCalendars()]);
			}
		} catch (e) {
			console.error('Refresh error:', e);
		} finally {
			this.isLoading = false;
		}
	}

	async updateNickname(courseId: number | string, nickname: string) {
		const trimmed = nickname.trim();
		try {
			if (pb.authStore.token) {
				await fetch(`${POCKETBASE_URL}/api/calendar/nickname`, {
					method: 'POST',
					headers: {
						'Content-Type': 'application/json',
						Authorization: `Bearer ${pb.authStore.token}`
					},
					body: JSON.stringify({ courseId, nickname: trimmed })
				}).catch((e) => console.warn('Could not persist nickname to backend:', e));

				// Update corresponding calendar object in PocketBase directly for immediate realtime sync
				try {
					const origCourse = this.courses.find((c) => String(c.id) === String(courseId));
					let filter = `course_id = "${courseId}" || id = "${courseId}"`;
					if (origCourse) {
						const searchName = (origCourse.original_name || origCourse.name).replace(/"/g, '\\"');
						filter += ` || name = "${searchName}"`;
					}
					const matchingCal = await pb.collection('calendars').getFirstListItem(filter).catch(() => null);
					if (matchingCal) {
						await pb.collection('calendars').update(matchingCal.id, {
							course_id: String(courseId),
							nickname: trimmed
						}).catch(() => {});
					}
				} catch {}
			}

			// Update in state locally with instant reactivity
			this.courses = this.courses.map((c) => {
				if (String(c.id) === String(courseId)) {
					const orig = c.original_name || c.name;
					return {
						...c,
						nickname: trimmed || undefined,
						name: trimmed || orig,
						original_name: orig
					};
				}
				return c;
			});
			return true;
		} catch (err: any) {
			console.error('Failed to update nickname:', err);
			throw err;
		}
	}

	skip() {
		this.skipped = true;
	}

	disconnect() {
		this.isConnected = false;
		this.studentName = '';
		this.courses = [];
		this.skipped = false;

		if (pb.authStore.token) {
			fetch(`${POCKETBASE_URL}/api/canvas/disconnect`, {
				method: 'POST',
				headers: {
					Authorization: `Bearer ${pb.authStore.token}`
				}
			}).catch(() => {});
		}
	}

	reset() {
		this.isConnected = false;
		this.studentName = '';
		this.courses = [];
		this.tasks = [];
		this.calendars = [];
		this.isSyncing = false;
		this.lastSynced = null;
		this.syncSuccessMessage = null;
		this.error = null;
		this.skipped = false;
	}
}

export const canvasState = new CanvasState();
