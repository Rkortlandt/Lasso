import { pb } from "$lib/pocketbase";
import { type CalendarRecord, type TaskRecord, type EventRecord, type SyncStatusRecord } from "$lib/dataState/dataRecordInterfaces";

interface workerState {
	inFlight: boolean;
	pendingPatch: Record<string, any> | null;
	baselineRecord: any;
}

interface creationState {
	isCancelled: boolean;
	pendingPatch: Record<string, any> | null;
}

// Fast 32-bit FNV-1a hash function for strings
function hashString(input: string): string {
	let hash = 2166136261;
	for (let i = 0; i < input.length; i++) {
		hash ^= input.charCodeAt(i);
		hash = Math.imul(hash, 16777619);
	}
	return (hash >>> 0).toString(36);
}

// Normalizes a record for fingerprinting by stripping PocketBase auto-generated system fields
function computeRecordFingerprint(collectionName: string, record: any): string {
	const {
		id,
		created,
		updated,
		collectionId,
		collectionName: _colName,
		expand,
		...domainFields
	} = record;
	return `${collectionName}_${hashString(JSON.stringify(domainFields))}`;
}

class DataState {
	private _calendars = $state<CalendarRecord[]>([]);
	private _tasks = $state<TaskRecord[]>([]);
	private _events = $state<EventRecord[]>([]);
	private _syncStatus = $state<SyncStatusRecord | null>(null);
	private _loading = $state<boolean>(false);
	private _refreshing = $state<boolean>(false);
	private _subscriptionRefs: (() => void)[] = [];

	get calendars(): readonly CalendarRecord[] {
		return this._calendars;
	}
	get tasks(): readonly TaskRecord[] {
		return this._tasks;
	}
	get events(): readonly EventRecord[] {
		return this._events;
	}
	get syncStatus(): SyncStatusRecord | null {
		return this._syncStatus;
	}
	get loading(): boolean {
		return this._loading;
	}

	// Canvas Sync Getters
	get isCanvasSyncing(): boolean {
		return this._syncStatus?.canvas_status === "running";
	}
	get canvasSyncedAt(): string | undefined {
		return this._syncStatus?.canvas_synced_at;
	}
	get canvasSyncError(): string | undefined {
		return this._syncStatus?.canvas_error;
	}
	get canvasSyncFeedback(): string | undefined {
		return this._syncStatus?.canvas_feedback;
	}

	// Google Import (Inbound) Getters
	get isGoogleImporting(): boolean {
		return this._syncStatus?.google_import_status === "running";
	}
	get googleImportSyncedAt(): string | undefined {
		return this._syncStatus?.google_import_synced_at;
	}
	get googleImportError(): string | undefined {
		return this._syncStatus?.google_import_error;
	}
	get googleImportFeedback(): string | undefined {
		return this._syncStatus?.google_import_feedback;
	}

	// Google Export (Outbound) Getters
	get isGoogleExporting(): boolean {
		return this._syncStatus?.google_export_status === "running";
	}
	get googleExportSyncedAt(): string | undefined {
		return this._syncStatus?.google_export_synced_at;
	}
	get googleExportError(): string | undefined {
		return this._syncStatus?.google_export_error;
	}
	get googleExportFeedback(): string | undefined {
		return this._syncStatus?.google_export_feedback;
	}
	get isGoogleExportEnabled(): boolean {
		return this._syncStatus?.google_export_enabled ?? true;
	}
	set isGoogleExportEnabled(val: boolean) {
		this.setGoogleExportEnabled(val);
	}
	get googleExportEnabled(): boolean {
		return this._syncStatus?.google_export_enabled ?? true;
	}
	set googleExportEnabled(val: boolean) {
		this.setGoogleExportEnabled(val);
	}

	// Sync History Log
	get syncHistory(): readonly import("./dataRecordInterfaces").SyncHistoryEntry[] {
		return this._syncStatus?.history ?? [];
	}

	constructor() {
		if (pb.authStore.isValid) {
			this.init();
		}

		pb.authStore.onChange((_, record) => {
			if (pb.authStore.isValid && record) {
				this.init();
			} else {
				this.reset();
			}
		});

		window.addEventListener("beforeunload", () => this.unsubscribe());
		window.addEventListener("pagehide", () => this.unsubscribe());

		document.addEventListener("visibilitychange", () => {
			if (document.visibilityState === "visible" && pb.authStore.isValid) {
				this.refresh();
			}
		});
	}

