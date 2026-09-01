<script lang="ts">
	import { authState } from '$lib/authState.svelte';
	import { Button } from '$lib/components/ui/button';
	import CalendarIcon from '@lucide/svelte/icons/calendar';
	import ShieldCheckIcon from '@lucide/svelte/icons/shield-check';
	import AlertCircleIcon from '@lucide/svelte/icons/alert-circle';
	import SparklesIcon from '@lucide/svelte/icons/sparkles';
	import { fade } from 'svelte/transition';

	let isSubmitting = $state(false);

	async function handleGoogleLogin() {
		isSubmitting = true;
		try {
			await authState.loginWithGoogle();
		} catch (e) {
			// error is captured in authState.error
		} finally {
			isSubmitting = false;
		}
	}
</script>

<div class="flex h-full w-full items-center justify-center p-6 bg-background text-foreground" in:fade={{ duration: 250 }}>
	<div class="w-full max-w-md flex flex-col items-center text-center">
		<!-- Logo / Branding placeholder -->
		<div class="size-12 rounded-2xl bg-primary/10 border border-primary/20 flex items-center justify-center mb-6 shadow-sm">
			<CalendarIcon class="size-6 text-primary" />
		</div>

		<h1 class="text-2xl font-semibold tracking-tight sm:text-3xl text-foreground">
			Sign in to Lasso
		</h1>
		<p class="mt-2 text-sm text-muted-foreground max-w-sm">
			Sync your Google Calendar to manage schedules and automate tasks in your workspace.
		</p>

		<!-- OAuth Google requirement banner -->
		<div class="mt-6 w-full rounded-lg border border-border bg-card/60 p-3.5 text-left flex items-start gap-3">
			<ShieldCheckIcon class="size-5 text-primary shrink-0 mt-0.5" />
			<div class="text-xs text-muted-foreground leading-relaxed">
				<strong class="text-foreground font-medium">Google Calendar Connection Required:</strong>
				Lasso uses Google OAuth to access and coordinate your calendar events seamlessly.
			</div>
		</div>

		<!-- Error alert if OAuth failed -->
		{#if authState.error}
			<div class="mt-4 w-full rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-left flex items-start gap-2.5 text-xs text-destructive">
				<AlertCircleIcon class="size-4 shrink-0 mt-0.5" />
				<div>
					<p class="font-medium">OAuth Authentication Notice</p>
					<p class="mt-0.5 opacity-90">{authState.error}</p>
					<p class="mt-1 text-[11px] text-muted-foreground">
						Tip: To enable live Google OAuth, ensure Google Client ID and Secret are configured in PocketBase Admin (<code class="text-xs">http://127.0.0.1:8090/_/</code>).
					</p>
				</div>
			</div>
		{/if}

		<!-- Primary Google OAuth Button -->
		<div class="mt-6 w-full space-y-3">
			<Button
				variant="outline"
				class="w-full h-11 justify-center gap-3 font-medium text-sm border-border bg-card hover:bg-muted hover:text-foreground cursor-pointer transition-all shadow-xs"
				onclick={handleGoogleLogin}
				disabled={isSubmitting}
			>
				<!-- Google colored icon -->
				<svg class="size-4" viewBox="0 0 24 24">
					<path
						fill="#4285F4"
						d="M23.745 12.27c0-.7-.06-1.4-.19-2.07H12v4.51h6.6c-.29 1.52-1.14 2.82-2.4 3.68v3.05h3.88c2.27-2.09 3.665-5.17 3.665-9.17z"
					/>
					<path
						fill="#34A853"
						d="M12 24c3.24 0 5.95-1.08 7.93-2.91l-3.88-3.05c-1.08.72-2.45 1.16-4.05 1.16-3.12 0-5.77-2.1-6.72-4.93H1.26v3.15C3.26 21.36 7.33 24 12 24z"
					/>
					<path
						fill="#FBBC05"
						d="M5.28 14.27c-.25-.72-.38-1.49-.38-2.27s.13-1.55.38-2.27V6.58H1.26C.46 8.16 0 9.94 0 12s.46 3.84 1.26 5.42l4.02-3.15z"
					/>
					<path
						fill="#EA4335"
						d="M12 4.75c1.77 0 3.35.61 4.6 1.8l3.42-3.42C17.95 1.19 15.24 0 12 0 7.33 0 3.26 2.64 1.26 6.58l4.02 3.15c.95-2.83 3.6-4.98 6.72-4.98z"
					/>
				</svg>
				<span>{isSubmitting ? 'Connecting...' : 'Continue with Google'}</span>
			</Button>

			<!-- Dev mode bypass button for instant local development -->
			<Button
				variant="ghost"
				size="sm"
				class="w-full text-xs text-muted-foreground hover:text-foreground cursor-pointer gap-1.5"
				onclick={() => authState.loginDevMock()}
			>
				<SparklesIcon class="size-3.5 text-primary" />
				<span>Dev Mode: Simulate Google Sign-In</span>
			</Button>
		</div>

		<p class="mt-8 text-xs text-muted-foreground">
			By continuing, you authorize Lasso to synchronize calendar events and workspace data via PocketBase.
		</p>
	</div>
</div>
