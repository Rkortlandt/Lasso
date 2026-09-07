<script lang="ts">
	import { pb, POCKETBASE_URL } from "$lib/pocketbase";
	import { authState } from "$lib/authState.svelte";
	import { dataState } from "$lib/dataState/dataState.svelte";
	import { syncController } from "$lib/syncController.svelte";
	import { Button } from "$lib/components/ui/button";
	import BookOpenIcon from "@lucide/svelte/icons/book-open";
	import ExternalLinkIcon from "@lucide/svelte/icons/external-link";
	import AlertCircleIcon from "@lucide/svelte/icons/alert-circle";
	import KeyRoundIcon from "@lucide/svelte/icons/key-round";
	import GlobeIcon from "@lucide/svelte/icons/globe";
	import CheckCircle2Icon from "@lucide/svelte/icons/check-circle-2";
	import SparklesIcon from "@lucide/svelte/icons/sparkles";
	import { fade } from "svelte/transition";

	interface Props {
		onSkip?: () => void;
	}

	let { onSkip }: Props = $props();

	let canvasUrl = $state(
		authState.record?.canvas_url || "https://canvas.instructure.com",
	);
	let canvasToken = $state("");
	let showInstructions = $state(false);
	let isLoading = $state(false);
	let error = $state<string | null>(null);

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		if (!canvasToken.trim()) return;

		let cleanUrl = canvasUrl.trim();
		if (!cleanUrl) {
			cleanUrl = "https://canvas.instructure.com";
		} else if (
			!cleanUrl.startsWith("http://") &&
			!cleanUrl.startsWith("https://")
		) {
			cleanUrl = "https://" + cleanUrl;
		}
		cleanUrl = cleanUrl.replace(/\/+$/, "");
		canvasUrl = cleanUrl;

		isLoading = true;
		error = null;

		try {
			// 1. Save canvas_url and canvas_token to user record
			if (pb.authStore.record?.id) {
				await pb.collection("users").update(pb.authStore.record.id, {
					canvas_url: cleanUrl,
					canvas_token: canvasToken.trim(),
				});
			}

			// 2. Call verify endpoint
			const res = await fetch(`${POCKETBASE_URL}/api/canvas/verify`, {
				method: "POST",
				headers: {
					"Content-Type": "application/json",
					Authorization: pb.authStore.token ? `Bearer ${pb.authStore.token}` : "",
				},
				body: JSON.stringify({ canvasUrl: cleanUrl, canvasToken: canvasToken.trim() }),
			});
			const data = await res.json();
			if (!res.ok || !data.success) {
				throw new Error(data.message || data.error || "Failed to verify Canvas token");
			}

			// 3. Refresh user record and run initial sync
			await pb.collection("users").authRefresh();
			await syncController.syncCanvas().catch(() => {});
			await dataState.refresh();
		} catch (err: any) {
			error = err?.message || "Could not connect to Canvas";
		} finally {
			isLoading = false;
		}
	}
</script>

<div
	class="flex h-full w-full items-center justify-center p-6 bg-background text-foreground overflow-y-auto"
	in:fade={{ duration: 250 }}
