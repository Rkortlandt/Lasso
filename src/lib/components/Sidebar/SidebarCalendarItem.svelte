<script lang="ts">
	import ChevronDown from "@lucide/svelte/icons/chevron-down";
	import ChevronUp from "@lucide/svelte/icons/chevron-up";
	import EyeIcon from "@lucide/svelte/icons/eye";
	import EyeOffIcon from "@lucide/svelte/icons/eye-off";
	import Pipette from "@lucide/svelte/icons/pipette";
	import { calendarVisibilityState } from "$lib/calendarVisibilityState.svelte";
	import { pageState } from "$lib/pageSystem.svelte";
	import { slide, fly } from "svelte/transition";
	import SidebarTaskItem from "./SidebarTaskItem.svelte";
	import type {
		CalendarRecord,
		TaskRecord,
	} from "$lib/dataState/dataRecordInterfaces";

	interface Props {
		calendar: CalendarRecord;
		collapsed?: boolean;
		collapsible?: boolean;
		isOpen?: boolean;
		tasks?: TaskRecord[];
		upcomingTasks?: TaskRecord[];
		completedTasks?: TaskRecord[];
		showColorDot?: boolean;
		showActions?: boolean;
		collapsedTooltip?: string;
		oncontextmenu?: (e: MouseEvent) => void;
		children?: import("svelte").Snippet;
	}

	let {
		calendar,
		collapsed = false,
		collapsible,
		isOpen = $bindable(),
		tasks,
		upcomingTasks = [],
		completedTasks = [],
		showColorDot,
		showActions = true,
		collapsedTooltip,
		oncontextmenu,
		children,
	}: Props = $props();

	function getSavedOpenState(calId: string): boolean {
		if (typeof window === "undefined") return false;
		try {
			const saved = localStorage.getItem("lasso_sidebar_open_courses");
			if (saved) {
				const parsed = JSON.parse(saved);
				return Boolean(parsed[calId]);
			}
		} catch {}
		return false;
	}

	function saveOpenState(calId: string, openVal: boolean) {
		if (typeof window === "undefined") return;
		try {
			const saved = localStorage.getItem("lasso_sidebar_open_courses");
			const parsed = saved ? JSON.parse(saved) : {};
			parsed[calId] = openVal;
			localStorage.setItem("lasso_sidebar_open_courses", JSON.stringify(parsed));
		} catch {}
	}

	let internalOpen = $state<boolean | null>(null);

	const isCollapsible = $derived(
		collapsible ??
			(tasks !== undefined ||
				upcomingTasks.length > 0 ||
				completedTasks.length > 0 ||
				children !== undefined ||
				calendar.source === "canvas"),
	);

	let currentOpen = $derived.by(() => {
		if (isOpen !== undefined) return isOpen;
		if (internalOpen !== null) return internalOpen;
		return getSavedOpenState(calendar.id);
	});

	let isUpcomingExpanded = $state(false);
	let isCompletedExpanded = $state(false);

	const displayName = $derived(calendar.nickname || calendar.name);
	const calColor = $derived(calendar.color || "#3b82f6");
	const isCalHidden = $derived(
		calendarVisibilityState.isHiddenOnCalendar(calendar.id, calendar.name),
	);
	const isCalIsolated = $derived(
		calendarVisibilityState.isIsolated(calendar.id, calendar.name),
	);

	const shouldShowColorDot = $derived(showColorDot ?? !isCollapsible);

	const totalTasksCount = $derived(
		tasks ? tasks.length : upcomingTasks.length + completedTasks.length,
	);

	const tooltipText = $derived(
		collapsedTooltip ??
			(isCollapsible
				? "Right-click to open in sidebar"
				: "Right-click to expand"),
	);

	function getCalendarInitials(name: string): string {
		if (!name) return "";
		const clean = name.trim();
		if (clean.length <= 2) return clean.toUpperCase();
		return clean.slice(0, 2).toUpperCase();
	}

	const initials = $derived(getCalendarInitials(displayName));

	function toggleOpen() {
		if (!isCollapsible) return;
		const next = !currentOpen;
		if (isOpen !== undefined) {
			isOpen = next;
		} else {
			internalOpen = next;
		}
		saveOpenState(calendar.id, next);
	}

	function handleContextMenu(e: MouseEvent) {
		if (oncontextmenu) {
			e.preventDefault();
			oncontextmenu(e);
		}
	}
</script>

