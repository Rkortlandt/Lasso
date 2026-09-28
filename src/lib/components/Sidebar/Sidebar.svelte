<script lang="ts">
	import * as Avatar from "$lib/components/ui/avatar";
	import { Button } from "$lib/components/ui/button";
	import { Calendar } from "$lib/components/ui/calendar";
	import { pageState } from "$lib/pageSystem.svelte";
	import { authState } from "$lib/authState.svelte";
	import {
		getLocalTimeZone,
		isToday,
	} from "@internationalized/date";
	import SettingsIcon from "@lucide/svelte/icons/settings";
	import CalendarIcon from "@lucide/svelte/icons/calendar";
	import UserIcon from "@lucide/svelte/icons/user";
	import RefreshCwIcon from "@lucide/svelte/icons/refresh-cw";
	import { dayState } from "$lib/dayState.svelte";
	import PanelLeftClose from "@lucide/svelte/icons/panel-left-close";
	import PanelLeftOpen from "@lucide/svelte/icons/panel-left-open";
	import SidebarTodoList from "./SidebarTodoList.svelte";
	import SidebarDropdownSection from "./SidebarDropdownSection.svelte";
	import SidebarCalendarItem from "./SidebarCalendarItem.svelte";
	import SidebarSyncIndicator from "./SidebarSyncIndicator.svelte";
	import SidebarMiniCalendar from "./SidebarMiniCalendar.svelte";
	import { calendarVisibilityState } from "$lib/calendarVisibilityState.svelte";
	import { fly, fade, slide } from "svelte/transition";
	import { cubicOut } from "svelte/easing";

	const SIDEBAR_COLLAPSED_STORAGE_KEY = "lasso_sidebar_collapsed";
	const CALENDAR_STORAGE_KEY = "lasso_sidebar_calendar_open";
	const TODO_STORAGE_KEY = "lasso_sidebar_todo_open";
	const TASKS_STORAGE_KEY = "lasso_sidebar_tasks_open";
	const GOOGLE_CALENDARS_STORAGE_KEY = "lasso_sidebar_google_calendars_open";
	const OPEN_COURSES_STORAGE_KEY = "lasso_sidebar_open_courses";

	import { dataState } from "$lib/dataState/dataState.svelte";
	import {
		getTasksByCourse,
		sortTasksChronological,
		sortTasksCompleted,
		type TaskGroup,
	} from "$lib/dataState/taskQueries.svelte";
	import {
		resolveCalendarColor,
		getCourseworkCalendars,
		getGoogleCalendars,
		getTodoCalendar,
		ensureTodoCalendar,
	} from "$lib/dataState/calendarQueries.svelte";
	import {
		type CalendarRecord,
		type TaskRecord,
	} from "$lib/dataState/dataRecordInterfaces";

	type PocketBaseCalendar = CalendarRecord;
	type PocketBaseTask = TaskRecord;

	let isSidebarCollapsed = $state(
		typeof window !== "undefined"
			? localStorage.getItem(SIDEBAR_COLLAPSED_STORAGE_KEY) === "true"
			: false,
	);

	let isVerticalSquished = $state(
		typeof window !== "undefined"
			? localStorage.getItem(SIDEBAR_COLLAPSED_STORAGE_KEY) === "true"
			: false,
	);

	let showCollapsedContent = $state(
		typeof window !== "undefined"
			? localStorage.getItem(SIDEBAR_COLLAPSED_STORAGE_KEY) === "true"
			: false,
	);

	let asideElement = $state<HTMLElement | null>(null);
	let scrollElement = $state<HTMLDivElement | null>(null);
	let scrollDiff = $state(0);
	let isSidebarHovered = $state(false);
	let isScrolling = $state(false);
	let scrollFadeTimeout: any = null;
	let collapseTimeout: any = null;
	let rafId: number | null = null;

	function checkScrollbar() {
		if (!scrollElement) return;
		const diff = Math.max(
			0,
			scrollElement.offsetWidth - scrollElement.clientWidth,
		);
		if (diff !== scrollDiff) {
			scrollDiff = diff;
		}
	}

	$effect(() => {
		if (!scrollElement) return;
		const ro = new ResizeObserver(() => {
			checkScrollbar();
		});
		ro.observe(scrollElement);
		checkScrollbar();
		return () => ro.disconnect();
	});

	const MIDPOINT_WIDTH = 148; // Exact halfway point between 56px (w-14) and 240px (w-60)

	function stopWidthMonitoring() {
		if (rafId !== null) {
			cancelAnimationFrame(rafId);
			rafId = null;
		}
	}

	function startWidthMonitoring() {
		stopWidthMonitoring();
		function check() {
			if (!asideElement) return;
			const currentWidth = asideElement.offsetWidth;
			if (isSidebarCollapsed) {
				// Collapsing: when width drops below midpoint (148px), swap to small variety
				if (currentWidth <= MIDPOINT_WIDTH) {
					showCollapsedContent = true;
				}
				if (currentWidth <= 57) {
					showCollapsedContent = true;
					rafId = null;
					return;
				}
			} else {
				// Expanding: when width expands above midpoint (148px), swap to full variety
				if (currentWidth >= MIDPOINT_WIDTH) {
					showCollapsedContent = false;
				}
				if (currentWidth >= 239) {
					showCollapsedContent = false;
					rafId = null;
					return;
				}
			}
			rafId = requestAnimationFrame(check);
		}
		rafId = requestAnimationFrame(check);
	}
	function getDevToolsMultiplier(): number {
		const anims =
			typeof document !== "undefined" ? document.getAnimations() : [];
		let minRate = 1;
		for (const a of anims) {
			if (
				typeof a.playbackRate === "number" &&
				a.playbackRate > 0 &&
				a.playbackRate < minRate
			) {
				minRate = a.playbackRate;
			}
		}
		return 1 / minRate;
	}

	function squishIn(
		node: HTMLElement,
		{
			duration = 200,
			easing = cubicOut,
			y = 20,
		}: { duration?: number; easing?: (t: number) => number; y?: number } = {},
	) {
		const style = getComputedStyle(node);
		const opacity = +style.opacity;
		const height = parseFloat(style.height);
		const paddingTop = parseFloat(style.paddingTop);
		const paddingBottom = parseFloat(style.paddingBottom);
		const marginTop = parseFloat(style.marginTop);
		const marginBottom = parseFloat(style.marginBottom);

		return {
			duration,
			easing,
			css: (t: number, u: number) => `
				overflow: hidden;
				opacity: ${t * opacity};
				height: ${t * height}px;
				padding-top: ${t * paddingTop}px;
				padding-bottom: ${t * paddingBottom}px;
				margin-top: ${t * marginTop}px;
				margin-bottom: ${t * marginBottom}px;
				transform: translateY(${u * y}px);
			`,
		};
	}

	function squishOut(
		node: HTMLElement,
		{
			duration = 200,
			easing = cubicOut,
			y = 20,
		}: { duration?: number; easing?: (t: number) => number; y?: number } = {},
	) {
		const style = getComputedStyle(node);
		const opacity = +style.opacity;
		const height = parseFloat(style.height);
		const paddingTop = parseFloat(style.paddingTop);
		const paddingBottom = parseFloat(style.paddingBottom);
		const marginTop = parseFloat(style.marginTop);
		const marginBottom = parseFloat(style.marginBottom);

		return {
			duration,
			easing,
			css: (t: number, u: number) => `
				overflow: hidden;
				opacity: ${t * opacity};
				height: ${t * height}px;
				padding-top: ${t * paddingTop}px;
				padding-bottom: ${t * paddingBottom}px;
				margin-top: ${t * marginTop}px;
				margin-bottom: ${t * marginBottom}px;
				transform: translateY(${-u * y}px);
				clip-path: inset(${u * y}px 0 0 0);
			`,
		};
	}

	function clearSidebarTimeouts() {
		if (collapseTimeout) {
			clearTimeout(collapseTimeout);
			collapseTimeout = null;
		}
		stopWidthMonitoring();
	}

	function handleVerticalSquishOutroEnd() {
		if (isVerticalSquished && !isSidebarCollapsed) {
			if (collapseTimeout) {
				clearTimeout(collapseTimeout);
				collapseTimeout = null;
			}
			isSidebarCollapsed = true;
			if (typeof window !== "undefined") {
				localStorage.setItem(SIDEBAR_COLLAPSED_STORAGE_KEY, "true");
			}
			startWidthMonitoring();
		}
	}

	function handleAsideTransitionEnd(e: TransitionEvent) {
		if (e.target !== asideElement || e.propertyName !== "width") return;
		if (collapseTimeout) {
			clearTimeout(collapseTimeout);
			collapseTimeout = null;
		}
		if (!isSidebarCollapsed) {
			// Width expansion to w-60 (240px) completed -> trigger Phase 2: slide down calendar/todo
			showCollapsedContent = false;
			isVerticalSquished = false;
		} else {
			showCollapsedContent = true;
		}
		stopWidthMonitoring();
	}

	function collapseSidebar() {
		clearSidebarTimeouts();
		isVerticalSquished = true;
		// Safety fallback timer: only fires if onoutroend does not fire (e.g. reduced-motion)
		// We set a generous 1000ms fallback, or scale with DevTools if an animation is already running.
		const anims =
			typeof document !== "undefined" ? document.getAnimations() : [];
		let minRate = 1;
		for (const a of anims) {
			if (
				typeof a.playbackRate === "number" &&
				a.playbackRate > 0 &&
				a.playbackRate < minRate
			) {
				minRate = a.playbackRate;
			}
		}
		const fallbackDuration = Math.max(1000, Math.round(1000 / minRate));
		collapseTimeout = setTimeout(() => {
			if (!isSidebarCollapsed) {
				isSidebarCollapsed = true;
				if (typeof window !== "undefined") {
					localStorage.setItem(SIDEBAR_COLLAPSED_STORAGE_KEY, "true");
				}
				startWidthMonitoring();
			}
		}, fallbackDuration);
	}

	function expandSidebar() {
		clearSidebarTimeouts();
		isSidebarCollapsed = false;
		if (typeof window !== "undefined") {
			localStorage.setItem(SIDEBAR_COLLAPSED_STORAGE_KEY, "false");
		}
		startWidthMonitoring();
		const anims =
			typeof document !== "undefined" ? document.getAnimations() : [];
		let minRate = 1;
		for (const a of anims) {
			if (
				typeof a.playbackRate === "number" &&
				a.playbackRate > 0 &&
				a.playbackRate < minRate
			) {
				minRate = a.playbackRate;
			}
		}
		const fallbackDuration = Math.max(1000, Math.round(1000 / minRate));
		// Safety fallback timer if ontransitionend does not fire
		collapseTimeout = setTimeout(() => {
			showCollapsedContent = false;
			isVerticalSquished = false;
		}, fallbackDuration);
	}

	function toggleSidebar() {
		if (isSidebarCollapsed) {
			expandSidebar();
		} else {
			collapseSidebar();
		}
	}

	function expandAndOpenCourse(calId: string) {
		clearSidebarTimeouts();
		isSidebarCollapsed = false;
		showCollapsedContent = false;
		if (typeof window !== "undefined") {
			localStorage.setItem(SIDEBAR_COLLAPSED_STORAGE_KEY, "false");
		}
		isTasksOpen = true;
		if (typeof window !== "undefined") {
			localStorage.setItem(TASKS_STORAGE_KEY, "true");
		}
		openCourses = {
			...openCourses,
			[calId]: true,
		};
		try {
			localStorage.setItem(
				OPEN_COURSES_STORAGE_KEY,
				JSON.stringify(openCourses),
			);
		} catch (e) {}
		const mult = getDevToolsMultiplier();
		collapseTimeout = setTimeout(() => {
			isVerticalSquished = false;
		}, 360 * mult);
	}

	function expandAndOpenGoogleCalendar() {
		clearSidebarTimeouts();
		isSidebarCollapsed = false;
		showCollapsedContent = false;
		if (typeof window !== "undefined") {
			localStorage.setItem(SIDEBAR_COLLAPSED_STORAGE_KEY, "false");
		}
		isGoogleCalendarsOpen = true;
		if (typeof window !== "undefined") {
			localStorage.setItem(GOOGLE_CALENDARS_STORAGE_KEY, "true");
		}
		const mult = getDevToolsMultiplier();
		collapseTimeout = setTimeout(() => {
			isVerticalSquished = false;
		}, 360 * mult);
	}

	$effect(() => {
		const handleDocLeave = () => {
			isSidebarHovered = false;
			isScrolling = false;
		};
		const handleBlur = () => {
			isSidebarHovered = false;
			isScrolling = false;
		};
		document.addEventListener("mouseleave", handleDocLeave);
		window.addEventListener("blur", handleBlur);
		return () => {
			document.removeEventListener("mouseleave", handleDocLeave);
			window.removeEventListener("blur", handleBlur);
			if (scrollFadeTimeout) clearTimeout(scrollFadeTimeout);
			clearSidebarTimeouts();
		};
	});

	let isTasksOpen = $state(
		typeof window !== "undefined"
			? localStorage.getItem(TASKS_STORAGE_KEY) !== "false"
			: true,
	);

	let isGoogleCalendarsOpen = $state(
		typeof window !== "undefined"
			? localStorage.getItem(GOOGLE_CALENDARS_STORAGE_KEY) !== "false"
			: true,
	);

	let openCourses = $state<Record<string, boolean>>(loadOpenCourses());
	let isScrolled = $state(false);
	let settingsRotation = $state(0);

	function handleSettingsClick() {
		settingsRotation += 360;

		if (pageState.current === "settings") {
			pageState.goBack();
		} else {
			pageState.setPage("settings");
		}
	}

	$effect(() => {
		if (!dataState.loading && dataState.isBackendReachable && authState.record?.id && !getTodoCalendar()) {
			ensureTodoCalendar();
		}
	});

	function handleScroll(e: UIEvent & { currentTarget: HTMLDivElement }) {
		isScrolled = e.currentTarget.scrollTop > 2;
		isScrolling = true;
		checkScrollbar();
		if (scrollFadeTimeout) clearTimeout(scrollFadeTimeout);
		scrollFadeTimeout = setTimeout(() => {
			isScrolling = false;
		}, 800);
	}

	function loadOpenCourses(): Record<string, boolean> {
		if (typeof window === "undefined") return {};
		try {
			localStorage.removeItem("lasso_sidebar_collapsed_calendars");
			const saved = localStorage.getItem(OPEN_COURSES_STORAGE_KEY);
			return saved ? JSON.parse(saved) : {};
		} catch {
			return {};
		}
	}

	// Group tasks by calendar with tasks sorted chronologically by due_date
	const canvasCalendars = $derived(getTasksByCourse());

	const visibleCanvasCalendars = $derived.by(() => {
		return canvasCalendars.filter(
			(g) => !calendarVisibilityState.isHiddenInSidebar(g.calendar.id),
		);
	});

	const googleCalendars = $derived(getGoogleCalendars());

	const visibleGoogleCalendars = $derived.by(() => {
		if (!authState.record?.google_connected) return [];
		return googleCalendars.filter(
			(cal) =>
				!calendarVisibilityState.isHiddenInSidebar(
					cal.id,
					cal.name,
				),
		);
	});
