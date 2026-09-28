<script lang="ts">
	import { dataState } from "$lib/dataState/dataState.svelte";
	import { getCourseworkCalendars, resolveCalendarColor } from "$lib/dataState/calendarQueries.svelte";
	import type { EventRecord } from "$lib/dataState/dataRecordInterfaces";
	import X from "@lucide/svelte/icons/x";
	import CalendarIcon from "@lucide/svelte/icons/calendar";
	import Clock from "@lucide/svelte/icons/clock";
	import MapPin from "@lucide/svelte/icons/map-pin";
	import BookOpen from "@lucide/svelte/icons/book-open";
	import { portal } from "$lib/portal";

	interface Props {
		open: boolean;
		initialCalendarId?: string;
		editEvent?: EventRecord | null;
		onClose: () => void;
	}

	let { open = $bindable(false), initialCalendarId = "", editEvent = null, onClose }: Props = $props();

	const courses = $derived(getCourseworkCalendars());

	let selectedCalendarId = $state("");
	let title = $state("");
	let location = $state("");
	let selectedDays = $state<string[]>(["MO", "WE", "FR"]);
	let startTime = $state("10:00");
	let endTime = $state("11:15");
	let startDate = $state("");
	let endDate = $state("");
	let isSubmitting = $state(false);
	let errorMessage = $state("");

	const isEditing = $derived(Boolean(editEvent && editEvent.id));

	$effect(() => {
		if (open) {
			if (editEvent) {
				selectedCalendarId = editEvent.calendar || editEvent.expand?.calendar?.id || initialCalendarId || "";
				const targetCal = dataState.calendars.find((c) => c.id === selectedCalendarId);
				const courseName = targetCal?.nickname || targetCal?.name || "";

				// Parse Title
				let rawTitle = editEvent.title || "Lecture";
				if (courseName && rawTitle.startsWith(`${courseName}: `)) {
					rawTitle = rawTitle.slice(`${courseName}: `.length);
				} else if (courseName && rawTitle === `${courseName} Class`) {
					rawTitle = "Lecture";
				}
				title = rawTitle;

				// Parse Location
				if (editEvent.description) {
					location = editEvent.description.replace(/^Location:\s*/i, "");
				} else {
					location = "";
				}

				// Parse Start / End Times and Start Date
				if (editEvent.start) {
					const sIso = editEvent.start.includes(" ") ? editEvent.start.replace(" ", "T") : editEvent.start;
					const sD = new Date(sIso);
					if (!isNaN(sD.getTime())) {
						const y = sD.getFullYear();
						const m = String(sD.getMonth() + 1).padStart(2, "0");
						const d = String(sD.getDate()).padStart(2, "0");
						startDate = `${y}-${m}-${d}`;
						startTime = `${String(sD.getHours()).padStart(2, "0")}:${String(sD.getMinutes()).padStart(2, "0")}`;
					}
				}

				if (editEvent.end) {
					const eIso = editEvent.end.includes(" ") ? editEvent.end.replace(" ", "T") : editEvent.end;
					const eD = new Date(eIso);
					if (!isNaN(eD.getTime())) {
						endTime = `${String(eD.getHours()).padStart(2, "0")}:${String(eD.getMinutes()).padStart(2, "0")}`;
					}
				}

				// Parse Recurrence rule (BYDAY and UNTIL)
				if (editEvent.recurr) {
					const rrule = editEvent.recurr;
					const parts = rrule.replace(/^RRULE:/i, "").split(";");
					for (const part of parts) {
						if (part.startsWith("BYDAY=")) {
							selectedDays = part.replace("BYDAY=", "").split(",");
						}
						if (part.startsWith("UNTIL=")) {
							const rawUntil = part.replace("UNTIL=", "");
							if (rawUntil.length >= 8) {
								const uy = rawUntil.slice(0, 4);
								const um = rawUntil.slice(4, 6);
								const ud = rawUntil.slice(6, 8);
								endDate = `${uy}-${um}-${ud}`;
							}
						}
					}
				}

				errorMessage = "";
			} else {
				selectedCalendarId = initialCalendarId || (courses.length > 0 ? courses[0].id : "");
				const today = new Date();
				startDate = today.toISOString().slice(0, 10);
				// Default semester end: ~3.5 months out
				const termEnd = new Date(today.getFullYear(), today.getMonth() + 4, 0);
				endDate = termEnd.toISOString().slice(0, 10);
				title = "Lecture";
				location = "";
				selectedDays = ["MO", "WE", "FR"];
				startTime = "10:00";
				endTime = "11:15";
				errorMessage = "";
			}
		}
	});

	const dayOptions = [
		{ label: "M", value: "MO", name: "Monday", dayIndex: 1 },
		{ label: "T", value: "TU", name: "Tuesday", dayIndex: 2 },
		{ label: "W", value: "WE", name: "Wednesday", dayIndex: 3 },
		{ label: "Th", value: "TH", name: "Thursday", dayIndex: 4 },
		{ label: "F", value: "FR", name: "Friday", dayIndex: 5 },
		{ label: "Sa", value: "SA", name: "Saturday", dayIndex: 6 },
		{ label: "Su", value: "SU", name: "Sunday", dayIndex: 0 },
	];

	function toggleDay(val: string) {
		if (selectedDays.includes(val)) {
			if (selectedDays.length > 1) {
				selectedDays = selectedDays.filter((d) => d !== val);
			}
		} else {
			selectedDays = [...selectedDays, val];
		}
	}

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		if (!selectedCalendarId) {
			errorMessage = "Please select a course/calendar.";
			return;
		}
		if (selectedDays.length === 0) {
			errorMessage = "Please select at least one day of the week.";
			return;
		}
		if (!startDate || !endDate) {
			errorMessage = "Please provide start and end dates.";
			return;
		}
		if (startDate > endDate) {
			errorMessage = "Start date must be before end date.";
			return;
		}
		if (startTime >= endTime) {
			errorMessage = "End time must be after start time.";
			return;
		}

		isSubmitting = true;
		errorMessage = "";

		try {
			const targetCal = dataState.calendars.find((c) => c.id === selectedCalendarId);
			const courseName = targetCal?.nickname || targetCal?.name || "Class";

			const [startH, startM] = startTime.split(":").map(Number);
			const [endH, endM] = endTime.split(":").map(Number);

			// Parse start date components in local time
			const [startY, startMonth, startD] = startDate.split("-").map(Number);
			const startLocalDate = new Date(startY, startMonth - 1, startD);

			// Find first matching date on or after startDate that aligns with selectedDays
			let firstLocalDate = new Date(startLocalDate);
			const targetDayIndices = selectedDays.map((d) => dayOptions.find((opt) => opt.value === d)!.dayIndex);

			let found = false;
			for (let i = 0; i < 7; i++) {
				const checkDate = new Date(startLocalDate.getFullYear(), startLocalDate.getMonth(), startLocalDate.getDate() + i);
				if (targetDayIndices.includes(checkDate.getDay())) {
					firstLocalDate = checkDate;
					found = true;
					break;
				}
			}

			const firstStart = new Date(
				firstLocalDate.getFullYear(),
				firstLocalDate.getMonth(),
				firstLocalDate.getDate(),
				startH,
				startM,
				0
			);
			const firstEnd = new Date(
				firstLocalDate.getFullYear(),
				firstLocalDate.getMonth(),
				firstLocalDate.getDate(),
				endH,
				endM,
				0
			);

			const firstStartISO = firstStart.toISOString();
			const firstEndISO = firstEnd.toISOString();

			// Build RFC 5545 RRULE (UNTIL formatted in UTC ISO)
			const [endY, endMonth, endD] = endDate.split("-").map(Number);
			const localUntil = new Date(endY, endMonth - 1, endD, 23, 59, 59);
			const untilStr = localUntil.toISOString().replace(/[-:]/g, "").replace(/\.\d{3}/, "");
			const byDaysStr = selectedDays.join(",");
			const rruleStr = `RRULE:FREQ=WEEKLY;BYDAY=${byDaysStr};UNTIL=${untilStr}`;

			const displayTitle = title.trim() ? `${courseName}: ${title.trim()}` : `${courseName} Class`;

			if (isEditing && editEvent) {
				await dataState.updateEvent(editEvent.id, {
					title: displayTitle,
					calendar: selectedCalendarId,
					start: firstStartISO,
					end: firstEndISO,
					allday: false,
					deadline: false,
					announcement: false,
					recurr: rruleStr,
					color: "",
					description: location.trim() ? `Location: ${location.trim()}` : "",
					source: "internal",
				});
			} else {
				await dataState.addEvent({
					title: displayTitle,
					calendar: selectedCalendarId,
					start: firstStartISO,
					end: firstEndISO,
					allday: false,
					deadline: false,
					announcement: false,
					recurr: rruleStr,
					color: "",
					description: location.trim() ? `Location: ${location.trim()}` : "",
					source: "internal",
					exdate: [],
				});
			}

			onClose();
		} catch (err: any) {
			console.error("Failed to save class schedule:", err);
			errorMessage = err?.message || "Failed to save recurring class event.";
		} finally {
			isSubmitting = false;
		}
	}
