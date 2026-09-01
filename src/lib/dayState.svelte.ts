import {
	today,
	getLocalTimeZone,
	parseDate,
	CalendarDate,
	type DateValue
} from '@internationalized/date';

class DayState {
	value = $state<DateValue>(today(getLocalTimeZone()));

	/**
	 * Get the currently selected day.
	 */
	get(): DateValue {
		return this.value;
	}

	/**
	 * Set the currently selected day.
	 * Accepts DateValue, JS Date, or "YYYY-MM-DD" string.
	 */
	set(newDay: DateValue | Date | string | null | undefined): void {
		if (!newDay) {
			this.value = today(getLocalTimeZone());
			return;
		}

		if (typeof newDay === 'string') {
			try {
				this.value = parseDate(newDay);
			} catch {
				const d = new Date(newDay);
				this.value = new CalendarDate(d.getFullYear(), d.getMonth() + 1, d.getDate());
			}
			return;
		}

		if (newDay instanceof Date) {
			this.value = new CalendarDate(newDay.getFullYear(), newDay.getMonth() + 1, newDay.getDate());
			return;
		}

		this.value = newDay;
	}

	/**
	 * Increment the currently selected day by N days (default 1).
	 */
	inc(days = 1): DateValue {
		this.value = (this.value as CalendarDate).add({ days });
		return this.value;
	}

	/**
	 * Decrement the currently selected day by N days (default 1).
	 */
	dec(days = 1): DateValue {
		this.value = (this.value as CalendarDate).subtract({ days });
		return this.value;
	}

	/**
	 * Reset selection to today.
	 */
	today(): DateValue {
		this.value = today(getLocalTimeZone());
		return this.value;
	}

	/**
	 * Convert the selected day to a native JavaScript Date object.
	 */
	toDate(): Date {
		return new Date(this.value.year, this.value.month - 1, this.value.day);
	}

	/**
	 * Convert the selected day to an ISO date string (YYYY-MM-DD).
	 */
	toString(): string {
		return this.value.toString();
	}
}

export const dayState = new DayState();
export const dateState = dayState;
