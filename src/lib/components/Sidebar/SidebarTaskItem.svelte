<script lang="ts">
	import GripVertical from "@lucide/svelte/icons/grip-vertical";
	import CheckIcon from "@lucide/svelte/icons/check";
	import Trash2 from "@lucide/svelte/icons/trash-2";
	import ExternalLink from "@lucide/svelte/icons/external-link";
	import { authState } from "$lib/authState.svelte";
	import { dataState } from "$lib/dataState/dataState.svelte";
	import { dragState, type DragTaskPayload } from "$lib/dragState.svelte";
	import { parseTaskCalendarId } from "$lib/dataState/taskQueries.svelte";
	import { resolveCalendarColor } from "$lib/dataState/calendarQueries.svelte";
	import { pageState } from "$lib/pageSystem.svelte";
	import { calendarSelectionState } from "$lib/calendarSelectionState.svelte";
	import type {
		CalendarRecord,
		TaskRecord,
	} from "$lib/dataState/dataRecordInterfaces";

	interface Props {
		task: TaskRecord;
		calendar: CalendarRecord;
		completed?: boolean;
		onDelete?: (taskId: string) => void;
	}

	let { task, calendar, completed = false, onDelete }: Props = $props();

	const canvasUrl = $derived.by(() => {
		if (task.source_link) return task.source_link;

		const isCanvas =
			calendar?.source === "canvas" ||
			task.expand?.calendar?.source === "canvas" ||
			Boolean(calendar?.course_id);
		if (!isCanvas) return null;

		const baseUrl = (
			authState.record?.canvas_url || "https://canvas.instructure.com"
		).replace(/\/+$/, "");

		let courseId = calendar?.course_id;
		if (
			!courseId &&
			typeof task.calendar === "object" &&
			!Array.isArray(task.calendar) &&
			task.calendar !== null
		) {
			courseId = (task.calendar as Record<string, any>).course_id;
		}
		if (!courseId && task.expand?.calendar?.course_id) {
			courseId = task.expand.calendar.course_id;
		}

		if (courseId) {
			return `${baseUrl}/courses/${courseId}/assignments`;
		}
		return baseUrl;
	});

	function handleDragStart(e: DragEvent) {
		if (!e.dataTransfer) return;
		const calId = parseTaskCalendarId(task) || calendar.id;
		const color =
			resolveCalendarColor(calendar) ||
			task.expand?.calendar?.color ||
			"#3b82f6";
		const courseName =
			calendar.nickname ||
			calendar.name ||
			task.expand?.calendar?.nickname ||
			task.expand?.calendar?.name ||
			"Coursework";

		const payload: DragTaskPayload = {
			taskId: task.id,
			taskName: task.name,
			calendarId: calId,
			color,
			courseName,
		};

		dragState.setTask(payload);
		e.dataTransfer.setData("application/json", JSON.stringify(payload));
		e.dataTransfer.setData("text/plain", task.name);
		e.dataTransfer.effectAllowed = "copy";
	}

	function handleDragEnd() {
		dragState.clear();
	}

	let isChecking = $state(false);

	async function setTaskState() {
		const calId = parseTaskCalendarId(task);
		const cal = calId ? dataState.calendars.find((c) => c.id === calId) : null;
		if (
			cal?.source === "canvas" ||
			task.expand?.calendar?.source === "canvas"
		) {
			return; // Canvas task state is managed by Canvas
		}

		if (isChecking) return;

		if (!completed && task.status !== "done") {
			isChecking = true;
			await new Promise((resolve) => setTimeout(resolve, 440));
		}

		const nextStatus = task.status === "done" ? "todo" : "done";
		try {
			await dataState.updateTask(task.id, { status: nextStatus });
		} catch (err) {
			console.error("Failed to update task:", err);
			isChecking = false;
		}
	}

	function formatDueDate(dueStr?: string): string {
		if (!dueStr) return "";
		try {
			const d = new Date(dueStr);
			if (isNaN(d.getTime())) return "";
			const now = new Date();
			const isSameDay =
				d.getFullYear() === now.getFullYear() &&
				d.getMonth() === now.getMonth() &&
				d.getDate() === now.getDate();
			const tomorrow = new Date(now);
			tomorrow.setDate(tomorrow.getDate() + 1);
			const isTomorrow =
				d.getFullYear() === tomorrow.getFullYear() &&
				d.getMonth() === tomorrow.getMonth() &&
				d.getDate() === tomorrow.getDate();

			const timeStr = d.toLocaleTimeString([], {
				hour: "numeric",
				minute: "2-digit",
			});

			if (isSameDay) {
				return `Today ${timeStr}`;
			}
			if (isTomorrow) {
				return `Tomorrow ${timeStr}`;
			}
			const monthDay = d.toLocaleDateString(undefined, {
				month: "short",
				day: "numeric",
			});
			return `${monthDay}`;
		} catch {
			return "";
		}
	}

	const formattedDue = $derived(formatDueDate(task.due_date));
	const isSettings = $derived(pageState.current === "settings");
