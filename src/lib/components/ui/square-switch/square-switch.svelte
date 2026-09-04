<script lang="ts">
	import { Switch as SwitchPrimitive } from "bits-ui";
	import { cn, type WithoutChildrenOrChild } from "$lib/utils.js";
	import CheckIcon from "@lucide/svelte/icons/check";

	const autoId = $props.id();

	let {
		ref = $bindable(null),
		class: className,
		checked = $bindable(false),
		size = "default",
		label,
		labelPlacement = "left",
		labelClass,
		id,
		disabled = false,
		...restProps
	}: WithoutChildrenOrChild<SwitchPrimitive.RootProps> & {
		size?: "sm" | "default";
		label?: string;
		labelPlacement?: "left" | "right";
		labelClass?: string;
	} = $props();

	const switchId = $derived(id || autoId);
</script>

{#snippet switchRoot()}
	<SwitchPrimitive.Root
		bind:ref
		bind:checked
		id={switchId}
		{disabled}
		data-slot="square-switch"
		data-size={size}
		class={cn(
			"peer group/switch relative inline-flex shrink-0 items-center justify-start rounded-md border transition-all duration-200 outline-none cursor-pointer select-none",
			// Default sizing
			"data-[size=default]:h-6 data-[size=default]:w-11 data-[size=default]:p-0.5",
			// Small sizing
			"data-[size=sm]:h-5 data-[size=sm]:w-9 data-[size=sm]:p-0.5 data-[size=sm]:rounded-sm",
			// Outline theme - unchecked
			"border-border/80 bg-background/50 hover:border-foreground/30 hover:bg-muted/30",
			// Outline theme - checked
			"data-[state=checked]:border-primary data-[state=checked]:bg-primary/10 data-[state=checked]:hover:bg-primary/15 data-[state=checked]:ring-1 data-[state=checked]:ring-primary/20",
			// Focus outline
			"focus-visible:border-primary focus-visible:ring-2 focus-visible:ring-primary/30 focus-visible:ring-offset-1",
			// Disabled
			"data-disabled:cursor-not-allowed data-disabled:opacity-50",
			className,
		)}
		{...restProps}
	>
		<SwitchPrimitive.Thumb
			data-slot="square-switch-thumb"
			class={cn(
				"pointer-events-none flex items-center justify-center rounded-sm border transition-all duration-200 ease-out",
				// Default thumb size
				"group-data-[size=default]/switch:size-4.5",
				// Small thumb size
				"group-data-[size=sm]/switch:size-3.5 group-data-[size=sm]/switch:rounded-[2px]",
				// Unchecked thumb state
				"data-[state=unchecked]:translate-x-0 data-[state=unchecked]:border-border/70 data-[state=unchecked]:bg-muted/70 data-[state=unchecked]:text-muted-foreground/60",
				// Checked thumb state
				"data-[state=checked]:border-primary data-[state=checked]:bg-primary data-[state=checked]:text-primary-foreground data-[state=checked]:shadow-xs",
				"group-data-[size=default]/switch:data-[state=checked]:translate-x-5",
				"group-data-[size=sm]/switch:data-[state=checked]:translate-x-4",
				"rtl:data-[state=checked]:-translate-x-5",
			)}
		>
			<span class="size-full flex items-center justify-center">
				{#if checked}{:else}
					<span class="size-1 rounded-[1px] bg-muted-foreground/40"></span>
				{/if}
			</span>
		</SwitchPrimitive.Thumb>
	</SwitchPrimitive.Root>
{/snippet}

{#if label}
	<div class="inline-flex items-center gap-2 select-none">
		{#if labelPlacement === "left"}
			<label
				for={switchId}
				class={cn(
					"text-xs font-medium cursor-pointer select-none transition-colors text-muted-foreground hover:text-foreground",
					disabled ? "cursor-not-allowed opacity-50" : "",
					labelClass,
				)}
			>
				{label}
			</label>
		{/if}

		{@render switchRoot()}

		{#if labelPlacement === "right"}
			<label
				for={switchId}
				class={cn(
					"text-xs font-medium cursor-pointer select-none transition-colors",
					checked
						? "text-foreground font-semibold"
						: "text-muted-foreground hover:text-foreground",
					disabled ? "cursor-not-allowed opacity-50" : "",
					labelClass,
				)}
			>
				{label}
			</label>
		{/if}
	</div>
{:else}
	{@render switchRoot()}
{/if}