>
	<div class="w-full max-w-lg flex flex-col items-center">
		<!-- Step indicator -->
		<div class="flex items-center gap-2 mb-6 text-xs text-muted-foreground">
			<span class="inline-flex items-center gap-1 text-emerald-400 font-medium">
				<CheckCircle2Icon class="size-3.5" />
				Google Logged In
			</span>
			<span>•</span>
			<span class="font-medium text-foreground">Step 2: Connect Canvas LMS</span
			>
		</div>

		<!-- Icon & Header -->
		<div
			class="size-12 rounded-2xl bg-primary/10 border border-primary/20 flex items-center justify-center mb-4 shadow-sm"
		>
			<BookOpenIcon class="size-6 text-primary" />
		</div>

		<h1
			class="text-2xl font-semibold tracking-tight sm:text-3xl text-foreground text-center"
		>
			Connect your Canvas
		</h1>
		<p class="mt-2 text-sm text-muted-foreground text-center max-w-md">
			Sync assignments, exam dates, and course schedules directly onto your
			Lasso calendar.
		</p>

		<!-- Error display -->
		{#if error}
			<div
				class="mt-4 w-full rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-left flex items-start gap-2.5 text-xs text-destructive"
			>
				<AlertCircleIcon class="size-4 shrink-0 mt-0.5" />
				<div>
					<p class="font-medium">Connection Failed</p>
					<p class="mt-0.5 opacity-90">{error}</p>
				</div>
			</div>
		{/if}

		<!-- Setup Form -->
		<form onsubmit={handleSubmit} novalidate class="mt-6 w-full space-y-4">
			<!-- Canvas Domain / Institution URL -->
			<div>
				<label
					for="canvas-url"
					class="block text-xs font-medium text-muted-foreground mb-1.5 text-left"
				>
					Canvas Institution URL
				</label>
				<div class="relative flex items-center">
					<GlobeIcon
						class="absolute left-3 size-4 text-muted-foreground pointer-events-none"
					/>
					<input
						id="canvas-url"
						type="text"
						inputmode="url"
						bind:value={canvasUrl}
						placeholder="canvas.instructure.com or school.instructure.com"
						class="w-full rounded-lg border border-input bg-card pl-9 pr-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring transition-all"
						required
					/>
				</div>
				<p class="text-[11px] text-muted-foreground mt-1 text-left">
					Enter your school or institution's Canvas web address.
				</p>
			</div>

			<!-- Canvas API Access Token -->
			<div>
				<div class="flex items-center justify-between mb-1.5">
					<label
						for="canvas-token"
						class="text-xs font-medium text-muted-foreground"
					>
						Canvas Access Token
					</label>
					<button
						type="button"
						class="text-[11px] text-primary hover:underline cursor-pointer inline-flex items-center gap-1"
						onclick={() => (showInstructions = !showInstructions)}
					>
						<span
							>{showInstructions
								? "Hide instructions"
								: "How to get a token?"}</span
						>
					</button>
				</div>

				<div class="relative flex items-center">
					<KeyRoundIcon
						class="absolute left-3 size-4 text-muted-foreground pointer-events-none"
					/>
					<input
						id="canvas-token"
						type="password"
						bind:value={canvasToken}
						placeholder="Paste your Canvas access token here"
						class="w-full rounded-lg border border-input bg-card pl-9 pr-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring transition-all"
						required
					/>
				</div>
			</div>

			<!-- Step by step helper box -->
			{#if showInstructions}
				<div
					class="rounded-lg border border-border bg-muted/40 p-3.5 text-xs text-muted-foreground space-y-2 text-left"
					in:fade={{ duration: 150 }}
				>
					<p class="font-medium text-foreground">
						How to generate a Canvas Access Token:
					</p>
					<ol class="list-decimal pl-4 space-y-1 text-muted-foreground">
						<li>Log in to your Canvas account in your browser.</li>
						<li>
							Click on <strong>Account</strong> in the left global navigation,
							then select <strong>Settings</strong>.
						</li>
						<li>
							Scroll down to the <strong>Approved Integrations</strong> section.
						</li>
						<li>
							Click <strong>+ New Access Token</strong>, give it a purpose like
							"Lasso", and click <strong>Generate Token</strong>.
						</li>
						<li>Copy the token and paste it in the field above.</li>
					</ol>
				</div>
			{/if}

			<!-- Submit button -->
			<div class="pt-2 space-y-2.5">
				<Button
					type="submit"
					class="w-full h-10 font-medium cursor-pointer transition-all"
					disabled={isLoading || !canvasToken.trim()}
				>
					<span
						>{isLoading
							? "Verifying with Canvas..."
							: "Connect Canvas Account"}</span
					>
				</Button>

				{#if onSkip}
					<Button
						type="button"
						variant="ghost"
						class="w-full text-xs text-muted-foreground hover:text-foreground cursor-pointer"
						onclick={onSkip}
					>
						Skip for now
					</Button>
				{/if}
			</div>
		</form>
	</div>
</div>
