<script lang="ts">
	import MegaphoneIcon from "@lucide/svelte/icons/megaphone";
	import type { FormattedAnnouncement } from "./calendarTypes";
	import { cleanCanvasText } from "$lib/canvasCleaner";

	interface Props {
		announcement: FormattedAnnouncement;
		visibleColIndex: number;
		onSelect?: (
			announcement: FormattedAnnouncement,
			visibleColIndex: number,
		) => void;
	}

	let { announcement, visibleColIndex, onSelect }: Props = $props();
</script>

<div
	role="button"
	tabindex="0"
	class="absolute pointer-events-auto group/announcement select-none flex items-center cursor-pointer z-10 hover:z-50 overflow-visible text-left {announcement.isEndOfDay
		? 'py-2 items-end'
		: 'py-2.5 items-center'}"
	style="
		left: calc({announcement.leftPercent}% + 1.5px);
		width: calc({announcement.widthPercent}% - 3px);
		{announcement.isEndOfDay
		? 'bottom: 0px;'
		: `top: ${announcement.topPercent}%; transform: translateY(-50%);`}
	"
	onclick={(e) => {
		e.stopPropagation();
		onSelect?.(announcement, visibleColIndex);
	}}
	onkeydown={(e) => {
		if (e.key === "Enter" || e.key === " ") {
			e.stopPropagation();
			onSelect?.(announcement, visibleColIndex);
		}
	}}
>
	<!-- Left line: thinner than deadline on sides (h-[2px] vs h-1) -->
	<div
		class="flex-1 h-[2px] rounded-full transition-opacity duration-150 opacity-80 group-hover/announcement:opacity-100"
		style="
			background-color: {announcement.color};
			box-shadow: 0 0 4px {announcement.color}70, 0 1px 2px rgba(0,0,0,0.1);
		"
	></div>

	<!-- Middle pill with megaphone icon -->
	<div
		class="px-2 py-0.5 rounded-full flex items-center justify-center gap-1 shadow-xs transition-transform duration-150 group-hover/announcement:scale-110 shrink-0 z-10 mx-1"
		style="
			background-color: {announcement.color};
			box-shadow: 0 0 6px {announcement.color}80, 0 1px 2px rgba(0,0,0,0.2);
		"
	>
		<MegaphoneIcon
			class="size-2.5 sm:size-3 text-white fill-white/20"
		/>
	</div>

	<!-- Right line: thinner than deadline on sides (h-[2px] vs h-1) -->
	<div
		class="flex-1 h-[2px] rounded-full transition-opacity duration-150 opacity-80 group-hover/announcement:opacity-100"
		style="
			background-color: {announcement.color};
			box-shadow: 0 0 4px {announcement.color}70, 0 1px 2px rgba(0,0,0,0.1);
		"
	></div>

	<!-- Hover Details Card -->
	<div
		class="absolute hidden group-hover/announcement:flex flex-col gap-1.5 z-[100] pointer-events-none p-2.5 rounded-lg bg-popover border border-border shadow-2xl text-popover-foreground text-xs min-w-[200px] max-w-[260px] animate-in fade-in zoom-in-95 duration-150
			{announcement.isEndOfDay || announcement.topPercent > 70 ? 'bottom-full mb-2' : 'top-full mt-2'}
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
					style="background-color: {announcement.color};"
				></span>
				<span
					class="font-semibold text-[11px] truncate"
					style="color: {announcement.color};"
				>
					{announcement.courseName}
				</span>
			</div>
			<span
				class="text-[10px] text-muted-foreground font-mono shrink-0"
			>
				{announcement.timeStr}
			</span>
		</div>

		<!-- Announcement Title -->
		<p class="font-medium text-xs leading-snug text-foreground">
			{announcement.title}
		</p>

		{#if announcement.description}
			<p
				class="text-[11px] text-muted-foreground line-clamp-2 leading-relaxed"
			>
				{cleanCanvasText(announcement.description)}
			</p>
		{/if}

		<!-- Status & Tag Footer -->
		<div
			class="flex items-center justify-between pt-1 border-t border-border/40 text-[10px] text-muted-foreground"
		>
			<span
				class="inline-flex items-center gap-1 font-medium text-primary"
			>
				<MegaphoneIcon class="size-3 text-primary" />
				Announcement
			</span>
		</div>
	</div>
</div>
