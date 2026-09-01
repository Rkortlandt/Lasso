class CalendarVisibilityState {
	// Calendar IDs hidden from the sidebar (configured in Settings)
	private hiddenInSidebarSet = $state<Set<string>>(new Set());

	// Calendar IDs hidden from the calendar grid (toggled in Sidebar)
	private hiddenOnCalendarSet = $state<Set<string>>(new Set());

	// Currently isolated calendar ID (null if none isolated)
	isolated = $state<string | null>(null);

	constructor() {
		if (typeof window !== "undefined") {
			this.loadFromStorage();
		}
	}

	private loadFromStorage() {
		try {
			const sidebarJson = localStorage.getItem("lasso_calendars_hidden_in_sidebar");
			if (sidebarJson) {
				const arr = JSON.parse(sidebarJson);
				if (Array.isArray(arr)) {
					this.hiddenInSidebarSet = new Set(arr);
				}
			}

			const calJson = localStorage.getItem("lasso_calendars_hidden_on_calendar");
			if (calJson) {
				const arr = JSON.parse(calJson);
				if (Array.isArray(arr)) {
					this.hiddenOnCalendarSet = new Set(arr);
				}
			}

			const isolatedVal = localStorage.getItem("lasso_calendar_isolated");
			if (isolatedVal && isolatedVal !== "null" && isolatedVal !== "undefined") {
				this.isolated = isolatedVal;
			} else {
				this.isolated = null;
			}
		} catch (e) {
			console.warn("Failed to load calendar visibility from localStorage:", e);
		}
	}

	private saveSidebarToStorage() {
		if (typeof window === "undefined") return;
		try {
			localStorage.setItem(
				"lasso_calendars_hidden_in_sidebar",
				JSON.stringify(Array.from(this.hiddenInSidebarSet)),
			);
		} catch (e) {
			console.warn("Failed to save sidebar visibility to localStorage:", e);
		}
	}

	private saveCalendarToStorage() {
		if (typeof window === "undefined") return;
		try {
			localStorage.setItem(
				"lasso_calendars_hidden_on_calendar",
				JSON.stringify(Array.from(this.hiddenOnCalendarSet)),
			);
		} catch (e) {
			console.warn("Failed to save calendar visibility to localStorage:", e);
		}
	}

	private saveIsolatedToStorage() {
		if (typeof window === "undefined") return;
		try {
			if (this.isolated) {
				localStorage.setItem("lasso_calendar_isolated", this.isolated);
			} else {
				localStorage.removeItem("lasso_calendar_isolated");
			}
		} catch (e) {
			console.warn("Failed to save isolated calendar to localStorage:", e);
		}
	}

	// Sidebar visibility controls (configured in Settings)
	isHiddenInSidebar(calId: string, secondaryKey?: string): boolean {
		if (!calId && !secondaryKey) return false;
		if (calId && this.hiddenInSidebarSet.has(calId)) return true;
		if (secondaryKey && this.hiddenInSidebarSet.has(secondaryKey)) return true;
		return false;
	}

	isShownInSidebar(calId: string, secondaryKey?: string): boolean {
		return !this.isHiddenInSidebar(calId, secondaryKey);
	}

	setSidebarVisibility(calId: string, visible: boolean, secondaryKey?: string) {
		if (!calId && !secondaryKey) return;
		const next = new Set(this.hiddenInSidebarSet);
		if (visible) {
			if (calId) next.delete(calId);
			if (secondaryKey) next.delete(secondaryKey);
		} else {
			if (calId) next.add(calId);
			if (secondaryKey) next.add(secondaryKey);
		}
		this.hiddenInSidebarSet = next;
		this.saveSidebarToStorage();
	}

	toggleSidebarVisibility(calId: string, secondaryKey?: string) {
		this.setSidebarVisibility(
			calId,
			this.isHiddenInSidebar(calId, secondaryKey),
			secondaryKey,
		);
	}

	// Main Calendar grid visibility controls (configured in Sidebar)
	isHiddenOnCalendar(calId: string, secondaryKey?: string): boolean {
		if (!calId && !secondaryKey) return false;
		if (calId && this.hiddenOnCalendarSet.has(calId)) return true;
		if (secondaryKey && this.hiddenOnCalendarSet.has(secondaryKey)) return true;
		return false;
	}

	isShownOnCalendar(calId: string, secondaryKey?: string): boolean {
		return !this.isHiddenOnCalendar(calId, secondaryKey);
	}

	setCalendarVisibility(calId: string, visible: boolean) {
		if (!calId) return;
		const next = new Set(this.hiddenOnCalendarSet);
		if (visible) {
			next.delete(calId);
		} else {
			next.add(calId);
		}
		this.hiddenOnCalendarSet = next;
		this.saveCalendarToStorage();
	}

	toggleCalendarVisibility(calId: string) {
		this.setCalendarVisibility(calId, this.isHiddenOnCalendar(calId));
	}

	// Isolate controls (configured in Sidebar via eyedropper)
	isIsolated(calId: string, secondaryKey?: string): boolean {
		if (!this.isolated) return false;
		if (calId && this.isolated === calId) return true;
		if (secondaryKey && this.isolated === secondaryKey) return true;
		return false;
	}

	setIsolated(calId: string | null) {
		this.isolated = calId;
		this.saveIsolatedToStorage();
	}

	toggleIsolate(calId: string) {
		if (this.isolated === calId) {
			this.setIsolated(null);
		} else {
			this.setIsolated(calId);
		}
	}

	// Full Grid Visibility: A calendar appears on the grid ONLY if it is neither hidden from sidebar nor hidden from calendar.
	// If a calendar is isolated, it takes precedence over hidden states and ONLY the isolated calendar is shown.
	isHiddenFromGrid(calId: string, secondaryKey?: string): boolean {
		if (this.isolated !== null) {
			const isThisIsolated =
				(calId && this.isolated === calId) ||
				(secondaryKey && this.isolated === secondaryKey);
			return !isThisIsolated;
		}

		if (this.isHiddenInSidebar(calId, secondaryKey)) return true;
		if (this.isHiddenOnCalendar(calId, secondaryKey)) return true;
		return false;
	}

	isShownOnGrid(calId: string, secondaryKey?: string): boolean {
		return !this.isHiddenFromGrid(calId, secondaryKey);
	}
}

export const calendarVisibilityState = new CalendarVisibilityState();