</script>

{#if open}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div
		use:portal
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 animate-in fade-in duration-150"
		role="dialog"
		aria-modal="true"
		onclick={(e) => {
			if (e.target === e.currentTarget) onClose();
		}}
		onkeydown={(e) => {
			if (e.key === "Escape") onClose();
		}}
		tabindex="-1"
	>
		<div
			class="relative w-full max-w-lg rounded-xl border bg-card text-card-foreground shadow-2xl overflow-hidden animate-in zoom-in-95 duration-150"
		>
			<!-- Header -->
			<div class="flex items-center justify-between border-b px-5 py-3.5 bg-muted/30">
				<div class="flex items-center gap-2 font-semibold text-sm">
					<BookOpen class="size-4 text-primary" />
					<span>{isEditing ? "Edit Class Schedule" : "Add Class Schedule"}</span>
				</div>
				<button
					type="button"
					onclick={onClose}
					class="rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors cursor-pointer"
					aria-label="Close"
				>
					<X class="size-4" />
				</button>
			</div>

			<!-- Form -->
			<form onsubmit={handleSubmit} class="p-5 space-y-4 text-xs">
				{#if errorMessage}
					<div class="rounded-md bg-destructive/10 border border-destructive/20 p-2.5 text-destructive text-xs">
						{errorMessage}
					</div>
				{/if}

				<!-- Course / Calendar Selection -->
				<div class="space-y-1.5">
					<label for="course-select" class="font-medium text-foreground">Course / Calendar</label>
					<select
						id="course-select"
						bind:value={selectedCalendarId}
						class="w-full rounded-md border bg-background px-3 py-2 text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-primary"
					>
						{#each courses as course}
							<option value={course.id}>
								{course.nickname || course.name}
							</option>
						{/each}
					</select>
				</div>

				<!-- Class Type / Subtitle -->
				<div class="space-y-1.5">
					<label for="class-title" class="font-medium text-foreground">Session Title / Type</label>
					<input
						id="class-title"
						type="text"
						bind:value={title}
						placeholder="e.g. Lecture, Discussion, Lab"
						class="w-full rounded-md border bg-background px-3 py-2 text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-primary"
					/>
				</div>

				<!-- Days of Week Selector -->
				<div class="space-y-1.5">
					<span class="font-medium text-foreground">Repeating Days</span>
					<div class="flex items-center gap-1.5">
						{#each dayOptions as day}
							{@const isSelected = selectedDays.includes(day.value)}
							<button
								type="button"
								onclick={() => toggleDay(day.value)}
								class="flex-1 py-1.5 rounded-md font-medium text-xs border transition-all cursor-pointer select-none text-center
									{isSelected
										? 'bg-primary text-primary-foreground border-primary shadow-xs'
										: 'bg-muted/40 text-muted-foreground border-border/70 hover:bg-muted/80'}"
								title={day.name}
							>
								{day.label}
							</button>
						{/each}
					</div>
				</div>

				<!-- Times -->
				<div class="grid grid-cols-2 gap-3">
					<div class="space-y-1.5">
						<label for="start-time" class="font-medium text-foreground flex items-center gap-1">
							<Clock class="size-3 text-muted-foreground" />
							<span>Start Time</span>
						</label>
						<input
							id="start-time"
							type="time"
							bind:value={startTime}
							class="w-full rounded-md border bg-background px-3 py-1.5 text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-primary"
						/>
					</div>
					<div class="space-y-1.5">
						<label for="end-time" class="font-medium text-foreground flex items-center gap-1">
							<Clock class="size-3 text-muted-foreground" />
							<span>End Time</span>
						</label>
						<input
							id="end-time"
							type="time"
							bind:value={endTime}
							class="w-full rounded-md border bg-background px-3 py-1.5 text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-primary"
						/>
					</div>
				</div>

				<!-- Semester / Term Bounds -->
				<div class="grid grid-cols-2 gap-3">
					<div class="space-y-1.5">
						<label for="start-date" class="font-medium text-foreground flex items-center gap-1">
							<CalendarIcon class="size-3 text-muted-foreground" />
							<span>Term Start Date</span>
						</label>
						<input
							id="start-date"
							type="date"
							bind:value={startDate}
							class="w-full rounded-md border bg-background px-3 py-1.5 text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-primary"
						/>
					</div>
					<div class="space-y-1.5">
						<label for="end-date" class="font-medium text-foreground flex items-center gap-1">
							<CalendarIcon class="size-3 text-muted-foreground" />
							<span>Term End Date</span>
						</label>
						<input
							id="end-date"
							type="date"
							bind:value={endDate}
							class="w-full rounded-md border bg-background px-3 py-1.5 text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-primary"
						/>
					</div>
				</div>

				<!-- Location -->
				<div class="space-y-1.5">
					<label for="class-location" class="font-medium text-foreground flex items-center gap-1">
						<MapPin class="size-3 text-muted-foreground" />
						<span>Location / Room (optional)</span>
					</label>
					<input
						id="class-location"
						type="text"
						bind:value={location}
						placeholder="e.g. Science Hall 102"
						class="w-full rounded-md border bg-background px-3 py-2 text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-primary"
					/>
				</div>

				<!-- Action Buttons -->
				<div class="flex items-center justify-end gap-2 pt-2 border-t">
					<button
						type="button"
						onclick={onClose}
						disabled={isSubmitting}
						class="px-3 py-1.5 rounded-md border bg-background hover:bg-muted text-foreground transition-colors cursor-pointer text-xs font-medium"
					>
						Cancel
					</button>
					<button
						type="submit"
						disabled={isSubmitting}
						class="px-4 py-1.5 rounded-md bg-primary hover:bg-primary/90 text-primary-foreground transition-colors cursor-pointer text-xs font-medium shadow-xs disabled:opacity-50"
					>
						{isSubmitting ? "Saving..." : isEditing ? "Save Changes" : "Add Class Time"}
					</button>
				</div>
			</form>
		</div>
	</div>
{/if}
