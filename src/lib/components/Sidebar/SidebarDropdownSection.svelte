<script lang="ts">
	import ChevronUp from "@lucide/svelte/icons/chevron-up";
	import { slide } from "svelte/transition";

	interface Props {
		title?: string;
		storageKey?: string;
		defaultOpen?: boolean;
		open?: boolean;
		collapsed?: boolean;
		contentClass?: string;
		children?: import("svelte").Snippet;
	}

	let {
		title = "",
		storageKey,
		defaultOpen = true,
		open = $bindable(),
		collapsed = false,
		contentClass = "",
		children,
	}: Props = $props();

	let internalOpen = $state<boolean | null>(null);

	let isOpen = $derived.by(() => {
		if (open !== undefined) return open;
		if (internalOpen !== null) return internalOpen;
		if (typeof window !== "undefined" && storageKey) {
			return localStorage.getItem(storageKey) !== "false";
		}
		return defaultOpen;
	});

	function toggle() {
		const next = !isOpen;
		if (open !== undefined) {
			open = next;
		} else {
			internalOpen = next;
		}
		if (storageKey && typeof window !== "undefined") {
			localStorage.setItem(storageKey, String(next));
		}
	}
</script>

{#if collapsed}
	<div class="w-[34px] flex flex-col items-center gap-1.5 shrink-0">
		<button
			type="button"
			onclick={toggle}
			class="size-5 flex items-center justify-center rounded-full hover:bg-sidebar-accent/80 text-muted-foreground hover:text-foreground cursor-pointer transition-colors"
			aria-label={isOpen ? `Close ${title}` : `Open ${title}`}
			title={isOpen ? `Close ${title}` : `Open ${title}`}
		>
			<ChevronUp
				class="size-3 transition-transform duration-200 {isOpen
					? ''
					: 'rotate-180'}"
			/>
		</button>

		{#if isOpen}
			<div
				transition:slide={{ duration: 150 }}
				class="w-[34px] flex flex-col items-center gap-1.5 {contentClass}"
			>
				{@render children?.()}
			</div>
		{/if}
	</div>
{:else}
	<div class="w-full">
		<div class="relative flex w-full my-1 items-center">
			<button
				type="button"
				onclick={toggle}
				class="shrink-0 cursor-pointer text-left"
			>
				<span
					class="text-[10px] pr-1 font-semibold uppercase tracking-wider text-muted-foreground hover:text-foreground transition-colors"
				>
					{title}
				</span>
			</button>
			<div class="w-full h-full border-t border-sidebar-border"></div>
			<div class="w-8 flex items-center justify-center shrink-0">
				<button
					type="button"
					onclick={toggle}
					class="relative flex items-center justify-center rounded-full border border-sidebar-border bg-sidebar-accent text-sidebar-foreground hover:bg-sidebar-accent/80 hover:text-primary transition-all cursor-pointer shadow-2xs h-4.5 px-2 gap-1"
					aria-label={isOpen ? `Close ${title}` : `Open ${title}`}
					title={isOpen ? `Close ${title}` : `Open ${title}`}
				>
					<ChevronUp
						class="size-2.5 transition-transform duration-200 {isOpen
							? ''
							: 'rotate-180'}"
					/>
				</button>
			</div>
		</div>
	</div>

	{#if isOpen}
		<div transition:slide={{ duration: 200 }} class={contentClass}>
			{@render children?.()}
		</div>
	{/if}
{/if}
