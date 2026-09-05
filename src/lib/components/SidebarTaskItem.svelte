<script lang="ts">
	import GripVertical from "@lucide/svelte/icons/grip-vertical";
	import CheckIcon from "@lucide/svelte/icons/check";
	import Trash2 from "@lucide/svelte/icons/trash-2";
	import { authState } from "$lib/authState.svelte";
	import { dataState } from "$lib/dataState/dataState.svelte";
	import { dragState, type DragTaskPayload } from "$lib/dragState.svelte";
	import { parseTaskCalendarId } from "$lib/dataState/taskQueries.svelte";
	import { resolveCalendarColor } from "$lib/dataState/calendarQueries.svelte";
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

	async function setTaskState() {
		const calId = parseTaskCalendarId(task);
		const cal = calId ? dataState.calendars.find((c) => c.id === calId) : null;
		if (
			cal?.source === "canvas" ||
			task.expand?.calendar?.source === "canvas"
		) {
			return; // Canvas task state is managed by Canvas
		}

		const nextStatus = task.status === "done" ? "todo" : "done";
		try {
			await dataState.updateTask(task.id, { status: nextStatus });
		} catch (err) {
			console.error("Failed to update task:", err);
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
</script>

<div
	role="group"
	aria-label={task.name}
	class="group/task flex rounded-md items-stretch hover:bg-sidebar-accent/50 transition-colors text-xs select-none overflow-hidden cursor-grab active:cursor-grabbing"
	draggable="true"
	ondragstart={handleDragStart}
	ondragend={handleDragEnd}
>
	<!-- Left section: Drag handle -->
	<div
		class="flex items-center justify-center px-.5 cursor-grab active:cursor-grabbing text-muted-foreground/40 hover:text-foreground transition-colors shrink-0 select-none opacity-0 group-hover/task:opacity-100"
		title="Drag onto calendar"
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
		<a
			href={canvasUrl}
			target="_blank"
			rel="noopener noreferrer"
			draggable="false"
			class="group/task-link flex-1 min-w-0 flex items-start gap-1.5 px-1 py-1.5 cursor-pointer no-underline bg-transparent"
			title={`Open ${task.name} in Canvas`}
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
					{#if completed && task.grade}
						<span
							class="text-[9px] font-semibold font-mono text-emerald-600 dark:text-emerald-400 shrink-0 px-1.5 py-0.5 rounded bg-emerald-500/10 border border-emerald-500/20 leading-none"
							title={`Grade: ${task.grade}`}
						>
							{task.grade}
						</span>
					{/if}
				</div>

				{#if formattedDue}
					<span
						class="text-[9px] text-muted-foreground/75 block {completed
							? 'font-mono'
							: ''}"
					>
						{formattedDue}
					</span>
				{/if}
			</div>
		</a>
	{:else}
		<div
			role="presentation"
			class="flex-1 min-w-0 flex items-start gap-1.5 px-1 py-1.5 cursor-default bg-transparent"
			draggable="false"
			ondragstart={(e) => {
				e.preventDefault();
				e.stopPropagation();
			}}
		>
			<!-- Checkbox toggle button for manual tasks -->
			<button
				type="button"
				class="mt-0.5 size-3.5 rounded border flex items-center justify-center shrink-0 transition-colors cursor-pointer {completed
					? 'bg-primary border-primary text-primary-foreground'
					: 'border-muted-foreground/40 hover:border-primary'}"
				onclick={(e) => {
					e.stopPropagation();
					setTaskState();
				}}
				aria-label={completed ? "Mark as incomplete" : "Mark as completed"}
			>
				{#if completed}
					<CheckIcon class="size-2.5 stroke-[3]" />
				{/if}
			</button>

			<!-- Task info -->
			<div class="flex-1 min-w-0">
				<div class="flex items-center justify-between gap-1.5 min-w-0">
					<p
						class="text-[11px] leading-snug truncate transition-colors flex-1 min-w-0 {completed
							? 'line-through text-muted-foreground opacity-60'
							: 'text-sidebar-foreground'}"
						title={task.name}
					>
						{task.name}
					</p>
					{#if completed && task.grade}
						<span
							class="text-[9px] font-semibold font-mono text-emerald-600 dark:text-emerald-400 shrink-0 px-1.5 py-0.5 rounded bg-emerald-500/10 border border-emerald-500/20 leading-none"
							title={`Grade: ${task.grade}`}
						>
							{task.grade}
						</span>
					{/if}
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

				{#if formattedDue}
					<span
						class="text-[9px] text-muted-foreground/75 block {completed
							? 'font-mono'
							: ''}"
					>
						{formattedDue}
					</span>
				{/if}
			</div>
		</div>
	{/if}
</div>