	/**
	 * Refreshes data from PocketBase without tearing down subscriptions.
	 * Preserves any active `temp_` records and in-flight worker modifications.
	 */
	async refresh(): Promise<void> {
		if (!pb.authStore.isValid) return;
		if (this._refreshing == true) return;

		this._refreshing = true;

		try {
			const [cals, tsks, evts, syncRecord] = await Promise.all([
				pb.collection("calendars").getFullList<CalendarRecord>({ sort: "name" }),
				pb.collection("tasks").getFullList<TaskRecord>({ sort: "due_date" }),
				pb.collection("events").getFullList<EventRecord>({ sort: "start" }),
				pb
					.collection("sync_status")
					.getFirstListItem<SyncStatusRecord>(`user = "${pb.authStore.record?.id}"`)
					.catch(() => null),
			]);

			// Helper to merge incoming server records with active local state
			const mergeWithInFlight = <T extends { id: string }>(
				serverList: T[],
				localList: T[],
				collectionName: string,
			): T[] => {
				const activeTemps = localList.filter((item) => item.id.startsWith("temp_"));
				const tempHashes = new Set(
					activeTemps.map((tempItem) => computeRecordFingerprint(collectionName, tempItem)),
				);

				const mergedServer = serverList
					.filter((serverItem) => {
						if (this.pendingDeletes.has(serverItem.id)) {
							return false; // hide items currently being deleted, even though server hasn't caught up
						}
						const serverHash = computeRecordFingerprint(collectionName, serverItem);
						if (tempHashes.has(serverHash)) {
							return false;
						}
						return true;
					})
					.map((serverItem) => {
						if (this.updateWorkers.has(serverItem.id)) {
							const localItem = localList.find((item) => item.id === serverItem.id);
							return localItem ?? serverItem;
						}
						return serverItem;
					});

				return [...mergedServer, ...activeTemps];
			};


			this._calendars = mergeWithInFlight(cals, this._calendars, "calendars");
			this._tasks = mergeWithInFlight(tsks, this._tasks, "tasks");
			this._events = mergeWithInFlight(evts, this._events, "events");
			this._syncStatus = syncRecord;
		} catch (err) {
			console.error("Failed to refresh data:", err);
		} finally {
			this._refreshing = false;
		}
	}

	// Load initial arrays and establish sync subscription
	private async init() {
		if (!pb.authStore.isValid) return;

		if (this._loading) return;
		this._loading = true;

		try {
			// Initial parallel load (only user's records return from PB)
			const [cals, tsks, evts, syncRecord] = await Promise.all([
				pb.collection("calendars").getFullList<CalendarRecord>({ sort: "name" }),
				pb.collection("tasks").getFullList<TaskRecord>({ sort: "due_date" }),
				pb.collection("events").getFullList<EventRecord>({ sort: "start" }),
				pb
					.collection("sync_status")
					.getFirstListItem<SyncStatusRecord>(`user = "${pb.authStore.record?.id}"`)
					.catch(() => null),
			]);

			this._calendars = cals;
			this._tasks = tsks;
			this._events = evts;
			this._syncStatus = syncRecord;

			await this.subscribeToSync();
		} catch (err) {
			console.error("Failed to load initial data:", err);
		} finally {
			this._loading = false;
		}
	}

	private isSubscribing = false;

