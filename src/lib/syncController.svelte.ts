import { pb, POCKETBASE_URL } from "./pocketbase";
import { authState } from "$lib/authState.svelte";
import { dataState } from "$lib/dataState/dataState.svelte";

export type SyncStep = "canvas" | "google-in" | "google-out" | "inbound" | null;

export interface SyncAllOptions {
	onStepChange?: (step: SyncStep) => Promise<void> | void;
}

export interface SyncAllResult {
	success: boolean;
	results: string[];
	errors: string[];
	message: string | null;
	error: string | null;
}

class SyncController {
	isSyncingAll = $state(false);
	syncingStep = $state<SyncStep>(null);
	syncAllMessage = $state<string | null>(null);
	syncAllError = $state<string | null>(null);

	// Progression flags for each service
	canvasSynced = $state(false);
	googleInSynced = $state(false);
	googleOutSynced = $state(false);

	isSyncingCanvas = $state(false);
	isSyncingGoogleIn = $state(false);
	isSyncingGoogleOut = $state(false);

	// Derived stage tracking (1 = Inbound, 2 = Outbound, null = idle)
	get syncingStage(): 1 | 2 | null {
		if (
			this.syncingStep === "inbound" ||
			this.isSyncingCanvas ||
			this.isSyncingGoogleIn ||
			dataState.isCanvasSyncing ||
			dataState.isGoogleImporting
		) {
			return 1;
		}
		if (
			this.syncingStep === "google-out" ||
			this.isSyncingGoogleOut ||
			dataState.isGoogleExporting
		) {
			return 2;
		}
		return null;
	}

	// Global isAnySyncing helper across all services
	isAnySyncing = $derived(
		this.isSyncingAll ||
			this.isSyncingCanvas ||
			this.isSyncingGoogleIn ||
			this.isSyncingGoogleOut ||
			dataState.isCanvasSyncing ||
			dataState.isGoogleImporting ||
			dataState.isGoogleExporting,
	);

	get isCanvasConnected(): boolean {
		return Boolean(authState.record?.canvas_connected);
	}

	get isGoogleConnected(): boolean {
		return Boolean(authState.record?.google_connected);
	}

	async syncCanvas(fromSyncAll = false): Promise<any> {
		if (!this.isCanvasConnected) {
			const err =
				"Canvas LMS is not connected. Configure Canvas in settings.";
			this.syncAllError = err;
			return;
		}
		if (this.isSyncingCanvas || dataState.isCanvasSyncing || (!fromSyncAll && this.isSyncingAll)) return;
		this.syncAllError = null;
		if (!fromSyncAll) this.syncAllMessage = null;
		this.isSyncingCanvas = true;
		this.canvasSynced = true;

		try {
			const canvasUrl =
				authState.record?.canvas_url || "https://canvas.instructure.com";
			const res = await fetch(`${POCKETBASE_URL}/api/sync/canvas`, {
				method: "POST",
				headers: {
					"Content-Type": "application/json",
					Authorization: pb.authStore.token ? `Bearer ${pb.authStore.token}` : "",
				},
				body: JSON.stringify({ canvasUrl }),
			});
			const data = await res.json();
			if (!res.ok || !data.success) {
				throw new Error(
					data.message ||
						data.error ||
						`Canvas sync failed (HTTP ${res.status})`,
				);
			}
			if (!fromSyncAll) {
				await dataState.refresh();
				const created = data?.tasksCreated ?? 0;
				const updated = data?.tasksUpdated ?? 0;
				if (created === 0 && updated === 0) {
					this.syncAllMessage = "Canvas LMS is up to date.";
				} else {
					const parts: string[] = [];
					if (created > 0) parts.push(`${created} created`);
					if (updated > 0) parts.push(`${updated} updated`);
					this.syncAllMessage = `Canvas LMS synced (${parts.join(", ")}).`;
				}
			}
			return data;
		} catch (e: any) {
			console.error("Canvas sync failed:", e);
			this.syncAllError = `Canvas sync failed: ${e.message || "Unknown error"}`;
			throw e;
		} finally {
			this.isSyncingCanvas = false;
		}
	}

