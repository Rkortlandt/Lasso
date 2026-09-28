<script lang="ts">
	import { pb, POCKETBASE_URL } from "$lib/pocketbase";
	import { authState } from "$lib/authState.svelte";
	import { syncController } from "$lib/syncController.svelte";
	import { Button } from "$lib/components/ui/button";
	import BookOpenIcon from "@lucide/svelte/icons/book-open";
	import CalendarIcon from "@lucide/svelte/icons/calendar";
	import RefreshCwIcon from "@lucide/svelte/icons/refresh-cw";
	import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
	import TagIcon from "@lucide/svelte/icons/tag";
	import CheckIcon from "@lucide/svelte/icons/check";
	import RotateCcwIcon from "@lucide/svelte/icons/rotate-ccw";
	import AlertTriangleIcon from "@lucide/svelte/icons/alert-triangle";
	import EyeIcon from "@lucide/svelte/icons/eye";
	import EyeOffIcon from "@lucide/svelte/icons/eye-off";
	import PaletteIcon from "@lucide/svelte/icons/palette";
	import ClockIcon from "@lucide/svelte/icons/clock";
	import PlusIcon from "@lucide/svelte/icons/plus";
	import Trash2Icon from "@lucide/svelte/icons/trash-2";
	import PencilIcon from "@lucide/svelte/icons/pencil";
	import MapPinIcon from "@lucide/svelte/icons/map-pin";
	import ClassScheduleModal from "$lib/components/calendar/ClassScheduleModal.svelte";
	import { portal } from "$lib/portal";
	import { calendarVisibilityState } from "$lib/calendarVisibilityState.svelte";
	import { fade, slide, scale } from "svelte/transition";
	import { dataState } from "$lib/dataState/dataState.svelte";
	import {
		getCourseworkCalendars,
		getCurrentCourseworkCalendars,
		getPreviousCourseworkCalendars,
		resolveCalendarColor,
	} from "$lib/dataState/calendarQueries.svelte";
	import {
		type CalendarRecord,
		type EventRecord,
	} from "$lib/dataState/dataRecordInterfaces";
	import { themeState } from "$lib/themeState.svelte";
	import {
		CANVAS_PRESET_PALETTE,
		generateThemeHarmonicPalette,
		getThemeBaseHue,
		oklchToHex,
		hexToOklch,
		type ColorSwatch,
	} from "$lib/colorUtils";

	let showConnectForm = $state(false);
	let showDisconnectModal = $state(false);
	let canvasUrl = $state(
		authState.record?.canvas_url || "https://canvas.instructure.com",
	);
	let canvasToken = $state("");
	let isLoading = $state(false);

	const isConnected = $derived(Boolean(authState.record?.canvas_connected));
	const courses = $derived(getCourseworkCalendars());
	const currentCourses = $derived(getCurrentCourseworkCalendars());
	const previousCourses = $derived(getPreviousCourseworkCalendars());
	let isPreviousCoursesExpanded = $state(false);

	// Harmonic Theme Palette derived from active theme accent
	const themeBaseHue = $derived(
		getThemeBaseHue(
			themeState.accentId,
			themeState.customHue,
			themeState.currentAccent.swatch,
		),
	);
	const harmonicPalette = $derived(generateThemeHarmonicPalette(themeBaseHue));

	// Section detail & nickname state
	let expandedCourseId = $state<string | null>(null);
	let nicknameInput = $state("");
	let isSavingNickname = $state(false);
	let nicknameSuccessId = $state<string | null>(null);

	// Course color customization state
	let customHues = $state<Record<string, number>>({});
	let customHexInputs = $state<Record<string, string>>({});
	let isCustomMode = $state<Record<string, boolean>>({});
	let colorSavingState = $state<Record<string, boolean>>({});
	let colorSuccessState = $state<Record<string, boolean>>({});
	let colorDebounceTimers = new Map<string, ReturnType<typeof setTimeout>>();

	function getCourseHue(course: CalendarRecord): number {
		if (customHues[course.id] !== undefined) return customHues[course.id];
		const oklch = hexToOklch(course.color || "#3b82f6");
		return oklch ? Math.round(oklch.h) : 180;
	}

	function isCourseCustom(course: CalendarRecord): boolean {
		if (isCustomMode[course.id] !== undefined) return isCustomMode[course.id];
		if (!course.color) return false;
		const hexLower = course.color.toLowerCase();
		const isPreset = CANVAS_PRESET_PALETTE.some(
			(p) => p.hex.toLowerCase() === hexLower,
		);
		const isHarmonic = harmonicPalette.allThemeSwatches.some(
			(p) => p.hex.toLowerCase() === hexLower,
		);
		return !isPreset && !isHarmonic;
	}

	function getCourseHexInput(course: CalendarRecord): string {
		if (customHexInputs[course.id] !== undefined)
			return customHexInputs[course.id];
		return course.color || "#3b82f6";
	}

	async function setCourseColor(
		course: CalendarRecord,
		hex: string,
		isFromCustom = false,
	) {
		let normalizedHex = hex.trim();
		if (!normalizedHex.startsWith("#")) {
			normalizedHex = "#" + normalizedHex;
		}
		normalizedHex = normalizedHex.toLowerCase();

		// Update local state optimistically
		await dataState.updateCalendar(course.id, { color: normalizedHex });

		if (isFromCustom) {
			isCustomMode[course.id] = true;
			const oklch = hexToOklch(normalizedHex);
			if (oklch) {
				customHues[course.id] = Math.round(oklch.h);
			}
			customHexInputs[course.id] = normalizedHex;
		} else {
			isCustomMode[course.id] = false;
			customHexInputs[course.id] = normalizedHex;
		}

		// Debounce backend sync to Canvas and PocketBase
		if (colorDebounceTimers.has(course.id)) {
			clearTimeout(colorDebounceTimers.get(course.id));
		}

		colorSavingState[course.id] = true;
		colorSuccessState[course.id] = false;

		const timer = setTimeout(async () => {
			try {
				const res = await fetch(`${POCKETBASE_URL}/api/canvas/color`, {
					method: "POST",
					headers: {
						"Content-Type": "application/json",
						Authorization: pb.authStore.token
							? `Bearer ${pb.authStore.token}`
							: "",
					},
					body: JSON.stringify({
						calendarId: course.id,
						courseId: course.course_id,
						hexcode: normalizedHex,
					}),
				});
				if (res.ok) {
					colorSuccessState[course.id] = true;
					setTimeout(() => {
						colorSuccessState[course.id] = false;
					}, 3000);
				}
			} catch (err) {
				console.error("Failed to sync color to Canvas:", err);
			} finally {
				colorSavingState[course.id] = false;
				colorDebounceTimers.delete(course.id);
			}
		}, 350);

		colorDebounceTimers.set(course.id, timer);
	}

	function handleCustomHueChange(course: CalendarRecord, hue: number) {
		customHues[course.id] = hue;
		isCustomMode[course.id] = true;
		const hex = oklchToHex(0.62, 0.22, hue);
		customHexInputs[course.id] = hex;
		setCourseColor(course, hex, true);
	}

	function handleCustomHexInput(course: CalendarRecord, val: string) {
		customHexInputs[course.id] = val;
		let clean = val.trim();
		if (!clean.startsWith("#")) clean = "#" + clean;
		if (/^#[0-9a-fA-F]{6}$/.test(clean) || /^#[0-9a-fA-F]{3}$/.test(clean)) {
			setCourseColor(course, clean, true);
		}
	}

	function formatCourseDate(dateStr?: string): string {
		if (!dateStr || !dateStr.trim()) return "—";
		const normalized = dateStr.includes("T")
			? dateStr
			: dateStr.replace(" ", "T");
		const d = new Date(normalized);
		if (isNaN(d.getTime())) return dateStr;
		return d.toLocaleDateString(undefined, {
			year: "numeric",
			month: "short",
			day: "numeric",
		});
	}

	function toggleExpand(course: CalendarRecord) {
		if (expandedCourseId === course.id) {
			expandedCourseId = null;
		} else {
			expandedCourseId = course.id;
			nicknameInput = course.nickname || "";
			nicknameSuccessId = null;
		}
	}

	async function saveNickname(e: SubmitEvent, calendarId: string) {
		e.preventDefault();
		isSavingNickname = true;
		try {
			const trimmedNickname = nicknameInput.trim();
			await dataState.updateCalendar(calendarId, { nickname: trimmedNickname });
			nicknameSuccessId = calendarId;
			setTimeout(() => {
				if (nicknameSuccessId === calendarId) nicknameSuccessId = null;
			}, 3000);
		} catch (err) {
			console.error("Failed to save nickname:", err);
		} finally {
			isSavingNickname = false;
		}
	}

	async function clearNickname(calendarId: string) {
		isSavingNickname = true;
		try {
			await dataState.updateCalendar(calendarId, { nickname: "" });
			nicknameInput = "";
			nicknameSuccessId = calendarId;
			setTimeout(() => {
				if (nicknameSuccessId === calendarId) nicknameSuccessId = null;
			}, 3000);
		} catch (err) {
			console.error("Failed to clear nickname:", err);
		} finally {
			isSavingNickname = false;
		}
	}

	// Class Schedule & Recurrence state
	let isScheduleModalOpen = $state(false);
	let modalCalendarId = $state("");
	let modalEditEvent = $state<EventRecord | null>(null);
	let isDeletingScheduleId = $state<string | null>(null);

	const recurringClassEvents = $derived.by(() => {
		return dataState.events.filter((e) =>
			Boolean(e.recurr && e.recurr.trim() !== ""),
		);
	});

	const eventsByCourseId = $derived.by(() => {
		const map = new Map<string, EventRecord[]>();
		for (const evt of recurringClassEvents) {
			const calId = evt.calendar || evt.expand?.calendar?.id || "";
			if (!calId) continue;
			let list = map.get(calId);
			if (!list) {
				list = [];
				map.set(calId, list);
			}
			list.push(evt);
		}
		return map;
	});

	function openAddSchedule(courseId: string) {
		modalCalendarId = courseId;
		modalEditEvent = null;
		isScheduleModalOpen = true;
	}

	function openEditSchedule(evt: EventRecord) {
		modalCalendarId = evt.calendar || evt.expand?.calendar?.id || "";
		modalEditEvent = evt;
		isScheduleModalOpen = true;
	}

	async function handleDeleteSchedule(evtId: string) {
		if (
			confirm(
				"Are you sure you want to delete this recurring class schedule? All occurrences will be removed.",
			)
		) {
			isDeletingScheduleId = evtId;
			try {
				await dataState.deleteEvent(evtId);
			} catch (err) {
				console.error("Failed to delete recurring event:", err);
			} finally {
				isDeletingScheduleId = null;
			}
		}
	}

	function parseRruleHuman(
		rrule: string,
		startIso: string,
		endIso?: string,
	): { days: string; time: string; until: string } {
		let days = "";
		let until = "";

		const dayMap: Record<string, string> = {
			MO: "Mon",
			TU: "Tue",
			WE: "Wed",
			TH: "Thu",
			FR: "Fri",
			SA: "Sat",
			SU: "Sun",
		};

		const parts = rrule.replace(/^RRULE:/i, "").split(";");
		for (const part of parts) {
			if (part.startsWith("BYDAY=")) {
				const dayCodes = part.replace("BYDAY=", "").split(",");
				days = dayCodes.map((c) => dayMap[c] || c).join(", ");
			}
			if (part.startsWith("UNTIL=")) {
				const rawUntil = part.replace("UNTIL=", "");
				if (rawUntil.length >= 8) {
					const y = rawUntil.slice(0, 4);
					const m = rawUntil.slice(4, 6);
					const d = rawUntil.slice(6, 8);
					until = `${m}/${d}/${y}`;
				}
			}
		}

		let timeStr = "";
		if (startIso) {
			const sD = new Date(
				startIso.includes(" ") ? startIso.replace(" ", "T") : startIso,
			);
			const sTime = sD.toLocaleTimeString([], {
				hour: "numeric",
				minute: "2-digit",
			});
			if (endIso) {
				const eD = new Date(
					endIso.includes(" ") ? endIso.replace(" ", "T") : endIso,
				);
				const eTime = eD.toLocaleTimeString([], {
					hour: "numeric",
					minute: "2-digit",
				});
				timeStr = `${sTime} - ${eTime}`;
			} else {
				timeStr = sTime;
			}
		}

		return {
			days: days || "Weekly",
			time: timeStr || "Scheduled",
			until: until ? `until ${until}` : "",
		};
	}

	async function handleConnect(e: SubmitEvent) {
		e.preventDefault();
		if (!canvasToken.trim()) return;

		let cleanUrl = canvasUrl.trim();
		if (!cleanUrl) {
			cleanUrl = "https://canvas.instructure.com";
		} else if (
			!cleanUrl.startsWith("http://") &&
			!cleanUrl.startsWith("https://")
		) {
			cleanUrl = "https://" + cleanUrl;
		}
		cleanUrl = cleanUrl.replace(/\/+$/, "");
		canvasUrl = cleanUrl;

		isLoading = true;
		try {
			if (pb.authStore.record?.id) {
				await pb.collection("users").update(pb.authStore.record.id, {
					canvas_url: cleanUrl,
					canvas_token: canvasToken.trim(),
				});
			}

			const res = await fetch(`${POCKETBASE_URL}/api/canvas/verify`, {
				method: "POST",
				headers: {
					"Content-Type": "application/json",
					Authorization: pb.authStore.token
						? `Bearer ${pb.authStore.token}`
						: "",
				},
				body: JSON.stringify({
					canvasUrl: cleanUrl,
					canvasToken: canvasToken.trim(),
				}),
			});
			const data = await res.json();
			if (!res.ok || !data.success) {
				throw new Error(
					data.message || data.error || "Failed to verify Canvas token",
				);
			}

			await pb.collection("users").authRefresh();
			await syncController.syncCanvas().catch(() => {});
			await dataState.refresh();
			showConnectForm = false;
			canvasToken = "";
		} catch (err: any) {
			console.error("Canvas connect error:", err);
		} finally {
			isLoading = false;
		}
	}

	async function handleDisconnect() {
		showDisconnectModal = false;
		try {
			await fetch(`${POCKETBASE_URL}/api/canvas/disconnect`, {
				method: "POST",
				headers: {
					Authorization: pb.authStore.token
						? `Bearer ${pb.authStore.token}`
						: "",
				},
			}).catch(() => {});

			if (pb.authStore.record?.id) {
				await pb.collection("users").update(pb.authStore.record.id, {
					canvas_url: "",
					canvas_token: "",
					canvas_connected: false,
					canvas_student_name: "",
				});
			}
			await pb.collection("users").authRefresh();
			await dataState.refresh();
		} catch (err) {
			console.error("Disconnect error:", err);
		}
	}

	let syncAlert = $state<{ type: "success" | "error"; message: string } | null>(
		null,
	);

	async function handleSyncCanvas() {
		syncAlert = null;
		try {
			const res = await syncController.syncCanvas();
			syncAlert = {
				type: "success",
				message:
					res?.message || `Successfully synced Canvas courses and assignments.`,
			};
			setTimeout(() => {
				syncAlert = null;
			}, 6000);
		} catch (err: any) {
			syncAlert = {
				type: "error",
				message:
					err?.message ||
					"Failed to sync Canvas. Please verify your Canvas access token and institution URL.",
			};
		}
	}