</script>

<aside
	bind:this={asideElement}
	ontransitionend={handleAsideTransitionEnd}
	onpointerenter={() => {
		isSidebarHovered = true;
	}}
	onpointerleave={() => {
		isSidebarHovered = false;
	}}
	class="flex h-screen {isSidebarCollapsed
		? 'w-14'
		: 'w-60'} transition-[width] duration-300 ease-in-out flex-col border-r border-border bg-sidebar text-sidebar-foreground shrink-0 select-none relative overflow-x-hidden"
>
	<!-- Top header with branding and sidebar collapse button -->
	<div
		class="relative flex h-14 items-center border-b border-sidebar-border z-10 bg-sidebar shrink-0 overflow-hidden flex-row-reverse"
	>
		<div class="w-14 h-14 flex items-center justify-center">
			<Button
				variant="ghost"
				size="icon"
				class="size-8 text-sidebar-foreground hover:bg-primary/10 hover:text-primary cursor-pointer shrink-0 transition-colors"
				onclick={toggleSidebar}
				aria-label={isSidebarCollapsed ? "Expand sidebar" : "Collapse sidebar"}
				title={isSidebarCollapsed ? "Expand sidebar" : "Collapse sidebar"}
			>
				{#if isSidebarCollapsed}
					<PanelLeftOpen class="size-4" />
				{:else}
					<PanelLeftClose class="size-4" />
				{/if}
			</Button>
		</div>
		{#if !isSidebarCollapsed}
			<div
				class="absolute top-1/2 translate-y-[-50%] left-2 flex items-center min-w-0 select-none overflow-hidden"
				transition:fade={{ duration: 150 }}
			>
				<img
					src="/lasso.svg"
					alt="Lasso"
					class="h-13 max-w-none w-auto object-contain shrink-0 invert dark:invert-0"
				/>
			</div>
		{/if}
	</div>

	{#if !isVerticalSquished}
		<!-- Pop-up Today button when away, or Clock & Sync indicator when viewing Today (hidden in collapsed mode) -->
		<div
			transition:slide={{ duration: 200 }}
			onoutroend={handleVerticalSquishOutroEnd}
			class="relative w-full shrink-0 flex items-center justify-center bg-sidebar z-10 overflow-hidden"
			style="height: var(--calendar-header-offset);"
		>
			{#if !isToday(dayState.value, getLocalTimeZone())}
				<div
					class="absolute inset-0 flex justify-center w-full -mt-[1px]"
					transition:fly={{ y: -24, duration: 200 }}
				>
					<button
						type="button"
						style="height: var(--today-button-height);"
						class="w-[90%] flex items-center justify-center gap-1 rounded-b-md rounded-t-none border-b border-x border-sidebar-border bg-sidebar-accent text-sidebar-foreground hover:bg-sidebar-accent/80 hover:text-primary text-[11px] font-medium cursor-pointer transition-colors shadow-xs"
						onclick={() => dayState.today()}
						aria-label="Return to today"
					>
						<CalendarIcon class="size-3" />
						<span>Today</span>
					</button>
				</div>
			{:else}
				<div
					class="absolute inset-0 flex justify-center items-center w-full"
					style="height: var(--today-button-height);"
					transition:fade={{ duration: 180 }}
				>
					<SidebarSyncIndicator />
				</div>
			{/if}
		</div>
	{/if}

	<!-- Subtle top fade gradient when scrolled down -->
	<div
		class="pointer-events-none absolute left-[0px] right-[0px] z-20 h-6 bg-gradient-to-b from-sidebar via-sidebar/80 to-transparent transition-opacity duration-200 {isScrolled
			? 'opacity-100'
			: 'opacity-0'}"
		style="top: calc(3.5rem + {isVerticalSquished
			? '0px'
			: 'var(--calendar-header-offset)'});"
	></div>

	<!-- Middle scrollable section: Calendar, Tasks, Google Calendars sorted chronologically -->
	<div
		bind:this={scrollElement}
		class="flex-1 min-h-0 sidebar-scrollbar {isSidebarHovered || isScrolling
			? 'sidebar-scrollbar-active'
			: ''} {isSidebarCollapsed
			? 'sidebar-scrollbar-collapsed flex flex-col items-center pr-0'
			: 'pl-2.5 pr-2'} pt-1 pb-2 space-y-2 text-sidebar-foreground select-none overflow-x-hidden transition-[padding] duration-300"
		style:padding-left={isSidebarCollapsed ? `${scrollDiff}px` : undefined}
		onscroll={handleScroll}
	>
		{#if !isVerticalSquished && !showCollapsedContent}
			<div
				in:squishIn={{ duration: 220, y: 20 }}
				out:squishOut={{ duration: 200, y: 20 }}
				onoutroend={handleVerticalSquishOutroEnd}
				class="space-y-2 w-full overflow-hidden"
			>
				<!-- Calendar underneath the branding section, shifted down to align with 12AM marker and scaled down -->
				<SidebarDropdownSection
					title="Calendar"
					storageKey={CALENDAR_STORAGE_KEY}
				>
					<Calendar
						type="single"
						bind:value={dayState.value}
						preventDeselect={true}
						class="w-full border-none bg-transparent p-0 shadow-none [--cell-size:--spacing(6)] text-xs"
					/>
				</SidebarDropdownSection>

				<!-- To Do Section (User-controlled personal todo list) -->
				<SidebarDropdownSection
					title="To Do"
					storageKey={TODO_STORAGE_KEY}
					contentClass="mb-1"
				>
					<SidebarTodoList />
				</SidebarDropdownSection>
			</div>
		{/if}

		{#if isVerticalSquished || showCollapsedContent}
			<SidebarMiniCalendar />
		{/if}

		{#if showCollapsedContent}
			<!-- Collapsed Canvas Section -->
			{#if visibleCanvasCalendars.length > 0}
				<SidebarDropdownSection
					title="Canvas courses"
					collapsed={true}
					storageKey={TASKS_STORAGE_KEY}
					bind:open={isTasksOpen}
				>
					{#each visibleCanvasCalendars as group (group.calendar.id)}
						<SidebarCalendarItem
							calendar={group.calendar}
							collapsed={true}
							oncontextmenu={() => expandAndOpenCourse(group.calendar.id)}
						/>
					{/each}
				</SidebarDropdownSection>
			{/if}

			<!-- Collapsed Google Calendars Section -->
			{#if visibleGoogleCalendars.length > 0}
				<!-- Divider between Canvas and Google Calendars -->
				<div class="w-6 mx-auto my-1 border-t border-sidebar-border/60"></div>

				<SidebarDropdownSection
					title="Google calendars"
					collapsed={true}
					storageKey={GOOGLE_CALENDARS_STORAGE_KEY}
					bind:open={isGoogleCalendarsOpen}
				>
					{#each visibleGoogleCalendars as gcal (gcal.id)}
						<SidebarCalendarItem
							calendar={gcal}
							collapsed={true}
							oncontextmenu={() => expandAndOpenGoogleCalendar()}
						/>
					{/each}
				</SidebarDropdownSection>
			{/if}
		{:else if dataState.loading && dataState.tasks.length === 0}
			<div
				class="flex items-center justify-center py-6 text-xs text-muted-foreground gap-2"
			>
				<RefreshCwIcon class="size-3 animate-spin" />
				<span>Loading tasks...</span>
			</div>
		{:else if visibleCanvasCalendars.length === 0 && visibleGoogleCalendars.length === 0}
			<div
				class="px-3 py-6 text-center text-xs text-muted-foreground space-y-1"
			>
				<p class="font-medium text-foreground/80">No calendars</p>
				<p class="text-[11px] opacity-75">
					Connect Canvas or Google Calendar in Settings to sync your schedule.
				</p>
			</div>
		{:else}
			{#if visibleCanvasCalendars.length > 0}
				<SidebarDropdownSection
					title="Canvas: Tasks by Course"
					storageKey={TASKS_STORAGE_KEY}
					bind:open={isTasksOpen}
					contentClass="space-y-2.5"
				>
					{#each visibleCanvasCalendars as group (group.calendar.id)}
						<SidebarCalendarItem
							calendar={group.calendar}
							tasks={group.tasks}
							upcomingTasks={group.upcomingTasks}
							completedTasks={group.completedTasks}
							bind:isOpen={openCourses[group.calendar.id]}
						/>
					{/each}
				</SidebarDropdownSection>
			{/if}

			{#if visibleGoogleCalendars.length > 0}
				<SidebarDropdownSection
					title="Google Calendars"
					storageKey={GOOGLE_CALENDARS_STORAGE_KEY}
					bind:open={isGoogleCalendarsOpen}
					contentClass="space-y-1"
				>
					{#each visibleGoogleCalendars as gcal (gcal.id)}
						<SidebarCalendarItem calendar={gcal} showColorDot={true} />
					{/each}
				</SidebarDropdownSection>
			{/if}
		{/if}
	</div>

	<!-- User display at the bottom -->
	<div
		class="border-t border-sidebar-border h-14 flex items-center shrink-0 overflow-hidden transition-[padding] duration-300 {isSidebarCollapsed
			? 'justify-center px-0'
			: 'px-2.5'}"
	>
		{#if authState.isAuthenticated && authState.user}
			<div
				class="flex items-center min-w-0 {isSidebarCollapsed
					? 'justify-center'
					: 'w-full'}"
			>
				<button
					type="button"
					class="cursor-pointer group flex items-center shrink-0"
					onclick={() =>
						pageState.current === "settings"
							? pageState.goBack()
							: pageState.setPage("settings")}
					title="{authState.user.name} • Settings"
					aria-label="Account settings"
				>
					<Avatar.Root
						class="size-8 rounded-full border border-sidebar-border bg-sidebar-accent group-hover:border-primary/40 transition-colors shrink-0"
					>
						{#if authState.user.avatarUrl}
							<Avatar.Image
								src={authState.user.avatarUrl}
								alt={authState.user.name}
							/>
						{/if}
						<Avatar.Fallback
							class="text-xs font-medium text-sidebar-foreground"
						>
							<UserIcon class="size-4" />
						</Avatar.Fallback>
					</Avatar.Root>
				</button>

				{#if !isSidebarCollapsed}
					<div
						transition:fade={{ duration: 150 }}
						class="flex items-center justify-between min-w-0 flex-1 overflow-hidden ml-2.5"
					>
						<button
							type="button"
							class="flex flex-col min-w-0 mr-1.5 text-left cursor-pointer group truncate"
							onclick={() =>
								pageState.current === "settings"
									? pageState.goBack()
									: pageState.setPage("settings")}
							aria-label="Account settings"
						>
							<span
								class="truncate text-sm font-medium text-sidebar-foreground group-hover:text-primary transition-colors"
							>
								{authState.user.name}
							</span>
							<span class="truncate text-xs text-muted-foreground">
								{authState.user.email}
							</span>
						</button>

						<Button
							variant={pageState.current === "settings" ? "secondary" : "ghost"}
							size="icon"
							class="size-7 text-sidebar-foreground hover:bg-primary/10 hover:text-primary cursor-pointer shrink-0 transition-colors {pageState.current === 'settings' ? 'text-primary' : ''}"
							onclick={handleSettingsClick}
							aria-label={pageState.current === "settings"
								? "Return to previous page"
								: "Open settings"}
							title="Settings"
						>
							<SettingsIcon
								class="size-4 transition-transform duration-500 ease-out"
								style="transform: rotate({settingsRotation}deg);"
							/>
						</Button>
					</div>
				{/if}
			</div>
		{:else}
			<div
				class="flex items-center min-w-0 text-xs text-muted-foreground {isSidebarCollapsed
					? 'justify-center'
					: ''}"
			>
				<span class="size-2 rounded-full bg-amber-500/80 shrink-0 mx-1"></span>
				{#if !isSidebarCollapsed}
					<div transition:fade={{ duration: 150 }} class="overflow-hidden ml-2">
						<span class="truncate whitespace-nowrap">Not signed in</span>
					</div>
				{/if}
			</div>
		{/if}
	</div>
</aside>

<style>
	:global(.sidebar-scrollbar) {
		overflow-y: auto;
		scrollbar-gutter: stable;
		overflow-x: hidden;
	}

	@supports (-moz-appearance: none) {
		:global(.sidebar-scrollbar) {
			scrollbar-width: thin;
			scrollbar-color: transparent transparent;
		}
		:global(.sidebar-scrollbar.sidebar-scrollbar-active) {
			scrollbar-color: color-mix(
					in srgb,
					var(--sidebar-foreground) 20%,
					transparent
				)
				transparent;
		}
	}

	:global(.sidebar-scrollbar::-webkit-scrollbar) {
		width: 4px;
		background: transparent;
	}

	:global(.sidebar-scrollbar::-webkit-scrollbar-track) {
		background: transparent;
	}

	:global(.sidebar-scrollbar::-webkit-scrollbar-thumb) {
		background-color: transparent;
		border-radius: 9999px;
	}

	:global(
			.sidebar-scrollbar.sidebar-scrollbar-active::-webkit-scrollbar-thumb
		) {
		background-color: color-mix(
			in srgb,
			var(--sidebar-foreground) 20%,
			transparent
		);
	}

	:global(
			.sidebar-scrollbar.sidebar-scrollbar-active::-webkit-scrollbar-thumb:hover
		) {
		background-color: color-mix(
			in srgb,
			var(--sidebar-foreground) 45%,
			transparent
		) !important;
	}

	/* Ultra-low profile scrollbar for collapsed bar */
	:global(.sidebar-scrollbar-collapsed) {
		overflow-y: auto;
		scrollbar-gutter: auto;
		overflow-x: hidden;
	}

	@supports (-moz-appearance: none) {
		:global(.sidebar-scrollbar-collapsed) {
			scrollbar-width: thin;
			scrollbar-color: transparent transparent;
		}
		:global(.sidebar-scrollbar-collapsed.sidebar-scrollbar-active) {
			scrollbar-color: color-mix(
					in srgb,
					var(--sidebar-foreground) 18%,
					transparent
				)
				transparent;
		}
	}

	:global(.sidebar-scrollbar-collapsed::-webkit-scrollbar) {
		width: 4px;
	}

	:global(.sidebar-scrollbar-collapsed::-webkit-scrollbar-thumb) {
		background-color: transparent;
		border-radius: 9999px;
	}

	:global(
			.sidebar-scrollbar-collapsed.sidebar-scrollbar-active::-webkit-scrollbar-thumb
		) {
		background-color: color-mix(
			in srgb,
			var(--sidebar-foreground) 18%,
			transparent
		);
	}

	:global(
			.sidebar-scrollbar-collapsed.sidebar-scrollbar-active::-webkit-scrollbar-thumb:hover
		) {
		background-color: color-mix(
			in srgb,
			var(--sidebar-foreground) 40%,
			transparent
		);
	}
</style>