</script>

<div
	role="group"
	aria-label={task.name}
	class="group/task flex rounded-md items-stretch hover:bg-sidebar-accent/50 transition-colors text-xs select-none overflow-hidden {isSettings
		? ''
		: 'cursor-grab active:cursor-grabbing'}"
	draggable={!isSettings}
	ondragstart={isSettings ? undefined : handleDragStart}
	ondragend={isSettings ? undefined : handleDragEnd}
>
	<!-- Left section: Drag handle (hidden in settings, exact space preserved) -->
	<div
		class="flex items-center justify-center px-.5 text-muted-foreground/40 shrink-0 select-none {isSettings
			? 'invisible pointer-events-none'
			: 'cursor-grab active:cursor-grabbing hover:text-foreground transition-colors opacity-0 group-hover/task:opacity-100'}"
		title={isSettings ? undefined : "Drag onto calendar"}
	>
		<GripVertical class="size-2.5" />
	</div>

	<!-- Dividing line: spans top to bottom, matches sidebar background -->
	<div
		class="w-[2px] self-stretch bg-sidebar shrink-0 pointer-events-none"
		aria-hidden="true"
	></div>

	<!-- Right section: Overlaps and overrides drag and cursor -->
	{#if canvasUrl}
		<div
			role="button"
			tabindex="0"
			class="group/task-link flex-1 min-w-0 flex items-start gap-1.5 px-1 py-1.5 cursor-pointer no-underline bg-transparent"
			title={`Click to view on calendar: ${task.name}`}
			onclick={() => calendarSelectionState.openTaskEvent(task, calendar)}
			onkeydown={(e) => {
				if (e.key === "Enter" || e.key === " ") {
					e.preventDefault();
					calendarSelectionState.openTaskEvent(task, calendar);
				}
			}}
			draggable="false"
			ondragstart={(e) => {
				e.preventDefault();
				e.stopPropagation();
			}}
		>
			{#if completed}
				<!-- Status checkmark on Canvas -->
				<span
					class="mt-0.5 size-3.5 flex items-center justify-center text-emerald-500 shrink-0 select-none group-hover/task-link:text-emerald-400"
					title="Submitted on Canvas"
				>
					<CheckIcon class="size-3 stroke-[2.5]" />
				</span>
			{:else}
				<!-- Status indicator / Canvas course dot -->
				<span
					class="mt-1.5 size-1.5 rounded-full shrink-0 mx-0.5 opacity-70 select-none group-hover/task-link:opacity-100 transition-opacity"
					style="background-color: {calendar.color || '#3b82f6'};"
					title="Due on Canvas"
				></span>
			{/if}

			<!-- Task info -->
			<div class="flex-1 min-w-0">
				<div class="flex items-center justify-between gap-1.5 min-w-0">
					<p
						class="text-[11px] leading-snug truncate transition-colors flex-1 min-w-0 group-hover/task-link:underline {completed
							? 'line-through text-muted-foreground opacity-60'
							: 'text-sidebar-foreground'}"
						title={task.name}
					>
						{task.name}
					</p>
					<a
						href={canvasUrl}
						target="_blank"
						rel="noopener noreferrer"
						class="opacity-0 group-hover/task:opacity-100 hover:text-foreground text-muted-foreground/40 hover:bg-sidebar-accent/50 p-0.5 rounded transition-all cursor-pointer shrink-0"
						title="Open in Canvas"
						aria-label={`Open ${task.name} in Canvas`}
						onclick={(e) => {
							e.stopPropagation();
						}}
					>
						<ExternalLink class="size-3" />
					</a>
				</div>

				{#if formattedDue || (completed && task.grade)}
					<div class="flex items-center gap-1.5 text-[9px] leading-tight">
						{#if formattedDue}
							<span
								class="text-muted-foreground/75 {completed
									? 'font-mono'
									: ''}"
							>
								{formattedDue}
							</span>
						{/if}
						{#if completed && task.grade}
							<span
								class="font-mono text-emerald-600 dark:text-emerald-400 font-medium"
								title={`Grade: ${task.grade}`}
							>
								{task.grade}
							</span>
						{/if}
					</div>
				{/if}
			</div>
		</div>
	{:else}
		<div
			role="button"
			tabindex="0"
			class="flex-1 min-w-0 flex items-start gap-1.5 px-1 py-1.5 cursor-pointer bg-transparent"
			draggable="false"
			onclick={() => calendarSelectionState.openTaskEvent(task, calendar)}
			onkeydown={(e) => {
				if (e.key === "Enter" || e.key === " ") {
					e.preventDefault();
					calendarSelectionState.openTaskEvent(task, calendar);
				}
			}}
			ondragstart={(e) => {
				e.preventDefault();
				e.stopPropagation();
			}}
		>
			<!-- Checkbox toggle button for manual tasks -->
			<button
				type="button"
				class="mt-0.5 size-3.5 rounded border flex items-center justify-center shrink-0 transition-all duration-200 cursor-pointer {isChecking || completed
					? 'bg-neutral-200/90 border-neutral-300 dark:bg-neutral-800 dark:border-neutral-700/80 text-emerald-600 dark:text-emerald-400 shadow-xs'
					: 'border-muted-foreground/40 hover:border-emerald-500/70 hover:bg-emerald-500/10'} {isChecking
					? 'task-box-anim'
					: ''}"
				onclick={(e) => {
					e.stopPropagation();
					setTaskState();
				}}
				aria-label={completed ? "Mark as incomplete" : "Mark as completed"}
				disabled={isChecking}
			>
				{#if isChecking}
					<svg
						class="size-2.5"
						viewBox="0 0 12 12"
						fill="none"
						stroke="currentColor"
						stroke-width="2.4"
						stroke-linecap="round"
						stroke-linejoin="round"
					>
						<path
							d="M2.5 6.3L4.8 8.6L9.5 3.4"
							class="task-check-path"
						/>
					</svg>
				{:else if completed}
					<CheckIcon class="size-2.5 stroke-[3]" />
				{/if}
			</button>

			<!-- Task info -->
			<div class="flex-1 min-w-0">
				<div class="flex items-center justify-between gap-1.5 min-w-0">
					<p
						class="text-[11px] leading-snug truncate flex-1 min-w-0"
						title={task.name}
					>
						<span class="relative inline-block max-w-full truncate align-bottom">
							<span
								class="truncate transition-colors duration-200 {isChecking || completed
									? 'text-muted-foreground opacity-60'
									: 'text-sidebar-foreground'} {completed && !isChecking
									? 'line-through'
									: ''}"
							>
								{task.name}
							</span>
							{#if isChecking}
								<span class="strike-through-line"></span>
							{/if}
						</span>
					</p>
					{#if onDelete}
						<button
							type="button"
							class="opacity-0 group-hover/task:opacity-100 hover:text-destructive text-muted-foreground/40 hover:bg-sidebar-accent/50 p-0.5 rounded transition-all cursor-pointer shrink-0"
							onclick={(e) => {
								e.stopPropagation();
								onDelete(task.id);
							}}
							title="Delete task"
							aria-label="Delete task"
						>
							<Trash2 class="size-3" />
						</button>
					{/if}
				</div>

				{#if formattedDue || (completed && task.grade)}
					<div
						class="flex items-center gap-1.5 text-[9px] leading-tight transition-opacity duration-200 {isChecking || completed
							? 'opacity-70'
							: ''}"
					>
						{#if formattedDue}
							<span
								class="text-muted-foreground/75 {isChecking || completed
									? 'font-mono'
									: ''}"
							>
								{formattedDue}
							</span>
						{/if}
						{#if completed && task.grade}
							<span
								class="font-mono text-emerald-600 dark:text-emerald-400 font-medium"
								title={`Grade: ${task.grade}`}
							>
								{task.grade}
							</span>
						{/if}
					</div>
				{/if}
			</div>
		</div>
	{/if}
</div>

<style>
	.task-box-anim {
		animation: box-pop 250ms cubic-bezier(0.175, 0.885, 0.32, 1.275) forwards;
	}

	@keyframes box-pop {
		0% {
			transform: scale(0.9);
		}
		50% {
			transform: scale(1.18);
		}
		100% {
			transform: scale(1);
		}
	}

	.task-check-path {
		stroke-dasharray: 14;
		stroke-dashoffset: 14;
		animation: draw-checkmark 200ms cubic-bezier(0.65, 0, 0.45, 1) 40ms forwards;
	}

	@keyframes draw-checkmark {
		0% {
			stroke-dashoffset: 14;
		}
		100% {
			stroke-dashoffset: 0;
		}
	}

	.strike-through-line {
		position: absolute;
		left: 0;
		top: 50%;
		height: 1.2px;
		background-color: currentColor;
		color: var(--color-muted-foreground, #888);
		opacity: 0.7;
		border-radius: 9999px;
		pointer-events: none;
		animation: strike-anim 240ms cubic-bezier(0.4, 0, 0.2, 1) 70ms forwards;
	}

	@keyframes strike-anim {
		0% {
			width: 0%;
		}
		100% {
			width: 100%;
		}
	}
</style>
