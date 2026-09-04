<script lang="ts">
	import PaletteIcon from "@lucide/svelte/icons/palette";
	import SunIcon from "@lucide/svelte/icons/sun";
	import MoonIcon from "@lucide/svelte/icons/moon";
	import LaptopIcon from "@lucide/svelte/icons/laptop";
	import { themeState, ACCENT_PALETTE } from "$lib/themeState.svelte";
</script>

<!-- Appearance section -->
<div class="rounded-xl border border-border bg-card p-6 shadow-xs">
	<div class="flex items-center justify-between">
		<div class="flex items-center gap-3">
			<h2
				class="text-base font-normal tracking-wide text-card-foreground leading-none"
			>
				Theme & Appearance
			</h2>
		</div>
	</div>

	<div class="mt-4 pt-4 border-t border-border space-y-5">
		<!-- Light / Dark / System Mode Switcher -->
		<div
			class="flex flex-col sm:flex-row sm:items-center justify-between gap-3"
		>
			<div>
				<div class="text-xs font-medium text-foreground">Color Mode</div>
				<div class="text-[11px] text-muted-foreground">
					Select your interface visual appearance
				</div>
			</div>

			<div
				class="flex items-center bg-muted/60 p-1 rounded-lg text-xs gap-1 border border-border/40"
			>
				<button
					type="button"
					class="flex items-center gap-1.5 px-2.5 py-1 rounded-md font-medium transition-all cursor-pointer {themeState.mode ===
					'light'
						? 'bg-background text-foreground shadow-xs font-semibold'
						: 'text-muted-foreground hover:text-foreground'}"
					onclick={() => themeState.setMode("light")}
				>
					<SunIcon class="size-3.5" />
					<span>Light</span>
				</button>

				<button
					type="button"
					class="flex items-center gap-1.5 px-2.5 py-1 rounded-md font-medium transition-all cursor-pointer {themeState.mode ===
					'dark'
						? 'bg-background text-foreground shadow-xs font-semibold'
						: 'text-muted-foreground hover:text-foreground'}"
					onclick={() => themeState.setMode("dark")}
				>
					<MoonIcon class="size-3.5" />
					<span>Dark</span>
				</button>

				<button
					type="button"
					class="flex items-center gap-1.5 px-2.5 py-1 rounded-md font-medium transition-all cursor-pointer {themeState.mode ===
					'system'
						? 'bg-background text-foreground shadow-xs font-semibold'
						: 'text-muted-foreground hover:text-foreground'}"
					onclick={() => themeState.setMode("system")}
				>
					<LaptopIcon class="size-3.5" />
					<span>System</span>
				</button>
			</div>
		</div>

		<!-- Accent Color Switcher -->
		<div
			class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pt-3 border-t border-border/40"
		>
			<div>
				<div class="text-xs font-medium text-foreground">Accent Color</div>
				<div class="text-[11px] text-muted-foreground">
					Customize primary highlights and buttons
				</div>
			</div>

			<div class="flex items-center gap-2 flex-wrap">
				{#each ACCENT_PALETTE as accent}
					{@const isSelected = themeState.accentId === accent.id}
					<button
						type="button"
						class="relative size-6 rounded-full transition-all duration-150 cursor-pointer flex items-center justify-center {isSelected
							? 'scale-115 ring-2 ring-foreground/50 ring-offset-2 ring-offset-background'
							: 'hover:scale-105 opacity-85 hover:opacity-100'}"
						style="background-color: {accent.swatch};"
						onclick={() => themeState.setAccent(accent.id)}
						title="{accent.name} accent"
						aria-label="Select {accent.name} accent"
					>
						{#if isSelected}
							<span class="size-2 rounded-full bg-white shadow-xs"></span>
						{/if}
					</button>
				{/each}
			</div>
		</div>

		<!-- Calendar Past Days Offset Control -->
		<div
			class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pt-3 border-t border-border/40"
		>
			<div>
				<div class="text-xs font-medium text-foreground">Past Days Offset</div>
				<div class="text-[11px] text-muted-foreground">
					Number of days in the past shown on the calendar grid
				</div>
			</div>

			<div
				class="flex items-center bg-muted/60 p-1 rounded-lg text-xs gap-1 border border-border/40"
			>
				{#each [0, 1, 2, 3] as count}
					<button
						type="button"
						class="flex items-center gap-1 px-2.5 py-1 rounded-md font-medium transition-all cursor-pointer {themeState.calendarOffset ===
						count
							? 'bg-background text-foreground shadow-xs font-semibold'
							: 'text-muted-foreground hover:text-foreground'}"
						onclick={() => themeState.setCalendarOffset(count)}
					>
						<span>{count} {count === 1 ? "day" : "days"}</span>
						{#if count === 1}
							<span class="text-[9px] text-muted-foreground font-normal"
								>(Default)</span
							>
						{/if}
					</button>
				{/each}
			</div>
		</div>
	</div>
</div>

<style>
	h2 {
		margin: 0px;
	}
</style>