	/**
	 * Subscribes ONLY to the sync table (`sync_status`).
	 * Tracks sync timestamps and statuses for:
	 * - Canvas Sync (import)
	 * - Google Import (inbound personal events)
	 * - Google Export (outbound coursework push)
	 * 
	 * When any backend sync starts or finishes, we update our reactive status
	 * and re-fetch clean data when an import finishes.
	 */
	private async subscribeToSync() {
		this.unsubscribe();
		this.isSubscribing = true;

		try {
			// Subscribes to the single sync_status collection
			const syncSubRef = await pb.collection("sync_status").subscribe("*", (event) => {
				if (event.action === "create" || event.action === "update") {
					const record = event.record as unknown as SyncStatusRecord;
					const prevCanvasSync = this._syncStatus?.canvas_synced_at;
					const prevGoogleImportSync = this._syncStatus?.google_import_synced_at;
					const prevGoogleExportSync = this._syncStatus?.google_export_synced_at;

					this._syncStatus = record;

					// If a sync operation just completed, refresh data cleanly
					if (
						record.canvas_status === "success" ||
						record.google_import_status === "success" ||
						record.google_export_status === "success" ||
						(record.canvas_synced_at && record.canvas_synced_at !== prevCanvasSync) ||
						(record.google_import_synced_at && record.google_import_synced_at !== prevGoogleImportSync) ||
						(record.google_export_synced_at && record.google_export_synced_at !== prevGoogleExportSync)
					) {
						this.refresh();
					}
				} else if (event.action === "delete") {
					this._syncStatus = null;
				}
			});

			// If unsubscribe() or reset() ran while we were awaiting the network:
			if (!this.isSubscribing) {
				syncSubRef(); // Kill the subscription immediately!
				return;
			}

			this._subscriptionRefs.push(syncSubRef);
		} catch {
			// Graceful fallback if sync_status table isn't created in schema yet
		} finally {
			this.isSubscribing = false;
		}
	}

	private unsubscribe() {
		this.isSubscribing = false;
		for (const refUnSubscribe of this._subscriptionRefs) {
			try {
				refUnSubscribe();
			} catch { }
		}
		this._subscriptionRefs = [];
	}

	private reset() {
		this.unsubscribe();
		this._events = [];
		this._calendars = [];
		this._tasks = [];
		this._syncStatus = null;
	}

	// Generic Optimistic Mutation Helpers
	private updateWorkers = new Map<string, workerState>();
	private pendingCreations = new Map<string, creationState>();
	private creationFingerprints = new Map<string, string>();
	private pendingDeletes = new Set<string>();

	private async optimisticAdd<T extends { id: string }>(
		getList: () => T[],
		setList: (items: T[]) => void,
		collectionName: string,
		recordData: Omit<T, "id">,
	): Promise<T> {
		// Fingerprint buffer using normalized domain fields
		const fingerprint = computeRecordFingerprint(collectionName, recordData);
		const existingId = this.creationFingerprints.get(fingerprint)
		if (existingId) {
			var fingerprintRecord = getList().find((item) => item.id == existingId);
			if (fingerprintRecord != undefined) return fingerprintRecord;
		}

		// 2. Instant local UI update with temp ID
		const tempId = `temp_${Date.now()}_${Math.random().toString(36).slice(2, 7)}`;
		this.creationFingerprints.set(fingerprint, tempId);
		const optimisticRecord = { ...recordData, id: tempId } as T;

		this.pendingCreations.set(tempId, {
			isCancelled: false,
			pendingPatch: null,
		});

		// Always append to the freshest current array
		setList([...getList(), optimisticRecord]);

		try {
			const createdRecord = await pb.collection(collectionName).create<T>(recordData);
			const pendingState = this.pendingCreations.get(tempId);
			this.pendingCreations.delete(tempId);

			// If the user already clicked delete while creation was in flight:
			if (pendingState?.isCancelled) {
				// Delete immediately on the server so it never persists or resurrects
				await pb.collection(collectionName).delete(createdRecord.id).catch(() => { });
				return createdRecord;
			}

			// ALWAYS fetch the freshest array reference via closure after an await
			const currentList = [...getList()];
			const itemIndex = currentList.findIndex((item) => item.id === tempId);
			if (itemIndex !== -1) {
				currentList[itemIndex] = createdRecord;
				setList(currentList);
			}

			this.creationFingerprints.set(fingerprint, createdRecord.id);

			// If the user made edits while creation was in flight:
			if (pendingState?.pendingPatch) {
				// Apply the queued patch using the official server ID!
				await this.optimisticUpdate(
					getList,
					setList,
					collectionName,
					createdRecord.id,
					pendingState.pendingPatch as Partial<T>,
				);
			}

			return createdRecord;
		} catch (err) {
			this.pendingCreations.delete(tempId);
			setList(getList().filter((item) => item.id !== tempId));
			console.error(`Failed to create ${collectionName} record, rolled back:`, err);
			throw err;
		} finally {
			// Free fingerprint after 400ms to allow future intentional duplicates
			setTimeout(() => {
				this.creationFingerprints.delete(fingerprint);
			}, 400);
		}
	}

