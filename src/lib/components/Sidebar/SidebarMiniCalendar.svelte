<script lang="ts">
	import {
		today,
		getLocalTimeZone,
		isToday,
	} from "@internationalized/date";
	import { dayState } from "$lib/dayState.svelte";
	import { fly } from "svelte/transition";

	const todayDate = $derived(today(getLocalTimeZone()));
	const monthAbbrev = $derived.by(() => {
		const d = new Date();
		return d.toLocaleDateString("en-US", { month: "short" }).toUpperCase();
	});
</script>

<div
	in:fly={{ y: 20, duration: 200 }}
	out:fly={{ y: -20, duration: 200 }}
	class="flex flex-col items-center py-2 shrink-0 gap-1 w-full select-none"
>
	<span
		class="text-[10px] font-bold text-muted-foreground uppercase tracking-widest leading-none select-none text-center"
	>
		{monthAbbrev}
	</span>
	<button
		type="button"
		class="size-10 rounded-[10px] border border-sidebar-border bg-sidebar-accent/50 hover:bg-sidebar-accent hover:border-primary/40 text-sidebar-foreground hover:text-primary font-bold text-sm flex items-center justify-center transition-all cursor-pointer shadow-2xs select-none {!isToday(
			dayState.value,
			getLocalTimeZone(),
		)
			? 'ring-1 ring-primary/40 text-primary'
			: ''}"
		onclick={() => dayState.today()}
		title="Today ({monthAbbrev} {todayDate.day}) — Click to jump to today"
		aria-label="Jump to today"
	>
		{todayDate.day}
	</button>
</div>
