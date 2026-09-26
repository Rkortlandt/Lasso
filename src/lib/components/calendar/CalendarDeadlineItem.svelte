<script lang="ts">
	import type { FormattedDeadline } from "./calendarTypes";

	interface Props {
		deadline: FormattedDeadline;
		visibleColIndex: number;
		onSelect?: (deadline: FormattedDeadline, visibleColIndex: number) => void;
	}

	let { deadline, visibleColIndex, onSelect }: Props = $props();
</script>

<div
	role="button"
	tabindex="0"
	class="absolute pointer-events-auto group/deadline select-none flex cursor-pointer z-10 hover:z-50 overflow-visible text-left {deadline.isEndOfDay
		? 'py-2 items-end'
		: 'py-2.5 items-center'}"
	style="
		left: calc({deadline.leftPercent}% + 1.5px);
		width: calc({deadline.widthPercent}% - 3px);
		{deadline.isEndOfDay
		? 'bottom: 0px;'
		: `top: ${deadline.topPercent}%; transform: translateY(-50%);`}
	"
	onclick={(e) => {
		e.stopPropagation();
		onSelect?.(deadline, visibleColIndex);
	}}
	onkeydown={(e) => {
		if (e.key === "Enter" || e.key === " ") {
			e.stopPropagation();
			onSelect?.(deadline, visibleColIndex);
		}
	}}
>
	<!-- Colored / Greyed Deadline Line (stable, no vertical movement on hover) -->
	<div
		class="w-full h-1 rounded-full transition-opacity duration-150 {deadline.status ===
		'done'
			? 'opacity-40 group-hover/deadline:opacity-85'
			: 'group-hover/deadline:opacity-100'}"
		style="
			background-color: {deadline.status === 'done' ? '#9ca3af' : deadline.color};
			box-shadow: {deadline.status === 'done'
			? '0 1px 2px rgba(0,0,0,0.1)'
			: `0 0 5px ${deadline.color}80, 0 1px 2px rgba(0,0,0,0.15)`};
		"
	></div>

	{#if !deadline.isEndOfDay && deadline.status !== "done"}
		<!-- Tag-like marker on right edge (triangley-square shape facing left) -->
		<div
			class="absolute right-0 top-1/2 -translate-y-1/2 flex items-center justify-center pointer-events-none z-10"
		>
			<svg
				class="w-2.5 h-2.5 transition-transform duration-150 group-hover/deadline:scale-125"
				viewBox="0 0 10 10"
				fill="none"
				style="filter: drop-shadow(0 1px 2px rgba(0,0,0,0.35));"
			>
				<path
					d="M0 5 L4.5 0 L9 0 C9.55 0 10 0.45 10 1 L10 9 C10 9.55 9.55 10 9 10 L4.5 10 Z"
					fill={deadline.color}
				/>
			</svg>
		</div>
	{/if}

	<!-- Hover Details Card (solid, zero transparency) -->
	<div
		class="absolute hidden group-hover/deadline:flex flex-col gap-1.5 z-[100] pointer-events-none p-2.5 rounded-lg bg-popover border border-border shadow-2xl text-popover-foreground text-xs min-w-[200px] max-w-[260px] animate-in fade-in zoom-in-95 duration-150
			{deadline.isEndOfDay || deadline.topPercent > 70
			? 'bottom-full mb-2'
			: 'top-full mt-2'}
			{visibleColIndex === 0
			? 'left-0'
			: visibleColIndex === 6
				? 'right-0'
				: 'left-1/2 -translate-x-1/2'}"
	>
		<!-- Course Header -->
		<div class="flex items-center justify-between gap-2">
			<div class="flex items-center gap-1.5 min-w-0">
				<span
					class="size-2 rounded-full shrink-0"
					style="background-color: {deadline.color};"
				></span>
				<span
					class="font-semibold text-[11px] truncate"
					style="color: {deadline.color};"
				>
					{deadline.courseName}
				</span>
				{#if deadline.priority}
					<span
						class="uppercase tracking-wider text-[9px] px-1 py-0.2 bg-muted rounded font-mono"
					>
						{deadline.priority}
					</span>
				{/if}
			</div>
		</div>

		<!-- Assignment Name -->
		<p
			class="font-medium text-xs leading-snug text-foreground {deadline.status ===
			'done'
				? 'line-through text-muted-foreground opacity-75'
				: ''}"
		>
			{deadline.name}
		</p>

		<!-- Status & Priority -->
		<div
			class="flex items-center justify-between text-[10px] text-muted-foreground"
		>
			<span
				class="capitalize font-medium {deadline.status === 'done'
					? 'text-emerald-500'
					: 'text-amber-500'}"
			>
				{deadline.status === "done" ? "Completed" : deadline.dueTimeStr}
			</span>
		</div>
	</div>
</div>
