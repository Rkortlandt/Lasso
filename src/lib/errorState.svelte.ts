/**
 * Global reactive error state singleton.
 * Manages transient top-bar error messages that auto-dismiss after a duration (default 3500ms).
 */
class ErrorState {
	private _currentError = $state<string | null>(null);
	private _isVisible = $state(false);
	private _mode = $state<"error" | "warning" | "default">("error");
	private _dismissTimer: ReturnType<typeof setTimeout> | null = null;
	private _clearTextTimer: ReturnType<typeof setTimeout> | null = null;

	get currentError(): string | null {
		return this._currentError;
	}

	get isVisible(): boolean {
		return this._isVisible;
	}

	get mode(): "error" | "warning" | "default" {
		return this._mode;
	}

	/**
	 * Shows an error or warning banner at the top of the calendar.
	 * @param message Human-readable error message.
	 * @param durationMs Duration in ms before auto-dismissing (defaults to 3500ms).
	 * @param mode Visual styling: "error" (red), "warning" (amber), or "default" (primary).
	 */
	show(message: string, durationMs = 3500, mode: "error" | "warning" | "default" = "error") {
		if (this._dismissTimer) {
			clearTimeout(this._dismissTimer);
			this._dismissTimer = null;
		}
		if (this._clearTextTimer) {
			clearTimeout(this._clearTextTimer);
			this._clearTextTimer = null;
		}

		this._currentError = message;
		this._mode = mode;
		this._isVisible = true;

		if (durationMs > 0) {
			this._dismissTimer = setTimeout(() => {
				this.clear();
			}, durationMs);
		}
	}

	/**
	 * Immediately begins sliding up and hiding the current error,
	 * retaining the text until the slide-up animation completes.
	 */
	clear() {
		if (this._dismissTimer) {
			clearTimeout(this._dismissTimer);
			this._dismissTimer = null;
		}
		this._isVisible = false;

		// Retain error message during 350ms slide-up transition so text doesn't swap prematurely
		if (this._clearTextTimer) {
			clearTimeout(this._clearTextTimer);
		}
		this._clearTextTimer = setTimeout(() => {
			if (!this._isVisible) {
				this._currentError = null;
			}
		}, 400);
	}
}

/**
 * Detects whether an error is caused by backend server unavailability,
 * network drop, CORS/connection refused, or request timeout/abort.
 */
export function isConnectionError(err: any): boolean {
	if (!err) return false;
	if (typeof navigator !== "undefined" && !navigator.onLine) return true;
	if (err.status === 0 || err.status === 502 || err.status === 503 || err.status === 504) return true;
	if (err.isAbort) return true;
	const name = err.name || "";
	const message = (err.message || "").toLowerCase();
	if (name === "AbortError") return true;
	if (name === "TypeError" && (message.includes("fetch") || message.includes("network") || message.includes("failed"))) return true;
	if (
		message.includes("failed to fetch") ||
		message.includes("networkerror") ||
		message.includes("network error") ||
		message.includes("err_connection_refused") ||
		message.includes("err_name_not_resolved") ||
		message.includes("connection refused") ||
		message.includes("load failed")
	) {
		return true;
	}
	return false;
}

export const errorState = new ErrorState();
