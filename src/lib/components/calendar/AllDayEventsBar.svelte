<script lang="ts">
	import RefreshCwIcon from "@lucide/svelte/icons/refresh-cw";
	import Button from "$lib/components/ui/button/button.svelte";
	import CalendarAllDayItem from "./CalendarAllDayItem.svelte";
	import type { DateValue } from "@internationalized/date";
	import type { DayAllDayEvent, DayItem } from "./calendarTypes";

	interface Props {
		days: DayItem[];
		totalCount: number;
		visibleCount: number;
		transformPercent: number;
		isAnimating: boolean;
		isSyncing: boolean;
		getAllDayEventsForDate: (date: DateValue) => DayAllDayEvent[];
		onSync: () => void;
		onSelectEvent?: (title: string, visibleColIndex: number) => void;
	}

	let {
		days,
		totalCount,
		visibleCount,
		transformPercent,
		isAnimating,
		isSyncing,
		getAllDayEventsForDate,
		onSync,
		onSelectEvent,
	}: Props = $props();
</script>

<div
	class="absolute top-14 left-0 right-0 z-30 flex gap-2 px-2 -translate-y-1/4 pointer-events-none"
>
	<!-- Left: All-day label aligned with time column -->
	<div class="w-10 shrink-0 flex items-center justify-end pr-1 select-none">
		<span
			class="text-[9px] font-medium text-muted-foreground/60 uppercase tracking-tighter"
		>
			all-day
		</span>
	</div>

	<!-- Middle: 7-column neutral rounded box matching the 7 visible days -->
	<div
		class="flex-1 pointer-events-auto h-8 rounded-lg bg-muted/50 dark:bg-muted/35 backdrop-blur-md border border-border/50 shadow-xs overflow-hidden relative"
	>
		<div
			class="h-full flex flex-row"
			style="width: calc({totalCount} / {visibleCount} * 100%); transform: translate3d(-{transformPercent}%, 0, 0); {isAnimating
				? 'transition: transform 370ms cubic-bezier(0.16, 1, 0.3, 1);'
				: 'transition: none;'}"
		>
			{#each days as item, index (item.date.toString())}
				{@const dayEvents = getAllDayEventsForDate(item.date)}
				<div
					style="flex: 0 0 calc(100% / {totalCount});"
					class="h-full border-r border-border/30 last:border-r-0 flex flex-col overflow-y-auto overflow-x-hidden [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
				>
					{#each dayEvents as evt (evt.id)}
						<CalendarAllDayItem
							event={evt}
							colIndex={index}
							onSelect={onSelectEvent}
						/>
					{/each}
				</div>
			{/each}
		</div>
	</div>

	<!-- Right: Outline sync button in empty space -->
	<div
		class="w-10 shrink-0 flex items-center justify-center pointer-events-auto z-40"
	>
		<Button
			variant="outline"
			size="icon"
			class="size-8 rounded-lg border border-border bg-background hover:bg-primary/10 hover:border-primary/50 text-foreground hover:text-primary cursor-pointer shadow-xs transition-all"
			onclick={onSync}
			disabled={isSyncing}
			title={isSyncing
				? "Syncing Canvas & Google Calendar..."
				: "Sync Canvas & Google Calendar"}
			aria-label="Sync Canvas & Google Calendar"
		>
			<RefreshCwIcon
				class="size-3.5 {isSyncing ? 'animate-spin text-primary' : ''}"
			/>
		</Button>
	</div>
</div>
