<script lang="ts">
	import { pageState } from "$lib/pageSystem.svelte";
	import { Button } from "$lib/components/ui/button";
	import ArrowLeftIcon from "@lucide/svelte/icons/arrow-left";
	import CanvasCard from "$lib/components/settings/CanvasCard.svelte";
	import GoogleCalendarCard from "$lib/components/settings/GoogleCalendarCard.svelte";
	import SyncCard from "$lib/components/settings/SyncCard.svelte";
	import ThemeCard from "$lib/components/settings/ThemeCard.svelte";
	import AccountProfileCard from "$lib/components/settings/AccountProfileCard.svelte";
	import { fly, fade } from "svelte/transition";
</script>

<div class="absolute inset-0 flex flex-col overflow-hidden">
	<!-- Top header bar that slides in and out from the top -->
	<header
		class="flex h-14 items-center gap-3 border-b border-border px-6 shrink-0 z-10 bg-sidebar"
		transition:fly={{ y: -56, duration: 300 }}
	>
		<Button
			variant="ghost"
			size="icon"
			class="size-8 rounded-md text-muted-foreground hover:text-foreground hover:bg-muted cursor-pointer shrink-0"
			onclick={() => pageState.goBack()}
			aria-label="Back to home"
		>
			<ArrowLeftIcon class="size-4.5" />
		</Button>
		<h2
			class="text-base font-normal tracking-wide text-foreground leading-none"
		>
			Settings
		</h2>
	</header>

	<!-- Page content body that fades in after the header slides in -->
	<div
		class="flex-1 overflow-y-auto p-6 md:p-8 w-full"
		in:fade={{ duration: 300, delay: 200 }}
		out:fade={{ duration: 150 }}
	>
		<div class="max-w-3xl mx-auto space-y-6">
			<SyncCard />
			<CanvasCard />
			<GoogleCalendarCard />
			<ThemeCard />
			<AccountProfileCard />
		</div>
	</div>
</div>

<style>
	h2 {
		margin: 0px;
	}
</style>
