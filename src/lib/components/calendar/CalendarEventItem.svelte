<script lang="ts">
	import X from "@lucide/svelte/icons/x";
	import type { FormattedTimedEvent } from "./calendarTypes";

	interface Props {
		event: FormattedTimedEvent;
		visibleColIndex: number;
		onSelect?: (event: FormattedTimedEvent, visibleColIndex: number) => void;
		onDelete?: (event: FormattedTimedEvent) => void;
	}

	let { event, visibleColIndex, onSelect, onDelete }: Props = $props();

	const isCompletedWorkSession = $derived(
		Boolean(event.isTaskBlock && event.isTaskDone),
	);
</script>

<div
	role="button"
	tabindex="0"
	class="absolute pointer-events-auto group/gev select-none cursor-pointer overflow-visible z-10 hover:z-50 text-left"
	style="
		left: calc({event.leftPercent}% + 1.5px);
		width: calc({event.widthPercent}% - 3px);
		top: {event.topPercent}%;
		height: {event.heightPercent}%;
		min-height: 20px;
	"
	onclick={(e) => {
		e.stopPropagation();
		onSelect?.(event, visibleColIndex);
	}}
	onkeydown={(e) => {
		if (e.key === "Enter" || e.key === " ") {
			e.stopPropagation();
			onSelect?.(event, visibleColIndex);
		}
	}}
>
	<!-- Inner event pill -->
	<div
		class="w-full h-full rounded-md px-1.5 py-0.5 overflow-hidden transition-all duration-150 border text-[10px] font-medium leading-tight flex flex-col justify-start hover:ring-1 hover:ring-primary/40 shadow-xs relative border-3"
		style="
			background-color: {isCompletedWorkSession
			? `color-mix(in srgb, ${event.calendarColor || event.color || '#3b82f6'} 70%, black)`
			: event.color || '#3b82f6'};
			border-color: {isCompletedWorkSession
			? event.calendarColor || event.color || '#3b82f6'
			: event.color || '#3b82f6'};
			color: white;
		"
	>
		<!-- Line 1: Title & Delete button for work sessions -->
		<div class="flex items-center justify-between gap-1 min-w-0 w-full">
			<span
				class="truncate font-semibold text-[10px] text-white drop-shadow-xs leading-tight flex-1 min-w-0"
			>
				{event.title}
			</span>
		</div>
		<div class="flex items-center justify-between gap-1 min-w-0 w-full">
			<!-- Line 2: Time on new line if big enough (omitted if small) -->
			{#if event.heightPercent >= 3.0 && event.widthPercent >= 28}
				<span
					class="truncate text-[9px] text-white font-medium leading-tight mt-0.5"
				>
					{event.widthPercent > 60
						? event.timeStr
						: event.timeStr.split("-")[0].trim()}
				</span>
			{/if}
			{#if event.isTaskBlock}
				<button
					type="button"
					class="opacity-0 group-hover/gev:opacity-100 hover:bg-black/30 rounded size-3.5 flex items-center justify-center text-white/90 hover:text-white transition-all cursor-pointer shrink-0 z-20"
					title="Delete work session"
					onclick={(e) => {
						e.stopPropagation();
						onDelete?.(event);
					}}
				>
					<X class="size-2.5" />
				</button>
			{/if}
		</div>
	</div>

	<!-- Hover Details Card (solid, zero transparency) -->
	<div
		class="absolute hidden group-hover/gev:flex flex-col gap-1.5 z-[100] pointer-events-none p-2.5 rounded-lg bg-popover border border-border shadow-2xl text-popover-foreground text-xs min-w-[260px] max-w-[300px] animate-in fade-in zoom-in-95 duration-150 {event.topPercent >
		70
			? 'bottom-full mb-2'
			: 'top-full mt-2'} {visibleColIndex === 0
			? 'left-0'
			: visibleColIndex === 6
				? 'right-0'
				: 'left-1/2 -translate-x-1/2'}"
	>
		<div
			class="flex items-center justify-between gap-1.5 font-semibold text-[11px]"
			style="color: {event.color};"
		>
			<div class="flex items-center gap-1.5 min-w-0 truncate">
				<span
					class="size-2 rounded-full shrink-0"
					style="background-color: {event.color};"
				></span>
				<span class="truncate">{event.calendarName}</span>
			</div>

			{#if event.source === "google"}
				<span
					class="text-[9px] px-1 py-0.5 rounded bg-muted text-muted-foreground shrink-0 font-mono"
					>Google</span
				>
			{:else if event.source === "canvas"}
				<span
					class="text-[9px] px-1 py-0.5 rounded bg-muted text-muted-foreground shrink-0 font-mono"
					>Canvas</span
				>
			{/if}
		</div>
		<p class="font-medium text-xs text-foreground leading-snug">
			{event.title}
		</p>
		<div
			class="flex items-center justify-between gap-1 text-[10px] text-muted-foreground font-mono"
		>
			<span>{event.timeStr}</span>
			{#if event.isTaskBlock}
				<button
					type="button"
					class="text-[10px] text-destructive hover:underline cursor-pointer flex items-center gap-0.5"
					onclick={(e) => {
						e.stopPropagation();
						onDelete?.(event);
					}}
				>
					Delete
				</button>
			{/if}
		</div>
		{#if event.description}
			<p
				class="text-[10px] text-muted-foreground border-t border-border/40 pt-1 line-clamp-3 leading-relaxed whitespace-pre-line"
			>
				{event.description}
			</p>
		{/if}
	</div>
</div>