	async syncFromGoogle(fromSyncAll = false): Promise<any> {
		if (!this.isGoogleConnected) {
			const err =
				"Google Calendar is not connected. Sign in to Google in settings.";
			this.syncAllError = err;
			return;
		}
		if (this.isSyncingGoogleIn || dataState.isGoogleImporting || (!fromSyncAll && this.isSyncingAll)) return;
		this.syncAllError = null;
		if (!fromSyncAll) this.syncAllMessage = null;
		this.isSyncingGoogleIn = true;
		this.googleInSynced = true;

		try {
			const res = await fetch(`${POCKETBASE_URL}/api/google/sync-inbound`, {
				method: "POST",
				headers: {
					"Content-Type": "application/json",
					Authorization: pb.authStore.token ? `Bearer ${pb.authStore.token}` : "",
				},
			});
			const data = await res.json();
			if (!res.ok || !data.success) {
				throw new Error(
					data.message ||
						data.error ||
						`Google Inbound sync failed (HTTP ${res.status})`,
				);
			}
			if (!fromSyncAll) {
				await dataState.refresh();
				const created = data?.eventsCreated ?? 0;
				const updated = data?.eventsUpdated ?? 0;
				const deleted = data?.eventsDeleted ?? 0;
				if (created === 0 && updated === 0 && deleted === 0) {
					this.syncAllMessage = "Google Calendar is up to date.";
				} else {
					const parts: string[] = [];
					if (created > 0) parts.push(`${created} created`);
					if (updated > 0) parts.push(`${updated} updated`);
					if (deleted > 0) parts.push(`${deleted} removed`);
					this.syncAllMessage = `Google Calendar synced (${parts.join(", ")}).`;
				}
			}
			return data;
		} catch (e: any) {
			console.error("Google Inbound sync failed:", e);
			this.syncAllError = `Google Inbound sync failed: ${e.message || "Unknown error"}`;
			throw e;
		} finally {
			this.isSyncingGoogleIn = false;
		}
	}

	async syncToGoogle(fromSyncAll = false): Promise<any> {
		if (!this.isGoogleConnected) {
			const err =
				"Google Calendar is not connected. Sign in to Google in settings.";
			this.syncAllError = err;
			return;
		}
		if (!dataState.isGoogleExportEnabled) {
			const msg = "Google Calendar export is disabled in settings.";
			if (!fromSyncAll) this.syncAllMessage = msg;
			return { success: true, message: msg, syncedCount: 0 };
		}
		if (this.isSyncingGoogleOut || dataState.isGoogleExporting || (!fromSyncAll && this.isSyncingAll)) return;
		this.syncAllError = null;
		if (!fromSyncAll) this.syncAllMessage = null;
		this.isSyncingGoogleOut = true;
		this.googleOutSynced = true;

		try {
			const res = await fetch(`${POCKETBASE_URL}/api/google/sync`, {
				method: "POST",
				headers: {
					"Content-Type": "application/json",
					Authorization: pb.authStore.token ? `Bearer ${pb.authStore.token}` : "",
				},
			});
			const data = await res.json();
			if (!res.ok || !data.success) {
				throw new Error(
					data.message ||
						data.error ||
						`Google Outbound sync failed (HTTP ${res.status})`,
				);
			}
			if (!fromSyncAll) {
				await dataState.refresh();
				const created = data?.createdCount ?? 0;
				const updated = data?.updatedCount ?? 0;
				const deleted = data?.deletedCount ?? 0;
				if (created === 0 && updated === 0 && deleted === 0) {
					this.syncAllMessage = "Google Calendar export is up to date.";
				} else {
					const parts: string[] = [];
					if (created > 0) parts.push(`${created} created`);
					if (updated > 0) parts.push(`${updated} updated`);
					if (deleted > 0) parts.push(`${deleted} removed`);
					this.syncAllMessage = `Google Calendar export completed (${parts.join(", ")}).`;
				}
			}
			return data;
		} catch (e: any) {
			console.error("Google Outbound sync failed:", e);
			this.syncAllError = `Google Outbound sync failed: ${e.message || "Unknown error"}`;
			throw e;
		} finally {
			this.isSyncingGoogleOut = false;
		}
	}

