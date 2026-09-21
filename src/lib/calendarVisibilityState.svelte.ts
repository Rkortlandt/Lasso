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
	isHiddenInSidebar(calId: string, calName?: string): boolean {
		if (!calId && !calName) return false;
		if (calId && this.hiddenInSidebarSet.has(calId)) return true;
		if (calName && this.hiddenInSidebarSet.has(calName)) return true;
		return false;
	}

	isShownInSidebar(calId: string, calName?: string): boolean {
		return !this.isHiddenInSidebar(calId, calName);
	}

	setSidebarVisibility(calId: string, visible: boolean, calName?: string) {
		if (!calId && !calName) return;
		const next = new Set(this.hiddenInSidebarSet);
		if (visible) {
			if (calId) next.delete(calId);
			if (calName) next.delete(calName);
		} else {
			if (calId) next.add(calId);
			if (calName) next.add(calName);
		}
		this.hiddenInSidebarSet = next;
		this.saveSidebarToStorage();
	}

	toggleSidebarVisibility(calId: string, calName?: string) {
		this.setSidebarVisibility(
			calId,
			this.isHiddenInSidebar(calId, calName),
			calName,
		);
	}

	// Main Calendar grid visibility controls (configured in Sidebar)
	isHiddenOnCalendar(calId: string, calName?: string): boolean {
		if (!calId && !calName) return false;
		if (calId && this.hiddenOnCalendarSet.has(calId)) return true;
		if (calName && this.hiddenOnCalendarSet.has(calName)) return true;
		return false;
	}

	isShownOnCalendar(calId: string, calName?: string): boolean {
		return !this.isHiddenOnCalendar(calId, calName);
	}

	setCalendarVisibility(calId: string, visible: boolean, calName?: string) {
		if (!calId && !calName) return;
		const next = new Set(this.hiddenOnCalendarSet);
		if (visible) {
			if (calId) next.delete(calId);
			if (calName) next.delete(calName);
		} else {
			if (calId) next.add(calId);
			if (calName) next.add(calName);
		}
		this.hiddenOnCalendarSet = next;
		this.saveCalendarToStorage();
	}

	toggleCalendarVisibility(calId: string, calName?: string) {
		this.setCalendarVisibility(calId, this.isHiddenOnCalendar(calId, calName), calName);
	}

	// Isolate controls (configured in Sidebar via eyedropper)
	isIsolated(calId: string, calName?: string): boolean {
		if (!this.isolated) return false;
		if (calId && this.isolated === calId) return true;
		if (calName && this.isolated === calName) return true;
		return false;
	}

	setIsolated(calId: string | null) {
		this.isolated = calId;
		this.saveIsolatedToStorage();
	}

	toggleIsolate(calId: string, calName?: string) {
		if (this.isolated === calId || (calName && this.isolated === calName)) {
			this.setIsolated(null);
		} else {
			this.setIsolated(calId);
		}
	}

	// Full Grid Visibility: A calendar appears on the grid ONLY if it is neither hidden from sidebar nor hidden from calendar.
	// If a calendar is isolated, it takes precedence over hidden states and ONLY the isolated calendar is shown.
	isHiddenFromGrid(calId: string, calName?: string): boolean {
		if (this.isolated !== null) {
			const isThisIsolated =
				(calId && this.isolated === calId) ||
				(calName && this.isolated === calName);
			return !isThisIsolated;
		}

		if (this.isHiddenInSidebar(calId, calName)) return true;
		if (this.isHiddenOnCalendar(calId, calName)) return true;
		return false;
	}

	isShownOnGrid(calId: string, calName?: string): boolean {
		return !this.isHiddenFromGrid(calId, calName);
	}
}

export const calendarVisibilityState = new CalendarVisibilityState();
