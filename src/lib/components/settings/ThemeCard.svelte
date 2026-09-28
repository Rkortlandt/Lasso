<script lang="ts">
	import PaletteIcon from "@lucide/svelte/icons/palette";
	import SunIcon from "@lucide/svelte/icons/sun";
	import MoonIcon from "@lucide/svelte/icons/moon";
	import LaptopIcon from "@lucide/svelte/icons/laptop";
	import { slide } from "svelte/transition";
	import { SquareSwitch } from "$lib/components/ui/square-switch";
	import { themeState, ACCENT_PALETTE } from "$lib/themeState.svelte";

	const isCustomSelected = $derived(themeState.accentId === "custom");
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
		<div class="pt-3 border-t border-border/40 space-y-3">
			<div
				class="flex flex-col sm:flex-row sm:items-center justify-between gap-3"
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

					<!-- Custom Hue Option Swatch -->
					<button
						type="button"
						class="relative size-6 rounded-full transition-all duration-150 cursor-pointer flex items-center justify-center {isCustomSelected
							? 'scale-115 ring-2 ring-foreground/50 ring-offset-2 ring-offset-background'
							: 'hover:scale-105 opacity-85 hover:opacity-100'}"
						style="background: {isCustomSelected
							? `oklch(0.62 0.22 ${themeState.customHue})`
							: 'conic-gradient(from 90deg, #f43f5e, #f59e0b, #10b981, #06b6d4, #3b82f6, #6366f1, #a855f7, #f43f5e)'};"
						onclick={() => themeState.setAccent("custom")}
						title="Custom Hue Accent"
						aria-label="Select custom accent color"
					>
						{#if isCustomSelected}
							<span class="size-2 rounded-full bg-white shadow-xs"></span>
						{/if}
					</button>
				</div>
			</div>

			<!-- Interactive Hue Spectrum Slider (revealed when Custom is active) -->
			{#if themeState.accentId === "custom"}
				<div
					transition:slide={{ duration: 180 }}
					class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 bg-muted/40 p-2.5 rounded-lg border border-border/40"
				>
					<div class="flex items-center gap-2 min-w-0">
						<span
							class="size-3.5 rounded-full shrink-0 shadow-xs border border-white/20"
							style="background-color: oklch(0.62 0.22 {themeState.customHue});"
						></span>
						<span class="text-xs font-medium text-foreground">
							Hue Spectrum
						</span>
						<span class="text-[10px] font-mono text-muted-foreground">
							{themeState.customHue}°
						</span>
					</div>

					<div class="flex-1 max-w-xs flex items-center gap-2">
						<input
							type="range"
							min="0"
							max="360"
							step="1"
							value={themeState.customHue}
							oninput={(e) =>
								themeState.setCustomHue(
									Number((e.target as HTMLInputElement).value),
								)}
							class="w-full h-3 rounded-full appearance-none cursor-pointer custom-hue-slider shadow-xs"
							aria-label="Custom hue angle slider"
						/>
					</div>
				</div>
			{/if}
		</div>

		<!-- Colorful Mode Switcher -->
		<div
			class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pt-3 border-t border-border/40"
		>
			<div>
				<div class="text-xs font-medium text-foreground">
					Colorful Mode
				</div>
				<div class="text-[11px] text-muted-foreground">
					Tint neutral backgrounds and borders with the active accent color
				</div>
			</div>

			<div class="flex items-center gap-2">
				<SquareSwitch
					id="colorful-mode-switch"
					checked={themeState.colorful}
					onCheckedChange={(val) => themeState.setColorful(val)}
					label={themeState.colorful ? "Enabled" : "Disabled"}
				/>
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

	.custom-hue-slider {
		background: linear-gradient(
			to right,
			oklch(0.65 0.22 0),
			oklch(0.65 0.22 60),
			oklch(0.65 0.22 120),
			oklch(0.65 0.22 180),
			oklch(0.65 0.22 240),
			oklch(0.65 0.22 300),
			oklch(0.65 0.22 360)
		);
		outline: none;
	}

	.custom-hue-slider::-webkit-slider-thumb {
		appearance: none;
		width: 16px;
		height: 16px;
		border-radius: 50%;
		background: #ffffff;
		border: 2px solid rgba(0, 0, 0, 0.35);
		box-shadow: 0 1px 3px rgba(0, 0, 0, 0.35);
		cursor: pointer;
		transition: transform 0.1s ease;
	}

	.custom-hue-slider::-webkit-slider-thumb:hover {
		transform: scale(1.15);
	}

	.custom-hue-slider::-moz-range-thumb {
		width: 16px;
		height: 16px;
		border-radius: 50%;
		background: #ffffff;
		border: 2px solid rgba(0, 0, 0, 0.35);
		box-shadow: 0 1px 3px rgba(0, 0, 0, 0.35);
		cursor: pointer;
		transition: transform 0.1s ease;
	}

	.custom-hue-slider::-moz-range-thumb:hover {
		transform: scale(1.15);
	}
</style>