	async syncAll(options?: SyncAllOptions): Promise<SyncAllResult> {
		if (this.isAnySyncing) {
			return {
				success: false,
				results: [],
				errors: ["A sync is already in progress"],
				message: null,
				error: "A sync is already in progress",
			};
		}

		if (!this.isCanvasConnected && !this.isGoogleConnected) {
			const err =
				"No services connected. Please connect Canvas LMS or Google Calendar.";
			this.syncAllError = err;
			return {
				success: false,
				results: [],
				errors: [err],
				message: null,
				error: err,
			};
		}

		this.isSyncingAll = true;
		this.syncAllMessage = null;
		this.syncAllError = null;

		const results: string[] = [];
		const errors: string[] = [];

		try {
			// Phase 1: Run both Inbound imports (Canvas LMS & Google Inbound) concurrently!
			this.syncingStep = "inbound";
			if (options?.onStepChange) await options.onStepChange("inbound");

			const inboundTasks: Promise<void>[] = [];

			if (this.isCanvasConnected) {
				this.canvasSynced = true;
				inboundTasks.push(
					(async () => {
						try {
							const cRes = await this.syncCanvas(true);
							const created = cRes?.tasksCreated ?? 0;
							const updated = cRes?.tasksUpdated ?? 0;
							if (created === 0 && updated === 0) {
								results.push("Canvas (up to date)");
							} else {
								const parts: string[] = [];
								if (created > 0) parts.push(`${created} created`);
								if (updated > 0) parts.push(`${updated} updated`);
								results.push(`Canvas (${parts.join(", ")})`);
							}
						} catch (e: any) {
							errors.push(`Canvas: ${e.message || "failed"}`);
						}
					})(),
				);
			}

			if (this.isGoogleConnected) {
				this.googleInSynced = true;
				inboundTasks.push(
					(async () => {
						try {
							const gFromRes = await this.syncFromGoogle(true);
							const created = gFromRes?.eventsCreated ?? 0;
							const updated = gFromRes?.eventsUpdated ?? 0;
							const deleted = gFromRes?.eventsDeleted ?? 0;
							if (created === 0 && updated === 0 && deleted === 0) {
								results.push("Google In (up to date)");
							} else {
								const parts: string[] = [];
								if (created > 0) parts.push(`${created} created`);
								if (updated > 0) parts.push(`${updated} updated`);
								if (deleted > 0) parts.push(`${deleted} removed`);
								results.push(`Google In (${parts.join(", ")})`);
							}
						} catch (e: any) {
							errors.push(`Google Inbound: ${e.message || "failed"}`);
						}
					})(),
				);
			}

			if (inboundTasks.length > 0) {
				await Promise.all(inboundTasks);
			}

			this.syncingStep = null;
			if (options?.onStepChange) await options.onStepChange(null);

			// Phase 2: Google Outbound (Coursework Push)
			// Runs strictly after inbound imports complete so all fresh Canvas coursework is ready to push
			if (this.isGoogleConnected && dataState.isGoogleExportEnabled) {
				this.syncingStep = "google-out";
				this.googleOutSynced = true;
				if (options?.onStepChange) await options.onStepChange("google-out");
				try {
					const gToRes = await this.syncToGoogle(true);
					const created = gToRes?.createdCount ?? 0;
					const updated = gToRes?.updatedCount ?? 0;
					const deleted = gToRes?.deletedCount ?? 0;
					if (created === 0 && updated === 0 && deleted === 0) {
						results.push("Google Out (up to date)");
					} else {
						const parts: string[] = [];
						if (created > 0) parts.push(`${created} created`);
						if (updated > 0) parts.push(`${updated} updated`);
						if (deleted > 0) parts.push(`${deleted} removed`);
						results.push(`Google Out (${parts.join(", ")})`);
					}
				} catch (e: any) {
					errors.push(`Google Outbound: ${e.message || "failed"}`);
				}
			}

			if (results.length > 0) {
				await dataState.refresh();
				this.syncAllMessage = `Synced successfully: ${results.join(" • ")}`;
			}
			if (errors.length > 0) {
				this.syncAllError = errors.join("; ");
			}

			return {
				success: errors.length === 0,
				results,
				errors,
				message: this.syncAllMessage,
				error: this.syncAllError,
			};
		} catch (err: any) {
			const errMsg = err?.message || "Sync encountered an issue";
			this.syncAllError = errMsg;
			return {
				success: false,
				results,
				errors: [...errors, errMsg],
				message: null,
				error: errMsg,
			};
		} finally {
			this.syncingStep = null;
			this.isSyncingAll = false;
			if (options?.onStepChange) await options.onStepChange(null);
		}
	}
}

export const syncController = new SyncController();