{#if collapsed}
	<button
		type="button"
		in:fly={{ y: 16, duration: 200 }}
		out:fly={{ y: -16, duration: 200 }}
		class="size-[34px] rounded-md border flex items-center justify-center transition-all cursor-pointer select-none group/sq relative hover:bg-sidebar-accent/40 shadow-2xs shrink-0"
		style="border-color: {calColor}; color: {calColor}; background: transparent;"
		onclick={() =>
			calendarVisibilityState.toggleCalendarVisibility(
				calendar.id,
				calendar.name,
			)}
		oncontextmenu={handleContextMenu}
		title="{displayName} ({tooltipText})"
		aria-label={displayName}
	>
		{#if isCalHidden}
			<EyeOffIcon
				class="size-4 shrink-0 opacity-80 group-hover/sq:opacity-100"
			/>
		{:else}
			<span
				class="group-hover/sq:hidden tracking-wider text-[11px] font-bold leading-none text-white"
				style="color: #ffffff;">{initials}</span
			>
			<EyeIcon class="size-4 shrink-0 hidden group-hover/sq:block" />
		{/if}
	</button>
{:else}
	<div
		in:fly={{ y: 16, duration: 200 }}
		out:fly={{ y: -16, duration: 200 }}
		class="space-y-1 {isCalHidden && !isCalIsolated ? 'opacity-70' : ''}"
	>
		<div
			class="w-full grid grid-cols-[1fr_auto] items-center rounded-md text-xs font-medium text-sidebar-foreground bg-transparent select-none min-h-[34px] gap-1"
		>
			{#if isCollapsible}
				<button
					type="button"
					class="w-full min-w-0 flex items-center justify-between px-2.5 py-1 rounded-md text-xs font-medium text-sidebar-foreground border bg-transparent hover:bg-sidebar-accent/40 transition-colors cursor-pointer group select-none shadow-xs"
					style="border-color: {calColor};"
					onclick={toggleOpen}
					aria-expanded={currentOpen}
				>
					<div class="flex items-center gap-1.5 min-w-0">
						{#if shouldShowColorDot}
							<span
								class="size-2 rounded-xs shrink-0 shadow-2xs"
								style="background-color: {calColor};"
							></span>
						{/if}
						<span
							class="truncate font-medium text-[11px] tracking-tight {isCalHidden &&
							!isCalIsolated
								? 'text-muted-foreground line-through opacity-75'
								: ''}"
						>
							{displayName}
						</span>
					</div>

					<div
						class="flex items-center gap-0.5 shrink-0 text-muted-foreground"
					>
						<span class="size-5 flex items-center justify-center shrink-0">
							<ChevronDown
								class="size-3.5 transition-transform duration-150 {currentOpen
									? ''
									: '-rotate-90'}"
							/>
						</span>
					</div>
				</button>
			{:else}
				<div
					class="w-full min-w-0 flex items-center justify-between px-2.5 py-1 rounded-md text-xs font-medium text-sidebar-foreground border bg-transparent hover:bg-sidebar-accent/40 transition-colors select-none min-h-[34px]"
					style="border-color: {calColor};"
				>
					<div class="flex items-center gap-1.5 min-w-0">
						{#if shouldShowColorDot}
							<span
								class="size-2 rounded-xs shrink-0 shadow-2xs"
								style="background-color: {calColor};"
							></span>
						{/if}
						<span
							class="truncate font-medium text-[11px] tracking-tight {isCalHidden &&
							!isCalIsolated
								? 'text-muted-foreground line-through opacity-75'
								: ''}"
						>
							{displayName}
						</span>
					</div>
				</div>
			{/if}

			{#if showActions && pageState.current !== "settings"}
				<div class="col-start-2 flex items-center">
					<!-- Isolate button (eyedropper) -->
					<button
						tabindex="0"
						class="size-5 rounded flex items-center justify-center transition-colors cursor-pointer {isCalIsolated
							? 'text-primary hover:text-primary/70'
							: 'text-muted-foreground/60 hover:text-foreground hover:bg-sidebar-accent/80'}"
						onclick={(e) => {
							e.stopPropagation();
							calendarVisibilityState.toggleIsolate(calendar.id, calendar.name);
						}}
						onkeydown={(e) => {
							if (e.key === "Enter" || e.key === " ") {
								e.stopPropagation();
								calendarVisibilityState.toggleIsolate(calendar.id, calendar.name);
							}
						}}
						title={isCalIsolated
							? "Un-isolate calendar (click to clear)"
							: "Isolate calendar"}
						aria-label={isCalIsolated
							? "Un-isolate calendar"
							: "Isolate calendar"}
					>
						<Pipette class="size-3.5" />
					</button>

					<!-- Toggle visibility on main calendar -->
					<button
						type="button"
						class="size-5 rounded flex items-center justify-center transition-colors hover:text-foreground hover:bg-sidebar-accent/80 cursor-pointer {isCalHidden
							? 'text-muted-foreground/50 hover:text-foreground'
							: 'text-muted-foreground/80 hover:text-foreground'}"
						onclick={(e) => {
							e.stopPropagation();
							calendarVisibilityState.toggleCalendarVisibility(
								calendar.id,
								calendar.name,
							);
						}}
						title={isCalHidden
							? "Hidden on calendar grid (click to show)"
							: "Visible on calendar grid (click to hide)"}
						aria-label={isCalHidden
							? "Show on calendar grid"
							: "Hide on calendar grid"}
					>
						{#if isCalHidden}
							<EyeOffIcon class="size-3.5" />
						{:else}
							<EyeIcon class="size-3.5" />
						{/if}
					</button>
				</div>
			{/if}
		</div>

		{#if currentOpen}
			{#if children}
				<div
					transition:slide={{ duration: 150 }}
					class="space-y-0.5 pt-0.5 px-0.5"
				>
					{@render children()}
				</div>
			{:else if isCollapsible}
				{@const visibleUpcoming = isUpcomingExpanded
					? upcomingTasks
					: upcomingTasks.slice(0, 7)}
				{@const visibleCompleted = isCompletedExpanded
					? completedTasks
					: completedTasks.slice(0, 3)}
				<div
					transition:slide={{ duration: 150 }}
					class="space-y-0.5 pt-0.5 px-0.5"
				>
					{#if totalTasksCount === 0}
						<div
							class="px-2 py-1 text-[10.5px] text-muted-foreground/60 italic"
						>
							No assignments
						</div>
					{:else}
						<!-- Upcoming assignments -->
						{#if upcomingTasks.length > 0}
							{#each visibleUpcoming as task (task._clientId || task.id)}
								<div out:slide={{ duration: 220 }}>
									<SidebarTaskItem {task} {calendar} />
								</div>
							{/each}

							{#if upcomingTasks.length > 7}
								<button
									type="button"
									class="w-full py-1 px-2 text-[10px] text-muted-foreground hover:text-foreground font-medium text-center hover:bg-sidebar-accent/40 rounded transition-colors cursor-pointer flex items-center justify-center gap-1"
									onclick={() => (isUpcomingExpanded = !isUpcomingExpanded)}
								>
									{#if isUpcomingExpanded}
										<span>Show less</span>
										<ChevronUp class="size-2.5" />
									{:else}
										<span>Show {upcomingTasks.length - 7} more</span>
										<ChevronDown class="size-2.5" />
									{/if}
								</button>
							{/if}
						{/if}

						<!-- Completed assignments -->
						{#if completedTasks.length > 0}
							<div
								class="pt-1.5 pb-0.5 px-1.5 flex items-center gap-1.5 text-[10px] font-medium text-muted-foreground/70 uppercase tracking-wider"
							>
								<span>Completed</span>
								<div class="h-px flex-1 bg-sidebar-border/40"></div>
							</div>

							{#each visibleCompleted as task (task._clientId || task.id)}
								<div transition:slide={{ duration: 200 }}>
									<SidebarTaskItem
										{task}
										{calendar}
										completed={true}
									/>
								</div>
							{/each}

							{#if completedTasks.length > 3}
								<button
									type="button"
									class="w-full py-1 px-2 text-[10px] text-muted-foreground hover:text-foreground font-medium text-center hover:bg-sidebar-accent/40 rounded transition-colors cursor-pointer flex items-center justify-center gap-1"
									onclick={() => (isCompletedExpanded = !isCompletedExpanded)}
								>
									{#if isCompletedExpanded}
										<span>Show less</span>
										<ChevronUp class="size-2.5" />
									{:else}
										<span>Show {completedTasks.length - 3} more</span>
										<ChevronDown class="size-2.5" />
									{/if}
								</button>
							{/if}
						{/if}
					{/if}
				</div>
			{/if}
		{/if}
	</div>
{/if}