</script>

<!-- Canvas LMS integration status with grouped sections -->
<div class="rounded-xl border border-border bg-card p-6 shadow-xs space-y-3">
	<div>
		<div class="flex items-center justify-between">
			<h2
				class="text-base font-normal tracking-wide text-card-foreground leading-none"
			>
				Canvas LMS
			</h2>

			{#if !isConnected}
				<span
					class="inline-flex items-center rounded-full bg-muted px-2.5 py-1 text-xs font-medium text-muted-foreground leading-none"
				>
					Not Connected
				</span>
			{/if}
		</div>

		<p class="text-xs py-1 text-muted-foreground w-full">
			Track academic courses, section schedules, and assignment deadlines.
		</p>
	</div>
	{#if isConnected}
		<div class="mt-4 pt-4 border-t border-border flex flex-col gap-4">
			<div
				class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs bg-muted/40 p-3 rounded-lg border border-border/50"
			>
				<div class="space-y-1">
					<div>
						<span class="text-muted-foreground">Institution: </span>
						<span class="font-medium font-mono text-foreground"
							>{authState.record?.canvas_url ||
								"https://canvas.instructure.com"}</span
						>
					</div>
					{#if authState.record?.canvas_student_name}
						<div>
							<span class="text-muted-foreground">Student: </span>
							<span class="font-medium text-foreground"
								>{authState.record?.canvas_student_name}</span
							>
						</div>
					{/if}
					{#if dataState.tasks.length > 0}
						<div>
							<span class="text-muted-foreground">Synced Tasks: </span>
							<span class="font-medium text-foreground"
								>{dataState.tasks.length} tasks</span
							>
						</div>
					{/if}
				</div>
			</div>

			{#if syncAlert}
				<div
					transition:slide={{ duration: 180 }}
					class="p-3 rounded-lg text-xs flex items-start gap-2.5 {syncAlert.type ===
					'error'
						? 'bg-destructive/10 text-destructive border border-destructive/20'
						: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20'}"
				>
					{#if syncAlert.type === "error"}
						<AlertTriangleIcon class="size-4 shrink-0 mt-0.5" />
					{:else}
						<CheckIcon class="size-4 shrink-0 mt-0.5" />
					{/if}
					<div class="flex-1 min-w-0">
						<p class="font-medium">{syncAlert.message}</p>
						{#if syncAlert.type === "error"}
							<p class="text-[11px] opacity-80 mt-0.5">
								Please check your Canvas token and institution URL.
							</p>
						{/if}
					</div>
				</div>
			{/if}

			{#if currentCourses.length > 0 || previousCourses.length > 0}
				<div class="pt-2 space-y-4">
					{#if currentCourses.length > 0}
						<div>
							<!-- Filter Header -->
							<div
								class="flex flex-col sm:flex-row sm:items-center justify-between gap-2 mb-2"
							>
								<div class="flex items-center gap-2">
									<span class="text-xs text-foreground uppercase font-bold"
										>Course Sections ({currentCourses.length})</span
									>
								</div>
							</div>

							<!-- Course groups listing with interactive expand panels -->
							<div class="space-y-2">
								{#each currentCourses as course (course.id)}
									{@render courseItem(course)}
								{/each}
							</div>
						</div>
					{/if}

					<!-- Previous Courses Collapsible Section (hidden/collapsed by default) -->
					{#if previousCourses.length > 0}
						<div class="pt-2">
							<!-- Divider with close / toggle badge button like sidebar main dropdowns -->
							<div class="relative flex w-full my-1 items-center">
								<button
									type="button"
									onclick={() =>
										(isPreviousCoursesExpanded = !isPreviousCoursesExpanded)}
									class="shrink-0 cursor-pointer text-left pr-2"
								>
									<span
										class="text-xs text-foreground uppercase font-bold hover:text-primary transition-colors"
									>
										Previous Courses ({previousCourses.length})
									</span>
								</button>
								<div class="w-full h-full border-t border-border"></div>
								<div class="w-8 flex items-center justify-center shrink-0">
									<button
										type="button"
										onclick={() =>
											(isPreviousCoursesExpanded = !isPreviousCoursesExpanded)}
										class="relative flex items-center justify-center rounded-full border border-border bg-accent text-foreground hover:bg-accent/80 hover:text-primary transition-all cursor-pointer shadow-2xs h-4.5 px-2 gap-1"
										aria-label={isPreviousCoursesExpanded
											? "Close previous courses"
											: "Open previous courses"}
										title={isPreviousCoursesExpanded
											? "Close previous courses"
											: "Open previous courses"}
									>
										<ChevronDownIcon
											class="size-2.5 transition-transform duration-200 {isPreviousCoursesExpanded
												? 'rotate-180'
												: ''}"
										/>
									</button>
								</div>
							</div>

							{#if isPreviousCoursesExpanded}
								<div
									transition:slide={{ duration: 180 }}
									class="space-y-2 mt-2"
								>
									{#each previousCourses as course (course.id)}
										{@render courseItem(course)}
									{/each}
								</div>
							{/if}
						</div>
					{/if}
				</div>
			{/if}

			<div class="pt-2 flex justify-end">
				<Button
					variant="outline"
					size="sm"
					class="text-xs text-destructive hover:text-destructive hover:bg-destructive/10 border-border cursor-pointer"
					onclick={() => (showDisconnectModal = true)}
				>
					Disconnect Canvas
				</Button>
			</div>
		</div>
	{:else if showConnectForm}
		<form
			onsubmit={handleConnect}
			novalidate
			class="mt-4 pt-4 border-t border-border space-y-3"
			in:fade={{ duration: 150 }}
		>
			<div>
				<label
					for="settings-canvas-url"
					class="block text-xs font-medium text-muted-foreground mb-1"
				>
					Canvas URL
				</label>
				<input
					id="settings-canvas-url"
					type="text"
					inputmode="url"
					bind:value={canvasUrl}
					placeholder="canvas.instructure.com or school.instructure.com"
					class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-xs text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
					required
				/>
			</div>
			<div>
				<label
					for="settings-canvas-token"
					class="block text-xs font-medium text-muted-foreground mb-1"
				>
					Access Token
				</label>
				<input
					id="settings-canvas-token"
					type="password"
					bind:value={canvasToken}
					placeholder="Paste token"
					class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-xs text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
					required
				/>
			</div>
			<div class="flex items-center justify-between pt-1">
				<div class="flex gap-2">
					<Button
						type="button"
						variant="ghost"
						size="sm"
						class="text-xs cursor-pointer"
						onclick={() => (showConnectForm = false)}
					>
						Cancel
					</Button>
					<Button
						type="submit"
						size="sm"
						class="text-xs cursor-pointer"
						disabled={isLoading || !canvasToken.trim()}
					>
						{isLoading ? "Connecting..." : "Connect"}
					</Button>
				</div>
			</div>
		</form>
	{:else}
		<div
			class="mt-4 pt-4 border-t border-border flex items-center justify-between"
		>
			<p class="text-xs text-muted-foreground">No Canvas account linked yet.</p>
			<Button
				variant="outline"
				size="sm"
				class="text-xs cursor-pointer"
				onclick={() => (showConnectForm = true)}
			>
				Connect Canvas
			</Button>
		</div>
	{/if}
</div>

<!-- Confirmation Modal for Disconnecting Canvas -->
{#if showDisconnectModal}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div
		use:portal
		class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/65 backdrop-blur-xs"
		transition:fade={{ duration: 150 }}
		onclick={(e) => {
			if (e.target === e.currentTarget) showDisconnectModal = false;
		}}
		onkeydown={(e) => {
			if (e.key === "Escape") showDisconnectModal = false;
		}}
		tabindex="-1"
		role="alertdialog"
		aria-modal="true"
		aria-labelledby="disconnect-canvas-title"
		aria-describedby="disconnect-canvas-desc"
	>
		<div
			class="w-full max-w-md rounded-xl border border-border bg-card p-6 shadow-2xl space-y-4"
			transition:scale={{ start: 0.95, duration: 150 }}
		>
			<div class="flex items-start gap-3.5">
				<div
					class="size-10 rounded-full bg-destructive/15 flex items-center justify-center text-destructive shrink-0 mt-0.5"
				>
					<AlertTriangleIcon class="size-5" />
				</div>
				<div class="space-y-1">
					<h3
						id="disconnect-canvas-title"
						class="text-base font-semibold text-card-foreground leading-snug"
					>
						Disconnect Canvas LMS?
					</h3>
					<p
						id="disconnect-canvas-desc"
						class="text-xs text-muted-foreground leading-relaxed"
					>
						Are you sure you want to disconnect Canvas LMS? This will remove all
						synced academic courses, section schedules, and assignment dates
						from your Lasso calendar.
					</p>
				</div>
			</div>

			<div
				class="flex items-center justify-end gap-2.5 pt-2 border-t border-border/50"
			>
				<Button
					type="button"
					variant="outline"
					size="sm"
					class="text-xs cursor-pointer"
					onclick={() => (showDisconnectModal = false)}
				>
					Cancel
				</Button>
				<Button
					type="button"
					variant="destructive"
					size="sm"
					class="text-xs cursor-pointer gap-1.5"
					onclick={handleDisconnect}
				>
					Disconnect
				</Button>
			</div>
		</div>
	</div>
{/if}

<!-- Reusable snippet for course section card with interactive details and nickname editing -->
{#snippet courseItem(course: CalendarRecord)}
	{@const isExpanded = expandedCourseId === course.id}
	{@const courseColor = resolveCalendarColor(course, "#3b82f6")}
	{@const calId = course.id}
	{@const isHiddenInSidebar = calendarVisibilityState.isHiddenInSidebar(
		calId,
		course.name,
	)}
	<div
		class="rounded-lg border border-border bg-card/60 overflow-hidden transition-all duration-200 {isExpanded
			? 'border-primary/50 shadow-xs ring-1 ring-primary/20'
			: 'hover:border-border/80'}"
	>
		<!-- Clickable header card with right-aligned grey outlined visibility button -->
		<div class="w-full flex items-stretch select-none">
			<button
				type="button"
				class="flex items-center gap-2.5 min-w-0 flex-1 px-3.5 py-3 cursor-pointer text-left group"
				onclick={() => toggleExpand(course)}
				aria-expanded={isExpanded}
			>
				<span
					class="size-2.5 rounded-xs shrink-0 shadow-2xs transition-opacity {isHiddenInSidebar
						? 'opacity-40'
						: ''}"
					style="background-color: {courseColor};"
				></span>
				<span
					class="font-medium truncate text-xs leading-none {isHiddenInSidebar
						? 'text-muted-foreground line-through opacity-75'
						: 'text-foreground'}"
				>
					{course.nickname || course.name}
				</span>
			</button>

			<!-- Right controls: Grey outlined button with no text + expand chevron -->
			<div class="flex items-center gap-2 pr-3 shrink-0">
				<!-- Grey outlined visibility button with no text -->
				<button
					type="button"
					class="size-7 rounded-md border border-border/70 text-muted-foreground hover:text-foreground hover:border-foreground/40 hover:bg-muted/40 flex items-center justify-center transition-colors cursor-pointer {isHiddenInSidebar
						? 'opacity-50'
						: ''}"
					onclick={(e) => {
						e.stopPropagation();
						calendarVisibilityState.toggleSidebarVisibility(calId, course.name);
					}}
					title={isHiddenInSidebar
						? "Hidden in sidebar (click to show)"
						: "Visible in sidebar (click to hide)"}
					aria-label={isHiddenInSidebar ? "Show in sidebar" : "Hide in sidebar"}
				>
					{#if isHiddenInSidebar}
						<EyeOffIcon class="size-3.5" />
					{:else}
						<EyeIcon class="size-3.5" />
					{/if}
				</button>

				<button
					type="button"
					class="h-full flex items-center px-1 text-muted-foreground hover:text-foreground cursor-pointer transition-colors"
					onclick={() => toggleExpand(course)}
					aria-label="Toggle course details"
				>
					<ChevronDownIcon
						class="size-3.5 transition-transform duration-200 {isExpanded
							? 'rotate-180 text-foreground'
							: ''}"
					/>
				</button>
			</div>
		</div>

		<!-- Expanded detail panel: dates, IDs, and local nickname editor -->
		{#if isExpanded}
			{@const isCustomSelected = isCourseCustom(course)}
			{@const currentHue = getCourseHue(course)}
			{@const courseEvents = eventsByCourseId.get(course.id) || []}
			<div
				class="@container px-3 pb-3 pt-1 border-t border-border/50 bg-muted/20 space-y-3"
				transition:slide={{ duration: 180 }}
			>
				<!-- Grid of section metadata -->
				<div class="grid grid-cols-2 sm:grid-cols-3 gap-2.5 pt-1 text-[11px]">
					{#if course.course_id}
						<div class="flex flex-col gap-0.5">
							<span class="text-muted-foreground">Course ID</span>
							<span class="font-mono text-foreground font-medium">
								#{course.course_id}
							</span>
						</div>
					{/if}

					<div class="flex flex-col gap-0.5">
						<span class="text-muted-foreground">Calendar ID</span>
						<span
							class="font-mono text-foreground font-medium truncate"
							title={course.id}
						>
							{course.id}
						</span>
					</div>

					<div class="flex flex-col gap-0.5">
						<span class="text-muted-foreground">Original Name</span>
						<span
							class="font-mono text-foreground font-medium truncate"
							title={course.name}
						>
							{course.name}
						</span>
					</div>

					<div class="flex flex-col gap-0.5">
						<span class="text-muted-foreground">Start Date</span>
						<span
							class="font-mono text-foreground font-medium truncate"
							title={formatCourseDate(course.start_date)}
						>
							{formatCourseDate(course.start_date)}
						</span>
					</div>

					<div class="flex flex-col gap-0.5">
						<span class="text-muted-foreground">End Date</span>
						<span
							class="font-mono text-foreground font-medium truncate"
							title={formatCourseDate(course.end_date)}
						>
							{formatCourseDate(course.end_date)}
						</span>
					</div>
				</div>
				<!-- Local Nickname Editor -->
				<div class="pt-2 border-t border-border/40">
					<form onsubmit={(e) => saveNickname(e, course.id)} class="space-y-2">
						<div class="flex items-center justify-between">
							<label
								for="nickname-{course.id}"
								class="text-[11px] font-medium text-muted-foreground flex items-center gap-1"
							>
								<TagIcon class="size-3 text-primary" />
								Course Nickname
							</label>
							{#if nicknameSuccessId === course.id}
								<span
									class="text-[10px] text-emerald-400 font-medium inline-flex items-center gap-0.5"
									in:fade={{ duration: 150 }}
								>
									<CheckIcon class="size-3" /> Saved!
								</span>
							{/if}
						</div>

						<div class="flex gap-2">
							<input
								id="nickname-{course.id}"
								type="text"
								bind:value={nicknameInput}
								placeholder="e.g. Calc II, Physics Lab..."
								maxlength="59"
								class="flex-1 rounded-md border border-input bg-background px-2.5 py-1 text-xs text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
							/>
							<Button
								type="submit"
								size="sm"
								class="h-7 px-2.5 text-xs cursor-pointer gap-1"
								disabled={isSavingNickname || !nicknameInput.trim()}
							>
								{isSavingNickname ? "Saving..." : "Save Nickname"}
							</Button>
							{#if course.nickname}
								<Button
									type="button"
									variant="ghost"
									size="sm"
									class="h-7 px-2 text-xs text-muted-foreground hover:text-foreground cursor-pointer gap-1"
									onclick={() => clearNickname(course.id)}
									title="Reset to default course name"
									disabled={isSavingNickname}
								>
									<RotateCcwIcon class="size-3" />
									<span>Reset</span>
								</Button>
							{/if}
						</div>
						<p class="text-[10px] text-muted-foreground">
							Stored in your Lasso workspace. Customizes the course title on
							your calendar and sidebar.
						</p>
					</form>
				</div>

				<!-- Class Schedule & Recurrence -->
				<div class="pt-2 border-t border-border/40 space-y-2.5">
					<div class="flex items-center justify-between">
						<span
							class="text-[11px] font-medium text-muted-foreground flex items-center gap-1"
						>
							<ClockIcon class="size-3 text-primary" />
							Class Schedule
						</span>
						<button
							type="button"
							onclick={() => openAddSchedule(course.id)}
							class="flex items-center gap-1 text-[11px] font-medium text-primary hover:text-primary/80 transition-colors cursor-pointer px-1.5 py-0.5 rounded hover:bg-primary/10"
							title="Add recurring class time"
						>
							<PlusIcon class="size-3" />
							<span>Add Class Time</span>
						</button>
					</div>

					{#if courseEvents.length === 0}
						<p class="text-[11px] text-muted-foreground/80 italic">
							No weekly class times set for this course yet.
						</p>
					{:else}
						<div class="space-y-1.5">
							{#each courseEvents as evt (evt.id)}
								{@const parsed = parseRruleHuman(
									evt.recurr || "",
									evt.start,
									evt.end,
								)}
								<div
									class="flex items-center justify-between p-2.5 rounded-lg border bg-background/50 hover:bg-background transition-colors text-xs"
								>
									<div class="space-y-0.5 min-w-0">
										<div class="flex items-center gap-2">
											<span class="font-medium text-foreground"
												>{evt.title}</span
											>
											<span
												class="text-[10px] font-semibold px-1.5 py-0.5 rounded bg-primary/15 text-primary"
											>
												{parsed.days}
											</span>
										</div>
										<div
											class="flex items-center gap-3 text-[11px] text-muted-foreground"
										>
											<span class="flex items-center gap-1">
												<ClockIcon class="size-3 shrink-0" />
												<span>{parsed.time}</span>
											</span>
											{#if parsed.until}
												<span class="flex items-center gap-1">
													<CalendarIcon class="size-3 shrink-0" />
													<span>{parsed.until}</span>
												</span>
											{/if}
											{#if evt.description}
												<span
													class="flex items-center gap-1 truncate max-w-[150px]"
												>
													<MapPinIcon class="size-3 shrink-0" />
													<span class="truncate"
														>{evt.description.replace(
															/^Location:\s*/i,
															"",
														)}</span
													>
												</span>
											{/if}
										</div>
									</div>

									<div class="flex items-center gap-1 shrink-0 ml-2">
										<button
											type="button"
											onclick={() => openEditSchedule(evt)}
											class="p-1.5 rounded-md text-muted-foreground/70 hover:text-foreground hover:bg-muted transition-colors cursor-pointer"
											title="Edit class schedule"
											aria-label="Edit class schedule"
										>
											<PencilIcon class="size-3.5" />
										</button>
										<button
											type="button"
											onclick={() => handleDeleteSchedule(evt.id)}
											disabled={isDeletingScheduleId === evt.id}
											class="p-1.5 rounded-md text-muted-foreground/70 hover:text-destructive hover:bg-destructive/10 transition-colors cursor-pointer"
											title="Delete recurring schedule"
											aria-label="Delete recurring schedule"
										>
											<Trash2Icon class="size-3.5" />
										</button>
									</div>
								</div>
							{/each}
						</div>
					{/if}
					<!-- Course Color Editor & Canvas Sync -->
					<div class="pt-2 border-t border-border/40 space-y-2.5">
						<div class="flex items-center justify-between">
							<label
								for="course-color-{course.id}"
								class="text-[11px] font-medium text-muted-foreground flex items-center gap-1"
							>
								<PaletteIcon class="size-3 text-primary" />
								Course Color
							</label>
							{#if colorSavingState[course.id]}
								<span
									class="text-[10px] text-muted-foreground animate-pulse inline-flex items-center gap-1"
									in:fade={{ duration: 150 }}
								>
									<RefreshCwIcon class="size-2.5 animate-spin" /> Syncing...
								</span>
							{:else if colorSuccessState[course.id]}
								<span
									class="text-[10px] text-emerald-400 font-medium inline-flex items-center gap-0.5"
									in:fade={{ duration: 150 }}
								>
									<CheckIcon class="size-3" /> Synced with Canvas!
								</span>
							{/if}
						</div>

						<!-- Section 1: Theme Harmonic Colors (All 3 horizontal if room, all 3 vertical if not) -->
						<div class="grid grid-cols-1 harmonic-groups-grid gap-3">
							<!-- 1. Accent & Tints -->
							<div class="space-y-1">
								<div
									class="text-[10px] font-medium text-muted-foreground whitespace-nowrap"
								>
									Accent Tints & Shades
								</div>
								<div class="flex items-center gap-1.5 flex-wrap">
									{#each harmonicPalette.accentGroup as swatch (swatch.id)}
										{@const isSelected =
											!isCourseCustom(course) &&
											(course.color?.toLowerCase() ===
												swatch.hex.toLowerCase() ||
												courseColor.toLowerCase() === swatch.hex.toLowerCase())}
										<button
											type="button"
											class="relative size-5 rounded-full transition-all duration-150 cursor-pointer flex items-center justify-center {isSelected
												? 'scale-115 ring-2 ring-foreground/60 ring-offset-2 ring-offset-background'
												: 'hover:scale-110 opacity-85 hover:opacity-100'}"
											style="background-color: {swatch.hex};"
											onclick={() => setCourseColor(course, swatch.hex, false)}
											title="{swatch.name} ({swatch.hex})"
											aria-label="Select {swatch.name} color"
										>
											{#if isSelected}
												<span class="size-1.5 rounded-full bg-white shadow-xs"
												></span>
											{/if}
										</button>
									{/each}
								</div>
							</div>

							<!-- 2. Complementary Tints & Shades -->
							<div class="space-y-1">
								<div
									class="text-[10px] font-medium text-muted-foreground whitespace-nowrap"
								>
									Complementary Tints & Shades
								</div>
								<div class="flex items-center gap-1.5 flex-wrap">
									{#each harmonicPalette.complementGroup as swatch (swatch.id)}
										{@const isSelected =
											!isCourseCustom(course) &&
											(course.color?.toLowerCase() ===
												swatch.hex.toLowerCase() ||
												courseColor.toLowerCase() === swatch.hex.toLowerCase())}
										<button
											type="button"
											class="relative size-5 rounded-full transition-all duration-150 cursor-pointer flex items-center justify-center {isSelected
												? 'scale-115 ring-2 ring-foreground/60 ring-offset-2 ring-offset-background'
												: 'hover:scale-110 opacity-85 hover:opacity-100'}"
											style="background-color: {swatch.hex};"
											onclick={() => setCourseColor(course, swatch.hex, false)}
											title="{swatch.name} ({swatch.hex})"
											aria-label="Select {swatch.name} color"
										>
											{#if isSelected}
												<span class="size-1.5 rounded-full bg-white shadow-xs"
												></span>
											{/if}
										</button>
									{/each}
								</div>
							</div>

							<!-- 3. Adjacent / Split-Complementary -->
							<div class="space-y-1">
								<div
									class="text-[10px] font-medium text-muted-foreground whitespace-nowrap"
								>
									Adjacent & Split-Complementary
								</div>
								<div class="flex items-center gap-1.5 flex-wrap">
									{#each harmonicPalette.splitGroup as swatch (swatch.id)}
										{@const isSelected =
											!isCourseCustom(course) &&
											(course.color?.toLowerCase() ===
												swatch.hex.toLowerCase() ||
												courseColor.toLowerCase() === swatch.hex.toLowerCase())}
										<button
											type="button"
											class="relative size-5 rounded-full transition-all duration-150 cursor-pointer flex items-center justify-center {isSelected
												? 'scale-115 ring-2 ring-foreground/60 ring-offset-2 ring-offset-background'
												: 'hover:scale-110 opacity-85 hover:opacity-100'}"
											style="background-color: {swatch.hex};"
											onclick={() => setCourseColor(course, swatch.hex, false)}
											title="{swatch.name} ({swatch.hex})"
											aria-label="Select {swatch.name} color"
										>
											{#if isSelected}
												<span class="size-1.5 rounded-full bg-white shadow-xs"
												></span>
											{/if}
										</button>
									{/each}
								</div>
							</div>
						</div>

						<!-- Section 2: Full Rainbow Spectrum + Custom OKLCH Button -->
						<div class="space-y-1">
							<div class="text-[10px] font-medium text-foreground">
								Rainbow Spectrum & Custom
							</div>
							<div class="flex items-center gap-1.5 flex-wrap pt-0.5">
								{#each CANVAS_PRESET_PALETTE as swatch (swatch.id)}
									{@const isSelected =
										!isCourseCustom(course) &&
										(course.color?.toLowerCase() === swatch.hex.toLowerCase() ||
											courseColor.toLowerCase() === swatch.hex.toLowerCase())}
									<button
										type="button"
										class="relative size-5 rounded-full transition-all duration-150 cursor-pointer flex items-center justify-center {isSelected
											? 'scale-115 ring-2 ring-foreground/60 ring-offset-2 ring-offset-background'
											: 'hover:scale-110 opacity-85 hover:opacity-100'}"
										style="background-color: {swatch.hex};"
										onclick={() => setCourseColor(course, swatch.hex, false)}
										title="{swatch.name} ({swatch.hex})"
										aria-label="Select {swatch.name} color"
									>
										{#if isSelected}
											<span class="size-1.5 rounded-full bg-white shadow-xs"
											></span>
										{/if}
									</button>
								{/each}

								<!-- Custom OKLCH Swatch Button -->
								<button
									type="button"
									class="relative size-5 rounded-full transition-all duration-150 cursor-pointer flex items-center justify-center {isCustomSelected
										? 'scale-115 ring-2 ring-foreground/60 ring-offset-2 ring-offset-background'
										: 'hover:scale-110 opacity-85 hover:opacity-100'}"
									style="background: {isCustomSelected
										? `oklch(0.62 0.22 ${currentHue})`
										: 'conic-gradient(from 90deg, #f43f5e, #f59e0b, #10b981, #06b6d4, #3b82f6, #6366f1, #a855f7, #f43f5e)'};"
									onclick={() => {
										isCustomMode[course.id] = true;
										handleCustomHueChange(course, currentHue);
									}}
									title="Custom OKLCH color"
									aria-label="Select custom color"
								>
									{#if isCustomSelected}
										<span class="size-1.5 rounded-full bg-white shadow-xs"
										></span>
									{/if}
								</button>
							</div>
						</div>

						<!-- Section 3: Custom OKLCH Hue Spectrum & Hex Input (shown when Custom is active) -->
						{#if isCourseCustom(course)}
							<div
								transition:slide={{ duration: 180 }}
								class="flex flex-col sm:flex-row sm:items-center justify-between gap-2.5 bg-muted/40 p-2.5 rounded-lg border border-border/40 mt-1.5"
							>
								<div class="flex items-center gap-2 min-w-0">
									<span
										class="size-3.5 rounded-full shrink-0 shadow-xs border border-white/20"
										style="background-color: oklch(0.62 0.22 {currentHue});"
									></span>
									<span class="text-[11px] font-medium text-foreground">
										OKLCH Hue
									</span>
									<span class="text-[10px] font-mono text-muted-foreground">
										{currentHue}°
									</span>
								</div>

								<div class="flex-1 flex items-center gap-2">
									<input
										type="range"
										min="0"
										max="360"
										step="1"
										value={currentHue}
										oninput={(e) =>
											handleCustomHueChange(
												course,
												Number((e.target as HTMLInputElement).value),
											)}
										class="w-full h-2.5 rounded-full appearance-none cursor-pointer custom-hue-slider shadow-xs"
										aria-label="Course color hue slider"
									/>
									<input
										type="text"
										id="course-color-{course.id}"
										value={getCourseHexInput(course)}
										oninput={(e) =>
											handleCustomHexInput(
												course,
												(e.target as HTMLInputElement).value,
											)}
										placeholder="#3b82f6"
										maxlength="7"
										class="w-18 font-mono text-[11px] rounded border border-input bg-background px-1.5 py-0.5 text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring text-center"
										aria-label="Custom color hex code"
									/>
								</div>
							</div>
						{/if}

						<p class="text-[10px] text-muted-foreground">
							Sets the course color in Lasso and syncs the custom color to
							Canvas LMS.
						</p>
					</div>
				</div>
			</div>
		{/if}
	</div>
{/snippet}

<ClassScheduleModal
	bind:open={isScheduleModalOpen}
	initialCalendarId={modalCalendarId}
	editEvent={modalEditEvent}
	onClose={() => {
		isScheduleModalOpen = false;
		modalEditEvent = null;
	}}
/>

<style>
	h2 {
		margin: 0px;
	}

	.custom-hue-slider {
		background: linear-gradient(
			to right,
			oklch(0.65 0.22 0),
			oklch(0.65 0.22 60),
			oklch(0.65 0.22 120),
			oklch(0.65 0.22 180),
			oklch(0.65 0.22 240),
			oklch(0.65 0.22 300),
			oklch(0.65 0.22 360)
		);
		outline: none;
	}

	.custom-hue-slider::-webkit-slider-thumb {
		appearance: none;
		width: 14px;
		height: 14px;
		border-radius: 50%;
		background: #ffffff;
		border: 2px solid rgba(0, 0, 0, 0.35);
		box-shadow: 0 1px 3px rgba(0, 0, 0, 0.35);
		cursor: pointer;
		transition: transform 0.1s ease;
	}

	.custom-hue-slider::-webkit-slider-thumb:hover {
		transform: scale(1.15);
	}

	.custom-hue-slider::-moz-range-thumb {
		width: 14px;
		height: 14px;
		border-radius: 50%;
		background: #ffffff;
		border: 2px solid rgba(0, 0, 0, 0.35);
		box-shadow: 0 1px 3px rgba(0, 0, 0, 0.35);
		cursor: pointer;
		transition: transform 0.1s ease;
	}

	.custom-hue-slider::-moz-range-thumb:hover {
		transform: scale(1.15);
	}

	@container (min-width: 500px) {
		.harmonic-groups-grid {
			grid-template-columns: repeat(3, minmax(0, 1fr));
		}
	}
</style>
