import { pb, POCKETBASE_URL } from "./pocketbase";
import { authState } from "$lib/authState.svelte";
import { dataState } from "$lib/dataState/dataState.svelte";

export type SyncStep = "canvas" | "google-in" | "google-out" | null;

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

class SyncState {
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
			await dataState.refresh();
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
			await dataState.refresh();
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
			return { success: true, message: "Google Calendar export is disabled in settings.", syncedCount: 0 };
		}
		if (this.isSyncingGoogleOut || dataState.isGoogleExporting || (!fromSyncAll && this.isSyncingAll)) return;
		this.syncAllError = null;
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
			await dataState.refresh();
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
			// Step 1: Canvas LMS (if connected)
			if (this.isCanvasConnected) {
				this.syncingStep = "canvas";
				this.canvasSynced = true;
				if (options?.onStepChange) await options.onStepChange("canvas");
				try {
					const cRes = await this.syncCanvas(true);
					results.push(`Canvas (${cRes?.tasksSynced ?? "all"} tasks)`);
				} catch (e: any) {
					errors.push(`Canvas: ${e.message || "failed"}`);
				}
			}

			this.syncingStep = null;
			if (options?.onStepChange) await options.onStepChange(null);

			// Step 2: Google Inbound (if connected)
			if (this.isGoogleConnected) {
				this.syncingStep = "google-in";
				this.googleInSynced = true;
				if (options?.onStepChange) await options.onStepChange("google-in");
				try {
					const gFromRes = await this.syncFromGoogle(true);
					results.push(
						`Google In (${gFromRes?.calendarsSynced ?? gFromRes?.calendarCount ?? 0} cals, ${gFromRes?.eventsSynced ?? gFromRes?.eventCount ?? 0} events)`,
					);
				} catch (e: any) {
					errors.push(`Google Inbound: ${e.message || "failed"}`);
				}
			}

			this.syncingStep = null;
			if (options?.onStepChange) await options.onStepChange(null);

			// Step 3: Google Outbound (if connected and export enabled)
			if (this.isGoogleConnected && dataState.isGoogleExportEnabled) {
				this.syncingStep = "google-out";
				this.googleOutSynced = true;
				if (options?.onStepChange) await options.onStepChange("google-out");
				try {
					const gToRes = await this.syncToGoogle(true);
					results.push(`Google Out (${gToRes?.syncedCount ?? 0} pushed)`);
				} catch (e: any) {
					errors.push(`Google Outbound: ${e.message || "failed"}`);
				}
			}

			if (results.length > 0) {
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

export const syncState = new SyncState();
