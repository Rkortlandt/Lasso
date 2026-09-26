<script lang="ts">
	import type { DayAllDayEvent } from "./calendarTypes";

	interface Props {
		event: DayAllDayEvent;
		colIndex: number;
		onSelect?: (title: string, colIndex: number) => void;
	}

	let { event, colIndex, onSelect }: Props = $props();
</script>

<div class="h-1/2 w-full shrink-0 px-1 py-[1px] box-border">
	<button
		type="button"
		class="h-full w-full rounded-full px-2 flex items-center justify-start gap-1 truncate text-[9px] font-medium border select-none cursor-pointer transition-all shadow-2xs"
		style="
			background-color: {event.color || '#3b82f6'};
			border-color: {event.color || '#3b82f6'};
		"
		title={event.title}
		onclick={(e) => {
			e.stopPropagation();
			onSelect?.(event.title, colIndex);
		}}
	>
		<span
			class="truncate leading-none text-white drop-shadow-xs"
			style="font-weight: 600;">{event.title}</span
		>
	</button>
</div>
