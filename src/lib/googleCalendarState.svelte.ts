import { pb, POCKETBASE_URL } from './pocketbase';

export interface GoogleCalendar {
	id: string;
	summary: string;
	description?: string;
	primary?: boolean;
	backgroundColor?: string;
	foregroundColor?: string;
	colorId?: string;
	accessRole?: 'owner' | 'writer' | 'reader' | 'freeBusyReader';
	timeZone?: string;
	selected?: boolean;
	nickname?: string;
	original_name?: string;
	category?: 'primary' | 'owned' | 'subscribed';
}

export interface ReadOnlyEvent {
	id: string;
	calendarId: string;
	title: string;
	description?: string;
	start: string;
	end?: string;
	allday: boolean;
	color?: string;
	eventLabelId?: string;
	labelName?: string;
	readOnly: boolean;
}

class GoogleCalendarState {
	isConnected = $state(false);
	isChecking = $state(true);
	isLoading = $state(false);
	isSyncingToGoogle = $state(false);
	lastSyncedToGoogle = $state<Date | null>(null);
	syncMessage = $state<string | null>(null);
	isSyncingFromGoogle = $state(false);
	lastSyncedFromGoogle = $state<Date | null>(null);
	syncFromGoogleMessage = $state<string | null>(null);
	userEmail = $state('');
	calendars = $state<GoogleCalendar[]>([]);
	lassoCalendarId = $state<string | null>(null);
	readOnlyEvents = $state<ReadOnlyEvent[]>([]);
	isLoadingReadOnlyEvents = $state(false);
	private isFetchingEvents = false;
	error = $state<string | null>(null);
	needsReauth = $state(false);
	colors = $state<{
		event?: Record<string, string>;
		calendar?: Record<string, string>;
	}>({});

	constructor() {
		// Watch for user login / record changes via PocketBase authStore directly
		pb.authStore.onChange((token, record) => {
			if (record) {
				this.syncFromRecord(record);
			} else {
				this.reset();
			}
		});

		// Check server on initial load if already authenticated
		if (typeof window !== 'undefined' && pb.authStore.isValid) {
			const authRec = pb.authStore.record || (pb.authStore as any).model;
			if (authRec) {
				this.syncFromRecord(authRec);
			}
			this.loadFromPocketBase();
			this.checkServerStatus();
		} else {
			this.isChecking = false;
		}
	}

	syncFromRecord(record: any) {
		const isConn = Boolean(record.google_connected);
		if (isConn) {
			this.isConnected = true;
			this.userEmail = record.google_email || record.email || '';
			this.isChecking = false;
			this.loadFromPocketBase().then(() => {
				if (this.calendars.length === 0 && !this.isSyncingFromGoogle) {
					this.syncFromGoogle().catch(() => {});
				}
			});
		} else {
			this.checkServerStatus();
		}
	}

	get validCalendars(): GoogleCalendar[] {
		return this.calendars;
	}

	get lassoCalendar(): GoogleCalendar | undefined {
		return this.calendars.find(
			(c) => c.summary.toLowerCase() === 'lasso' || (this.lassoCalendarId && c.id === this.lassoCalendarId)
		);
	}

	// All user personal calendars (strictly read-only from Lasso's perspective)
	get readOnlyCalendars(): GoogleCalendar[] {
		return this.calendars.filter(
			(c) => c.summary.toLowerCase() !== 'lasso' && (!this.lassoCalendarId || c.id !== this.lassoCalendarId)
		);
	}

	get primaryCalendar(): GoogleCalendar | undefined {
		return this.calendars.find((c) => c.primary) || this.calendars[0];
	}

	get myCalendars(): GoogleCalendar[] {
		return this.readOnlyCalendars.filter(
			(c) => c.primary || c.accessRole === 'owner' || c.accessRole === 'writer' || c.category === 'owned'
		);
	}

	get subscribedCalendars(): GoogleCalendar[] {
		return this.readOnlyCalendars.filter(
			(c) => !c.primary && c.accessRole !== 'owner' && c.accessRole !== 'writer' && c.category !== 'owned'
		);
	}

