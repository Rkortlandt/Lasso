<script lang="ts">
	import { onMount } from "svelte";
	import { fly } from "svelte/transition";
	import { cubicOut } from "svelte/easing";
	import X from "@lucide/svelte/icons/x";
	import RefreshCwIcon from "@lucide/svelte/icons/refresh-cw";
	import { pb } from "$lib/pocketbase";
	import CanvasContent from "$lib/components/CanvasContent.svelte";

	interface Props {
		title: string;
		side?: "left" | "right";
		itemType?: "event" | "announcement" | "deadline";
		courseId?: string;
		description?: string;
		source_link?: string;
		onClose: () => void;
	}

	let {
		title,
		side = "right",
		itemType = "event",
		courseId = "",
		description = "",
		source_link = "",
		onClose,
	}: Props = $props();

	let containerEl = $state<HTMLElement | null>(null);
	let loading = $state(false);
	let details = $state<any>(null);
	let error = $state("");

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === "Escape") {
			onClose();
		}
	}

	function handlePointerDown(e: PointerEvent) {
		const target = e.target as HTMLElement | null;

		if (target && target.closest('button[aria-label="Close"]')) {
			onClose();
			return;
		}

		if (containerEl && containerEl.contains(e.target as Node)) {
			return;
		}

		if (
			target &&
			(target.closest(".group\\/gev") ||
				target.closest(".group\\/deadline") ||
				target.closest(".group\\/announcement"))
		) {
			return;
		}

		onClose();
	}

	$effect(() => {
		// Track dependencies
		const currentTitle = title;
		const currentCourseId = courseId;
		const currentItemType = itemType;
		const currentSourceLink = source_link;

		// Reset state when a new item is selected
		details = null;
		error = "";

		if (!currentCourseId && !currentSourceLink) {
			loading = false;
			return;
		}

		if (currentItemType !== "announcement" && currentItemType !== "deadline") {
			loading = false;
			return;
		}

		loading = true;

		pb.send("/api/canvas/item", {
			method: "POST",
			headers: {
				"Content-Type": "application/json",
			},
			body: JSON.stringify({
				courseId: currentCourseId,
				title: currentTitle,
				type: currentItemType,
				sourceLink: currentSourceLink,
			}),
		})
			.then((res) => {
				if (title !== currentTitle || courseId !== currentCourseId) return;
				if (res && res.success && res.data) {
					details = res.data;
				} else {
					error = "Failed to load details.";
				}
			})
			.catch((err) => {
				if (title !== currentTitle || courseId !== currentCourseId) return;
				console.error("Error fetching canvas details:", err);
				error = "Error loading details from Canvas.";
			})
			.finally(() => {
				if (title !== currentTitle || courseId !== currentCourseId) return;
				loading = false;
			});
	});

	onMount(() => {
		const timer = setTimeout(() => {
			window.addEventListener("pointerdown", handlePointerDown);
			window.addEventListener("keydown", handleKeydown);
		}, 10);

		return () => {
			clearTimeout(timer);
			window.removeEventListener("pointerdown", handlePointerDown);
			window.removeEventListener("keydown", handleKeydown);
		};
	});

	const effectiveUrl = $derived(details?.html_url || source_link);
</script>

<div
	bind:this={containerEl}
	in:fly={{
		x: side === "left" ? -120 : 120,
		duration: 260,
		easing: cubicOut,
	}}
	out:fly={{
		x: side === "left" ? -120 : 120,
		duration: 200,
		easing: cubicOut,
	}}
	class="absolute top-[5.75rem] bottom-4 z-[60] flex flex-col pointer-events-auto rounded-xl bg-muted/70 dark:bg-muted/50 backdrop-blur-md border border-border/60 shadow-xl overflow-hidden {side ===
	'left'
		? 'left-[4.25rem]'
		: 'right-4'}"
	style="width: calc((100% - 7.5rem) * (3 / 7));"
	role="dialog"
	aria-modal="false"
	aria-label="Item details"
>
	<!-- Header matching all-day bar aesthetics -->
	<div
		class="h-10 px-3 flex items-center justify-between gap-2 border-b border-border/40 shrink-0 bg-background/30 min-w-0"
	>
		<span
			class="text-[11px] font-semibold tracking-wider text-muted-foreground uppercase truncate"
			{title}
		>
			{title}
		</span>
		<button
			type="button"
			onclick={onClose}
			class="size-6 rounded-md flex items-center justify-center text-muted-foreground hover:text-foreground hover:bg-muted/80 transition-colors cursor-pointer shrink-0"
			aria-label="Close"
		>
			<X class="size-3.5" />
		</button>
	</div>

	<!-- Content area -->
	<div class="p-4 flex-1 overflow-y-auto">
		{#if loading}
			<div class="flex items-center gap-2 text-xs text-muted-foreground">
				<RefreshCwIcon class="size-3 animate-spin" />
				<span>Loading details from Canvas...</span>
			</div>
		{:else if error}
			<div class="text-xs text-destructive/80">
				{error}
			</div>
			{#if effectiveUrl}
				<div class="pt-3">
					<a
						href={effectiveUrl}
						target="_blank"
						rel="noopener noreferrer"
						class="text-xs text-primary hover:underline font-medium"
					>
						View on Canvas
					</a>
				</div>
			{/if}
		{:else if details}
			<div class="text-sm text-foreground/90 space-y-3 pb-4">
				{#if itemType === "announcement" && details.message}
					<CanvasContent html={details.message} />
				{:else if itemType === "deadline" && details.description}
					<CanvasContent html={details.description} />
				{:else}
					<div
						class="text-xs text-muted-foreground italic bg-muted/40 p-3 rounded-md border border-border/40"
					>
						No additional details provided for this {itemType === "announcement"
							? "announcement"
							: "assignment"}.
					</div>
				{/if}

				{#if effectiveUrl}
					<div class="pt-2">
						<a
							href={effectiveUrl}
							target="_blank"
							rel="noopener noreferrer"
							class="text-xs text-primary hover:underline font-medium"
						>
							View on Canvas
						</a>
					</div>
				{/if}
			</div>
		{:else if description}
			{#if description.includes("<") && description.includes(">")}
				<CanvasContent html={description} />
			{:else}
				<div
					class="text-xs text-muted-foreground bg-muted/40 p-3 rounded-md border border-border/40 whitespace-pre-wrap"
				>
					{description}
				</div>
			{/if}
			{#if effectiveUrl}
				<div class="pt-3">
					<a
						href={effectiveUrl}
						target="_blank"
						rel="noopener noreferrer"
						class="text-xs text-primary hover:underline font-medium"
					>
						View on Canvas
					</a>
				</div>
			{/if}
		{:else}
			<div
				class="text-xs text-muted-foreground italic bg-muted/40 p-3 rounded-md border border-border/40"
			>
				No additional details provided.
			</div>
			{#if effectiveUrl}
				<div class="pt-3">
					<a
						href={effectiveUrl}
						target="_blank"
						rel="noopener noreferrer"
						class="text-xs text-primary hover:underline font-medium"
					>
						View on Canvas
					</a>
				</div>
			{/if}
		{/if}
	</div>
</div>
