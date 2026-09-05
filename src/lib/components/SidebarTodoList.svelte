<script lang="ts">
	import Plus from "@lucide/svelte/icons/plus";
	import ChevronDown from "@lucide/svelte/icons/chevron-down";
	import ChevronUp from "@lucide/svelte/icons/chevron-up";
	import { pb } from "$lib/pocketbase";
	import { authState } from "$lib/authState.svelte";
	import { dataState } from "$lib/dataState/dataState.svelte";
	import { getTodoTasks } from "$lib/dataState/taskQueries.svelte";
	import { ensureTodoCalendar, resolveCalendarColor } from "$lib/dataState/calendarQueries.svelte";
	import SidebarTaskItem from "$lib/components/SidebarTaskItem.svelte";
	import type { CalendarRecord } from "$lib/dataState/dataRecordInterfaces";

	let newTaskName = $state("");
	let isAdding = $state(false);
	let isCompletedExpanded = $state(false);

	const todoData = $derived(getTodoTasks());
	const upcomingTasks = $derived(todoData.upcomingTasks);
	const completedTasks = $derived(todoData.completedTasks);

	const defaultCalendar: CalendarRecord = $derived({
		id: todoData.calendar?.id || "todo_fallback",
		name: "To Do",
		source: "todo",
		color: resolveCalendarColor(todoData.calendar, "#10b981"),
		visible: true,
	});

	async function handleAddTask(e?: Event) {
		if (e) e.preventDefault();
		const name = newTaskName.trim();
		if (!name || isAdding) return;

		isAdding = true;
		try {
			const cal = await ensureTodoCalendar();
			const userId = pb.authStore.record?.id || authState.record?.id;
			if (!cal || !userId) return;

			await dataState.addTask({
				user: userId,
				calendar: cal.id,
				name,
				status: "todo",
				priority: "med",
			});
			newTaskName = "";
		} catch (err) {
			console.error("Failed to add todo task:", err);
		} finally {
			isAdding = false;
		}
	}

	async function handleDeleteTask(taskId: string) {
		try {
			await dataState.deleteTask(taskId);
		} catch (err) {
			console.error("Failed to delete task:", err);
		}
	}
</script>

<div class="space-y-1 px-0.5">
	<!-- Quick Add Input -->
	<form
		onsubmit={handleAddTask}
		class="flex items-center gap-1.5 px-2 py-1 rounded-md border border-sidebar-border/70 bg-sidebar-accent/30 focus-within:border-primary/60 focus-within:bg-sidebar-accent/50 transition-all text-xs"
	>
		<input
			type="text"
			bind:value={newTaskName}
			placeholder="Add a task..."
			class="flex-1 min-w-0 bg-transparent text-[11.5px] text-sidebar-foreground placeholder:text-muted-foreground/60 focus:outline-none"
			disabled={isAdding}
		/>
		<button
			type="submit"
			disabled={!newTaskName.trim() || isAdding}
			class="size-4.5 rounded flex items-center justify-center text-muted-foreground hover:text-foreground hover:bg-sidebar-accent transition-colors cursor-pointer shrink-0 disabled:opacity-30 disabled:cursor-default"
			title="Add task"
			aria-label="Add task"
		>
			<Plus class="size-3" />
		</button>
	</form>

	<!-- Upcoming Tasks -->
	{#if upcomingTasks.length > 0}
		<div class="space-y-0.5 pt-0.5">
			{#each upcomingTasks as task (task.id)}
				<SidebarTaskItem
					{task}
					calendar={todoData.calendar || defaultCalendar}
					onDelete={handleDeleteTask}
				/>
			{/each}
		</div>
	{/if}

	<!-- Completed Tasks (Collapsible) -->
	{#if completedTasks.length > 0}
		<div class="pt-1">
			<button
				type="button"
				onclick={() => (isCompletedExpanded = !isCompletedExpanded)}
				class="w-full flex items-center justify-between text-[10px] font-medium text-muted-foreground/70 uppercase tracking-wider py-1 px-1 hover:text-foreground transition-colors cursor-pointer"
			>
				<span>Completed ({completedTasks.length})</span>
				{#if isCompletedExpanded}
					<ChevronUp class="size-2.5" />
				{:else}
					<ChevronDown class="size-2.5" />
				{/if}
			</button>

			{#if isCompletedExpanded}
				<div class="space-y-0.5">
					{#each completedTasks as task (task.id)}
						<SidebarTaskItem
							{task}
							calendar={todoData.calendar || defaultCalendar}
							completed={true}
							onDelete={handleDeleteTask}
						/>
					{/each}
				</div>
			{/if}
		</div>
	{/if}

	<!-- Empty state if 0 tasks -->
	{#if upcomingTasks.length === 0 && completedTasks.length === 0}
		<div class="py-2 px-2 text-center text-[10.5px] text-muted-foreground/50 italic">
			No tasks yet
		</div>
	{/if}
</div>