	private async optimisticUpdate<T extends { id: string }>(
		getList: () => T[],
		setList: (items: T[]) => void,
		collectionName: string,
		id: string,
		patch: Partial<T>,
	): Promise<T> {
		let currentList = [...getList()];
		const itemIndex = currentList.findIndex((item) => item.id === id);
		if (itemIndex === -1) throw new Error(`${collectionName} record with id ${id} not found`);

		// 1. Instant local UI update (0ms latency)
		const currentRecord = currentList[itemIndex];
		currentList[itemIndex] = { ...currentRecord, ...patch };
		setList(currentList);

		// 2. If editing a freshly created item whose real ID has not arrived from server yet:
		if (id.startsWith("temp_")) {
			const pendingCreation = this.pendingCreations.get(id);
			if (pendingCreation) {
				pendingCreation.pendingPatch = {
					...(pendingCreation.pendingPatch ?? {}),
					...patch,
				};
				return currentList[itemIndex];
			}
		}

		// 3. Trailing Worker check
		let worker = this.updateWorkers.get(id);
		if (!worker) {
			worker = {
				inFlight: false,
				pendingPatch: null,
				baselineRecord: currentRecord,
			};
			this.updateWorkers.set(id, worker);
		}

		const activeWorker = worker;

		// If a request is already in flight, buffer this patch into the trailing slot
		if (activeWorker.inFlight) {
			activeWorker.pendingPatch = { ...(activeWorker.pendingPatch ?? {}), ...patch };
			return currentList[itemIndex];
		}

		// 4. First request: start in-flight execution immediately
		activeWorker.inFlight = true;
		let patchToSend: Partial<T> = patch;

		try {
			while (true) {
				const updatedRecord = await pb.collection(collectionName).update<T>(id, patchToSend);

				// Advance baseline to the last successfully confirmed server record
				activeWorker.baselineRecord = updatedRecord;

				// ALWAYS fetch freshest array via getter closure after each await
				const freshList = [...getList()];
				const latestIndex = freshList.findIndex((item) => item.id === id);
				if (latestIndex !== -1) {
					freshList[latestIndex] = updatedRecord;
					setList(freshList);
				}

				// Check if newer user actions were buffered while waiting for the server
				if (activeWorker.pendingPatch) {
					patchToSend = activeWorker.pendingPatch as Partial<T>;
					activeWorker.pendingPatch = null;
					// Continue loop to send the trailing patch
				} else {
					// All trailing patches resolved
					this.updateWorkers.delete(id);
					return updatedRecord;
				}
			}
		} catch (err: any) {
			const freshList = [...getList()];
			const latestIndex = freshList.findIndex((item) => item.id === id);

			if (err?.status === 404) {
				// Record was deleted on the server! Remove dead record from UI instead of restoring baseline
				if (latestIndex !== -1) {
					freshList.splice(latestIndex, 1);
					setList(freshList);
				}
				console.warn(`Record ${id} was deleted on the server. Removed from UI.`);
			} else {
				// Network or validation error: Roll back to last known confirmed baseline
				if (latestIndex !== -1) {
					freshList[latestIndex] = activeWorker.baselineRecord;
					setList(freshList);
				}
			}
			this.updateWorkers.delete(id);
			console.error(`Failed to update ${collectionName} record:`, err);
			throw err;
		}
	}

	private async optimisticDelete<T extends { id: string }>(
		getList: () => T[],
		setList: (items: T[]) => void,
		collectionName: string,
		id: string,
	): Promise<void> {
		const currentList = [...getList()];
		const itemIndex = currentList.findIndex((item) => item.id === id);
		if (itemIndex === -1) return;

		this.updateWorkers.delete(id);

		const originalRecord = currentList[itemIndex];
		currentList.splice(itemIndex, 1);
		setList(currentList);

		if (id.startsWith("temp_")) {
			const pendingCreation = this.pendingCreations.get(id);
			if (pendingCreation) {
				pendingCreation.isCancelled = true;
			}
			return;
		}

		this.pendingDeletes.add(id);
		try {
			await pb.collection(collectionName).delete(id);
		} catch (err: any) {
			if (err?.status === 404) {
				// Already deleted on the server! Desired state achieved, do NOT resurrect.
				return;
			}
			// Only restore on genuine network / authorization failures
			const freshList = [...getList()];
			freshList.splice(itemIndex, 0, originalRecord);
			setList(freshList);
			console.error(`Failed to delete ${collectionName} record, rolled back:`, err);
			throw err;
		} finally {
			this.pendingDeletes.delete(id);
		}
	}

