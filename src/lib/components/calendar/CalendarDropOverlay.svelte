<script lang="ts">
	import type { Snippet } from "svelte";
	import { fly } from "svelte/transition";
	import { cubicOut } from "svelte/easing";
	import { dragState } from "$lib/dragState.svelte";
	import AlertCircle from "@lucide/svelte/icons/alert-circle";
	import AlertTriangle from "@lucide/svelte/icons/alert-triangle";

	type DropOverlayMode = "default" | "warning" | "error";

	interface Props {
		/** Explicit boolean to control visibility. Defaults to dragState.isDragging if not provided. */
		show?: boolean;
		/** Optional text displayed in the overlay. Defaults to "Drop on calendar" */
		text?: string;
		/** Mode styling: default (primary), warning (amber), error (destructive) */
		mode?: DropOverlayMode;
		/** Total number of visible days in the calendar header (typically 7) */
		visibleCount?: number;
		/** Number of middle days to cover (typically 3) */
		middleCount?: number;
		/** Additional CSS classes for the container */
		class?: string;
		/** Optional snippet children to override or augment text */
		children?: Snippet;
	}

	let {
		show,
		text = "Drop task onto calendar to schedule a work event",
		mode = "default",
		visibleCount = 7,
		middleCount = 3,
		class: className = "",
		children
	}: Props = $props();

	// If `show` is explicitly passed (true/false), use it; otherwise reactively track `dragState.isDragging`
	const isVisible = $derived(show !== undefined ? show : dragState.isDragging);

	// Position calculation for middle days:
	// For visibleCount = 7, middleCount = 3 -> startIndex = (7 - 3) / 2 = 2
	const startIndex = $derived(Math.max(0, (visibleCount - middleCount) / 2));
	const leftPercent = $derived((startIndex / visibleCount) * 100);
	const widthPercent = $derived((middleCount / visibleCount) * 100);

	// Styling map for modes
	const modeStyles: Record<DropOverlayMode, { bg: string; text: string; earFill: string }> = {
		default: {
			bg: "bg-primary text-primary-foreground shadow-md shadow-primary/20",
			text: "text-primary-foreground",
			earFill: "text-primary"
		},
		warning: {
			bg: "bg-amber-500 dark:bg-amber-600 text-white shadow-md shadow-amber-500/20",
			text: "text-white",
			earFill: "text-amber-500 dark:text-amber-600"
		},
		error: {
			bg: "bg-destructive text-destructive-foreground shadow-md shadow-destructive/20",
			text: "text-destructive-foreground",
			earFill: "text-destructive"
		}
	};

	const currentStyle = $derived(modeStyles[mode] || modeStyles.default);
</script>

<div
	class="absolute top-0 z-30 pointer-events-none select-none flex items-start justify-center {className} transition-all duration-300 ease-[cubic-bezier(0.16,1,0.3,1)]"
	style="left: {leftPercent}%; width: {widthPercent}%; transform: translate3d(0, {isVisible ? '0' : '-130%'}, 0); opacity: {isVisible ? '1' : '0'};"
	aria-live="polite"
>
	<div
		class="relative flex items-center justify-center gap-2 px-4 py-1.5 rounded-b-xl font-bold text-xs tracking-wide transition-colors duration-200 {currentStyle.bg}"
	>
		<!-- Left Inverted Corner / Tab Ear -->
		<svg
			class="absolute top-0 right-full w-3.5 h-3.5 pointer-events-none {currentStyle.earFill}"
			viewBox="0 0 14 14"
			fill="none"
			xmlns="http://www.w3.org/2000/svg"
			aria-hidden="true"
		>
			<!-- Fills top-right corner curving down-left away from top edge -->
			<path d="M 0 0 L 14 0 L 14 14 A 14 14 0 0 0 0 0 Z" fill="currentColor" />
		</svg>

		<!-- Right Inverted Corner / Tab Ear -->
		<svg
			class="absolute top-0 left-full w-3.5 h-3.5 pointer-events-none {currentStyle.earFill}"
			viewBox="0 0 14 14"
			fill="none"
			xmlns="http://www.w3.org/2000/svg"
			aria-hidden="true"
		>
			<!-- Fills top-left corner curving down-right away from top edge -->
			<path d="M 14 0 L 0 0 L 0 14 A 14 14 0 0 1 14 0 Z" fill="currentColor" />
		</svg>

		<!-- Content -->
		{#if children}
			{@render children()}
		{:else}
			{#if mode === "error"}
				<AlertCircle class="size-3.5 shrink-0" />
			{:else if mode === "warning"}
				<AlertTriangle class="size-3.5 shrink-0" />
			{/if}
			<span class="truncate">{text}</span>
		{/if}
	</div>
</div>
