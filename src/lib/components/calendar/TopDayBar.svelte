<script lang="ts">
	import ChevronLeft from "@lucide/svelte/icons/chevron-left";
	import ChevronRight from "@lucide/svelte/icons/chevron-right";
	import Button from "$lib/components/ui/button/button.svelte";
	import { dayState } from "$lib/dayState.svelte";
	import { errorState } from "$lib/errorState.svelte";
	import { dragState } from "$lib/dragState.svelte";
	import type { DayItem } from "./calendarTypes";
	import CalendarDropOverlay from "./CalendarDropOverlay.svelte";

	interface Props {
		days: DayItem[];
		totalCount: number;
		visibleCount: number;
		transformPercent: number;
		isAnimating: boolean;
		onTransitionEnd: (e: TransitionEvent) => void;
	}

	let {
		days,
		totalCount,
		visibleCount,
		transformPercent,
		isAnimating,
		onTransitionEnd,
	}: Props = $props();

	const isOverlayVisible = $derived(errorState.isVisible || dragState.isDragging);
	const overlayMode = $derived(errorState.currentError ? errorState.mode : "default");
	const overlayMiddleCount = $derived(errorState.currentError ? 4 : 3);
	const overlayText = $derived(
		dragState.isDragging && !errorState.isVisible
			? "Drop task onto calendar to schedule a work event"
			: (errorState.currentError || "Drop task onto calendar to schedule a work event")
	);
</script>

<header class="flex h-14 items-center gap-2 px-2 shrink-0 z-10">
	<div class="w-10 shrink-0 flex items-center justify-center">
		<Button
			variant="ghost"
			size="icon"
			class="size-8 rounded-md text-muted-foreground hover:text-foreground hover:bg-muted cursor-pointer shrink-0"
			aria-label="Previous day"
			onclick={() => dayState.dec()}
		>
			<ChevronLeft class="size-4" />
		</Button>
	</div>

	<div class="flex-1 h-full overflow-hidden relative">
		<CalendarDropOverlay
			{visibleCount}
			middleCount={overlayMiddleCount}
			show={isOverlayVisible}
			mode={overlayMode}
			text={overlayText}
		/>

		<div
			class="h-full flex flex-row items-center"
			style="width: calc({totalCount} / {visibleCount} * 100%); transform: translate3d(-{transformPercent}%, 0, 0); {isAnimating
				? 'transition: transform 370ms cubic-bezier(0.16, 1, 0.3, 1);'
				: 'transition: none;'}"
			ontransitionend={onTransitionEnd}
		>
			{#each days as item (item.date.toString())}
				<button
					type="button"
					style="flex: 0 0 calc(100% / {totalCount});"
					class="h-full flex flex-row items-center justify-center gap-2 text-center cursor-pointer select-none"
					onclick={() => dayState.set(item.date)}
					aria-label="{item.weekday}, {item.monthName} {item.dayNumber}"
					aria-current={item.isSelected ? "date" : undefined}
				>
					<span
						class="text-xs uppercase font-medium tracking-wide {item.isSelected
							? 'text-foreground font-semibold'
							: 'text-muted-foreground'}"
					>
						{item.weekday}
					</span>

					<div class="relative size-7 flex items-center justify-center">
						<!-- Expanding highlight from center -->
						<span
							class="absolute inset-0 rounded-md bg-primary shadow-xs origin-center pointer-events-none transition-all duration-250 ease-out {item.isSelected
								? 'scale-100 opacity-100'
								: 'scale-0 opacity-0'}"
						></span>

						<!-- Day number text -->
						<span
							class="relative z-10 text-sm font-semibold transition-colors duration-250 {item.isSelected
								? 'text-primary-foreground'
								: 'text-foreground'}"
						>
							{item.dayNumber}
						</span>

						{#if item.isToday}
							<span
								class="size-1 rounded-full absolute -top-1.5 left-1/2 -translate-x-1/2 z-10 transition-colors duration-150 {item.isSelected
									? 'bg-primary ring-1 ring-background'
									: 'bg-primary'}"
								title="Today"
							></span>
						{/if}
					</div>
				</button>
			{/each}
		</div>
	</div>

	<div class="w-10 shrink-0 flex items-center justify-center">
		<Button
			variant="ghost"
			size="icon"
			class="size-8 rounded-md text-muted-foreground hover:text-foreground hover:bg-muted cursor-pointer shrink-0"
			aria-label="Next day"
			onclick={() => dayState.inc()}
		>
			<ChevronRight class="size-4" />
		</Button>
	</div>
</header>