	// Optimistic Mutations for Events
	async addEvent(eventData: Omit<EventRecord, "id">): Promise<EventRecord> {
		return this.optimisticAdd(
			() => this._events,
			(val) => (this._events = val),
			"events",
			eventData,
		);
	}

	async updateEvent(id: string, patch: Partial<EventRecord>): Promise<EventRecord> {
		return this.optimisticUpdate(
			() => this._events,
			(val) => (this._events = val),
			"events",
			id,
			patch,
		);
	}

	async deleteEvent(id: string): Promise<void> {
		return this.optimisticDelete(
			() => this._events,
			(val) => (this._events = val),
			"events",
			id,
		);
	}

	// Optimistic Mutations for Tasks
	async addTask(taskData: Omit<TaskRecord, "id">): Promise<TaskRecord> {
		return this.optimisticAdd(
			() => this._tasks,
			(val) => (this._tasks = val),
			"tasks",
			taskData,
		);
	}

	async updateTask(id: string, patch: Partial<TaskRecord>): Promise<TaskRecord> {
		return this.optimisticUpdate(
			() => this._tasks,
			(val) => (this._tasks = val),
			"tasks",
			id,
			patch,
		);
	}

	async deleteTask(id: string): Promise<void> {
		return this.optimisticDelete(
			() => this._tasks,
			(val) => (this._tasks = val),
			"tasks",
			id,
		);
	}

	// Optimistic Mutations for Calendars
	async addCalendar(calendarData: Omit<CalendarRecord, "id">): Promise<CalendarRecord> {
		return this.optimisticAdd(
			() => this._calendars,
			(val) => (this._calendars = val),
			"calendars",
			calendarData,
		);
	}

	async updateCalendar(id: string, patch: Partial<CalendarRecord>): Promise<CalendarRecord> {
		return this.optimisticUpdate(
			() => this._calendars,
			(val) => (this._calendars = val),
			"calendars",
			id,
			patch,
		);
	}

	async deleteCalendar(id: string): Promise<void> {
		return this.optimisticDelete(
			() => this._calendars,
			(val) => (this._calendars = val),
			"calendars",
			id,
		);
	}

	// Optimistic Mutations for Sync Status
	async updateSyncStatus(patch: Partial<SyncStatusRecord>): Promise<SyncStatusRecord | null> {
		if (!pb.authStore.isValid || !pb.authStore.record?.id) return null;
		const userId = pb.authStore.record.id;

		if (this._syncStatus) {
			const previous = { ...this._syncStatus };
			this._syncStatus = { ...this._syncStatus, ...patch };
			try {
				const updated = await pb.collection("sync_status").update<SyncStatusRecord>(this._syncStatus.id, patch);
				this._syncStatus = updated;
				return updated;
			} catch (err) {
				this._syncStatus = previous;
				console.error("Failed to update sync_status:", err);
				throw err;
			}
		} else {
			try {
				let record: SyncStatusRecord;
				try {
					record = await pb.collection("sync_status").getFirstListItem<SyncStatusRecord>(`user = "${userId}"`);
					record = await pb.collection("sync_status").update<SyncStatusRecord>(record.id, patch);
				} catch {
					record = await pb.collection("sync_status").create<SyncStatusRecord>({
						user: userId,
						canvas_status: "idle",
						google_import_status: "idle",
						google_export_status: "idle",
						google_export_enabled: true,
						...patch,
					});
				}
				this._syncStatus = record;
				return record;
			} catch (err) {
				console.error("Failed to create/update sync_status:", err);
				throw err;
			}
		}
	}

	async setGoogleExportEnabled(enabled: boolean): Promise<void> {
		await this.updateSyncStatus({ google_export_enabled: enabled });
	}
}

export const dataState = new DataState();