	async checkServerStatus() {
		if (!pb.authStore.isValid || !pb.authStore.token) {
			this.isChecking = false;
			return;
		}

		try {
			const response = await fetch(`${POCKETBASE_URL}/api/google/calendars`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					Authorization: `Bearer ${pb.authStore.token}`
				},
				body: JSON.stringify({})
			});

			if (response.ok) {
				const data = await response.json();
				if (data.success && data.calendars) {
					this.isConnected = true;
					this.userEmail = data.email || pb.authStore.record?.email || '';
					this.calendars = data.calendars;
					if (data.colors) {
						this.colors = data.colors;
					}
					const lassoCal = data.calendars.find((c: any) => c.summary?.toLowerCase() === 'lasso');
					if (lassoCal) {
						this.lassoCalendarId = lassoCal.id;
					}
					this.needsReauth = false;
				} else if (data.needsReauth) {
					this.needsReauth = true;
					this.isConnected = false;
				}
			}
		} catch (e) {
			console.warn('Google Calendar status check failed:', e);
		} finally {
			this.isChecking = false;
		}
	}

	async initFromAuth(authData: any) {
		const accessToken = authData?.meta?.accessToken;
		const refreshToken = authData?.meta?.refreshToken;
		if (!accessToken) return;
		this.isConnected = true;
		this.userEmail = authData?.meta?.email || pb.authStore.record?.email || '';
		try {
			const res = await fetch(`${POCKETBASE_URL}/api/google/calendars`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					Authorization: pb.authStore.token ? `Bearer ${pb.authStore.token}` : ''
				},
				body: JSON.stringify({ accessToken, refreshToken })
			});
			const data = await res.json();
			if (data.success && data.calendars) {
				this.calendars = data.calendars;
				this.userEmail = data.email || this.userEmail;
				if (data.colors) {
					this.colors = data.colors;
				}
				const lassoCal = data.calendars.find((c: any) => c.summary?.toLowerCase() === 'lasso');
				if (lassoCal) {
					this.lassoCalendarId = lassoCal.id;
				}
				this.needsReauth = false;
			}
		} catch (e) {
			console.warn('Failed to load calendars from login OAuth:', e);
		}
	}

	async connectWithGoogle() {
		this.isLoading = true;
		this.error = null;

		try {
			const authData = await pb.collection('users').authWithOAuth2({
				provider: 'google',
				scopes: [
					'https://www.googleapis.com/auth/userinfo.profile',
					'https://www.googleapis.com/auth/userinfo.email',
					'https://www.googleapis.com/auth/calendar',
					'https://www.googleapis.com/auth/calendar.events'
				],
				urlCallback: (url) => {
					const authUrl = new URL(url);
					authUrl.searchParams.set('access_type', 'offline');
					window.open(authUrl.toString(), 'popup_window', 'width=1024,height=768');
				}
			});

			const accessToken = authData?.meta?.accessToken;
			const refreshToken = authData?.meta?.refreshToken;

			const response = await fetch(`${POCKETBASE_URL}/api/google/calendars`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					Authorization: pb.authStore.token ? `Bearer ${pb.authStore.token}` : ''
				},
				body: JSON.stringify({ accessToken, refreshToken })
			});

			const data = await response.json();
			if (data.success && data.calendars) {
				this.isConnected = true;
				this.userEmail = data.email || pb.authStore.record?.email || '';
				this.calendars = data.calendars;
				if (data.colors) {
					this.colors = data.colors;
				}
				const lassoCal = data.calendars.find((c: any) => c.summary?.toLowerCase() === 'lasso');
				if (lassoCal) {
					this.lassoCalendarId = lassoCal.id;
				}
				this.needsReauth = false;
				return data;
			} else {
				throw new Error(data.message || 'Failed to fetch Google calendars');
			}
		} catch (err: any) {
			console.error('Google Calendar connection error:', err);
			this.error = err?.message || 'Could not connect to Google Calendar';
			throw err;
		} finally {
			this.isLoading = false;
		}
	}

	async refresh() {
		this.isLoading = true;
		try {
			if (pb.authStore.token) {
				const response = await fetch(`${POCKETBASE_URL}/api/google/calendars`, {
					method: 'POST',
					headers: {
						'Content-Type': 'application/json',
						Authorization: `Bearer ${pb.authStore.token}`
					},
					body: JSON.stringify({})
				});
				const data = await response.json();
				if (data.success && data.calendars) {
					this.isConnected = true;
					this.calendars = data.calendars;
					this.userEmail = data.email || this.userEmail;
					if (data.colors) {
						this.colors = data.colors;
					}
					const lassoCal = data.calendars.find((c: any) => c.summary?.toLowerCase() === 'lasso');
					if (lassoCal) {
						this.lassoCalendarId = lassoCal.id;
					}
					this.needsReauth = false;
				} else if (data.needsReauth) {
					this.needsReauth = true;
				}
			}
		} catch (e) {
			console.error('Calendar refresh error:', e);
		} finally {
			this.isLoading = false;
		}
	}

	/**
	 * Ensures the dedicated "Lasso" calendar exists in the user's Google Calendar.
	 */
	async ensureLassoCalendar(): Promise<string> {
		if (!pb.authStore.token) throw new Error('Not authenticated');

		const resp = await fetch(`${POCKETBASE_URL}/api/google/lasso/ensure`, {
			method: 'POST',
			headers: {
				'Content-Type': 'application/json',
				Authorization: `Bearer ${pb.authStore.token}`
			}
		});

		const data = await resp.json();
		if (!resp.ok || !data.success) {
			throw new Error(data.message || 'Failed to ensure Lasso Google calendar');
		}

		this.lassoCalendarId = data.calendarId;
		await this.refresh();
		return data.calendarId;
	}

	/**
	 * Deletes the dedicated "Lasso" secondary calendar from Google Calendar (purging all events)
	 * and recreates a clean, empty one.
	 */
	async purgeLassoCalendar(): Promise<{ calendarId: string; message: string }> {
		if (!pb.authStore.token) throw new Error('Not authenticated');

		this.isSyncingToGoogle = true;
		this.syncMessage = null;
		this.error = null;

		try {
			const resp = await fetch(`${POCKETBASE_URL}/api/google/lasso/purge`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					Authorization: `Bearer ${pb.authStore.token}`
				}
			});

			const data = await resp.json();
			if (!resp.ok || !data.success) {
				throw new Error(data.message || 'Failed to purge Lasso Google calendar');
			}

			this.lassoCalendarId = data.calendarId;
			this.syncMessage = data.message;
			await this.refresh();
			return { calendarId: data.calendarId, message: data.message };
		} catch (err: any) {
			console.error('Purge Lasso calendar failed:', err);
			this.error = err.message || 'Failed to purge Lasso Google calendar';
			throw err;
		} finally {
			this.isSyncingToGoogle = false;
		}
	}

	/**
	 * Syncs all coursework tasks and deadlines to the dedicated "Lasso" Google Calendar,
	 * tagging each event by course with title prefix [Course], description, and color.
	 */
	async syncToGoogle(): Promise<{ syncedCount: number; message: string }> {
		if (!pb.authStore.token) throw new Error('Not authenticated');

		this.isSyncingToGoogle = true;
		this.syncMessage = null;
		this.error = null;

		try {
			const resp = await fetch(`${POCKETBASE_URL}/api/google/sync`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					Authorization: `Bearer ${pb.authStore.token}`
				}
			});

			const data = await resp.json();
			if (!resp.ok || !data.success) {
				throw new Error(data.message || 'Failed to sync coursework to Google Calendar');
			}

			this.lastSyncedToGoogle = new Date();
			this.syncMessage = data.message;
			if (data.calendarId) {
				this.lassoCalendarId = data.calendarId;
			}
			await this.refresh();
			return { syncedCount: data.syncedCount, message: data.message };
		} catch (err: any) {
			console.error('Sync to Google Calendar failed:', err);
			this.error = err.message || 'Failed to sync to Google Calendar';
			throw err;
		} finally {
			this.isSyncingToGoogle = false;
		}
	}

	/**
	 * Loads cached Google calendars and events directly from PocketBase (<5ms).
	 */
	async loadFromPocketBase() {
		if (!pb.authStore.isValid) return;
		try {
			// Query stored Google calendars from PocketBase
			const pbCals = await pb.collection('calendars').getFullList({
				filter: "source = 'google'",
				sort: 'name',
				requestKey: null
			});

			if (pbCals.length > 0) {
				this.calendars = pbCals.map((r: any) => ({
					id: r.calendar_id || r.id,
					summary: r.nickname || r.name,
					description: '',
					primary: false,
					accessRole: 'owner',
					color: r.color || '#4285f4',
					backgroundColor: r.color || '#4285f4',
					foregroundColor: '#ffffff',
					category: 'owned',
					nickname: r.nickname,
					original_name: r.name,
					pbId: r.id
				}));

				const lassoCal = this.calendars.find((c) => c.summary.toLowerCase() === 'lasso');
				if (lassoCal) {
					this.lassoCalendarId = lassoCal.id;
				}
			}

			// Query stored Google events from PocketBase
			const pbEvts = await pb.collection('events').getFullList({
				filter: "google_event_id != ''",
				sort: 'start',
				expand: 'calendar',
				requestKey: null
			});

			if (pbEvts.length > 0) {
				this.readOnlyEvents = pbEvts.map((e: any) => {
					const calObj = e.expand?.calendar;
					const calId = calObj?.calendar_id || e.calendar;
					return {
						id: e.google_event_id || e.id,
						calendarId: calId,
						pbCalendarId: e.calendar,
						title: e.title,
						description: e.description || '',
						start: e.start,
						end: e.end,
						allday: Boolean(e.allday),
						color: e.color || calObj?.color || '#4285f4',
						eventLabelId: e.event_label_id,
						labelName: e.label_name,
						readOnly: true
					};
				});
			}

			// In background, refresh live events with Google labels
			const ids = this.readOnlyCalendars.map((c) => c.id);
			if (ids.length > 0) {
				this.fetchReadOnlyEvents(ids).catch(() => {});
			}
		} catch (e) {
			console.warn('Failed to load Google calendars/events from PocketBase:', e);
			const ids = this.readOnlyCalendars.map((c) => c.id);
			if (ids.length > 0) {
				await this.fetchReadOnlyEvents(ids);
			}
		}
	}

	/**
	 * Synchronizes external information from Google Calendar into PocketBase in the background:
	 * Fetches Google calendars and expanded events, persists to PocketBase, and updates state.
	 */
	async syncFromGoogle(): Promise<{ calendarCount: number; eventCount: number; message: string }> {
		if (!pb.authStore.token) throw new Error('Not authenticated');
		this.isSyncingFromGoogle = true;
		this.syncFromGoogleMessage = null;
		this.error = null;

		try {
			const resp = await fetch(`${POCKETBASE_URL}/api/google/sync-inbound`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					Authorization: `Bearer ${pb.authStore.token}`
				}
			});

			const data = await resp.json();
			if (!resp.ok || !data.success) {
				throw new Error(data.message || 'Failed to sync Google calendars and events');
			}

			// Reload local cached records from PocketBase
			await this.loadFromPocketBase();

			this.lastSyncedFromGoogle = new Date();
			const msg = data.message || `Synced ${data.calendarsSynced ?? 0} calendars and ${data.eventsSynced ?? 0} events from Google.`;
			this.syncFromGoogleMessage = msg;
			return {
				calendarCount: data.calendarsSynced ?? 0,
				eventCount: data.eventsSynced ?? 0,
				message: msg
			};
		} catch (err: any) {
			console.error('Sync from Google Calendar failed:', err);
			this.error = err.message || 'Failed to sync from Google Calendar';
			throw err;
		} finally {
			this.isSyncingFromGoogle = false;
		}
	}

	/**
	 * Retrieves events from personal Google calendars (strictly read-only)
	 * for viewing on Lasso's calendar grid.
	 */
	async fetchReadOnlyEvents(calendarIds: string[], timeMin?: string, timeMax?: string): Promise<ReadOnlyEvent[]> {
		if (!pb.authStore.token || calendarIds.length === 0) {
			this.readOnlyEvents = [];
			return [];
		}

		if (this.isFetchingEvents) {
			return this.readOnlyEvents;
		}

		this.isFetchingEvents = true;
		this.isLoadingReadOnlyEvents = true;
		try {
			const resp = await fetch(`${POCKETBASE_URL}/api/google/read-events`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					Authorization: `Bearer ${pb.authStore.token}`
				},
				body: JSON.stringify({ calendarIds, timeMin, timeMax })
			});

			const data = await resp.json();
			if (resp.ok && data.success && Array.isArray(data.events)) {
				this.readOnlyEvents = data.events;
				return data.events;
			}
			return [];
		} catch (err) {
			console.warn('Failed to fetch read-only Google events:', err);
			return [];
		} finally {
			this.isLoadingReadOnlyEvents = false;
			this.isFetchingEvents = false;
		}
	}

	async updateNickname(calendarId: string, nickname: string) {
		const trimmed = nickname.trim();
		try {
			if (pb.authStore.token) {
				await fetch(`${POCKETBASE_URL}/api/calendar/nickname`, {
					method: 'POST',
					headers: {
						'Content-Type': 'application/json',
						Authorization: `Bearer ${pb.authStore.token}`
					},
					body: JSON.stringify({ calendarId, nickname: trimmed })
				}).catch((e) => console.warn('Could not persist calendar nickname:', e));

				// Update corresponding calendar object in PocketBase directly for immediate realtime sync
				try {
					const matchingCal = await pb.collection('calendars').getFirstListItem(
						`calendar_id = "${calendarId}" || id = "${calendarId}"`
					).catch(() => null);
					if (matchingCal) {
						await pb.collection('calendars').update(matchingCal.id, {
							nickname: trimmed
						}).catch(() => {});
					}
				} catch {}
			}

			// Update in state locally with instant reactivity
			this.calendars = this.calendars.map((c) => {
				if (String(c.id) === String(calendarId)) {
					const orig = c.original_name || c.summary;
					return {
						...c,
						nickname: trimmed || undefined,
						summary: trimmed || orig,
						original_name: orig
					};
				}
				return c;
			});
			return true;
		} catch (err: any) {
			console.error('Failed to update calendar nickname:', err);
			throw err;
		}
	}

	disconnect() {
		this.isConnected = false;
		this.calendars = [];
		this.colors = {};
		this.lassoCalendarId = null;
		this.readOnlyEvents = [];
		this.lastSyncedToGoogle = null;
		this.syncMessage = null;
		this.isSyncingFromGoogle = false;
		this.lastSyncedFromGoogle = null;
		this.syncFromGoogleMessage = null;
		this.needsReauth = false;

		if (pb.authStore.token) {
			fetch(`${POCKETBASE_URL}/api/google/disconnect`, {
				method: 'POST',
				headers: {
					Authorization: `Bearer ${pb.authStore.token}`
				}
			}).catch(() => {});
		}
	}

	reset() {
		this.isConnected = false;
		this.userEmail = '';
		this.calendars = [];
		this.colors = {};
		this.lassoCalendarId = null;
		this.readOnlyEvents = [];
		this.lastSyncedToGoogle = null;
		this.syncMessage = null;
		this.isSyncingFromGoogle = false;
		this.lastSyncedFromGoogle = null;
		this.syncFromGoogleMessage = null;
		this.error = null;
		this.needsReauth = false;
	}
}

export const googleCalendarState = new GoogleCalendarState();
