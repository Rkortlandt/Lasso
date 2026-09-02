import { canvasState } from "$lib/canvasState.svelte";
import { googleCalendarState } from "$lib/googleCalendarState.svelte";

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

	// Global isAnySyncing helper across all services
	isAnySyncing = $derived(
		this.isSyncingAll ||
			canvasState.isSyncing ||
			googleCalendarState.isSyncingFromGoogle ||
			googleCalendarState.isSyncingToGoogle,
	);

	async syncCanvas(): Promise<void> {
		if (!canvasState.isConnected) {
			this.syncAllError =
				"Canvas LMS is not connected. Configure Canvas in settings.";
			return;
		}
		if (this.isAnySyncing) return;
		this.syncAllError = null;
		this.canvasSynced = true;
		try {
			await canvasState.syncCanvas();
		} catch (e: any) {
			console.error("Canvas sync failed:", e);
			this.syncAllError = `Canvas sync failed: ${e.message || "Unknown error"}`;
			throw e;
		}
	}

	async syncFromGoogle(): Promise<void> {
		if (!googleCalendarState.isConnected) {
			this.syncAllError =
				"Google Calendar is not connected. Sign in to Google in settings.";
			return;
		}
		if (this.isAnySyncing) return;
		this.syncAllError = null;
		this.googleInSynced = true;
		try {
			await googleCalendarState.syncFromGoogle();
		} catch (e: any) {
			console.error("Google Inbound sync failed:", e);
			this.syncAllError = `Google Inbound sync failed: ${e.message || "Unknown error"}`;
			throw e;
		}
	}

	async syncToGoogle(): Promise<void> {
		if (!googleCalendarState.isConnected) {
			this.syncAllError =
				"Google Calendar is not connected. Sign in to Google in settings.";
			return;
		}
		if (this.isAnySyncing) return;
		this.syncAllError = null;
		this.googleOutSynced = true;
		try {
			await googleCalendarState.syncToGoogle();
		} catch (e: any) {
			console.error("Google Outbound sync failed:", e);
			this.syncAllError = `Google Outbound sync failed: ${e.message || "Unknown error"}`;
			throw e;
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

		if (!canvasState.isConnected && !googleCalendarState.isConnected) {
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
			if (canvasState.isConnected) {
				this.syncingStep = "canvas";
				this.canvasSynced = true;
				if (options?.onStepChange) await options.onStepChange("canvas");
				try {
					const cRes = await canvasState.syncCanvas();
					results.push(`Canvas (${cRes?.tasksSynced ?? "all"} tasks)`);
				} catch (e: any) {
					errors.push(`Canvas: ${e.message || "failed"}`);
				}
			}

			this.syncingStep = null;
			if (options?.onStepChange) await options.onStepChange(null);

			// Step 2: Google Inbound (if connected)
			if (googleCalendarState.isConnected) {
				this.syncingStep = "google-in";
				this.googleInSynced = true;
				if (options?.onStepChange) await options.onStepChange("google-in");
				try {
					const gFromRes = await googleCalendarState.syncFromGoogle();
					results.push(
						`Google In (${gFromRes?.calendarCount ?? 0} cals, ${gFromRes?.eventCount ?? 0} events)`,
					);
				} catch (e: any) {
					errors.push(`Google Inbound: ${e.message || "failed"}`);
				}
			}

			this.syncingStep = null;
			if (options?.onStepChange) await options.onStepChange(null);

			// Step 3: Google Outbound (if connected)
			if (googleCalendarState.isConnected) {
				this.syncingStep = "google-out";
				this.googleOutSynced = true;
				if (options?.onStepChange) await options.onStepChange("google-out");
				try {
					const gToRes = await googleCalendarState.syncToGoogle();
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
