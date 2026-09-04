<script lang="ts">
	import { authState } from "$lib/authState.svelte";
	import { pageState } from "$lib/pageSystem.svelte";
	import { Button } from "$lib/components/ui/button";
	import * as Avatar from "$lib/components/ui/avatar";
	import UserIcon from "@lucide/svelte/icons/user";
	import LogOutIcon from "@lucide/svelte/icons/log-out";
</script>

<!-- Profile section -->
<div class="rounded-xl border border-border bg-card p-6 shadow-xs">
	<div class="flex items-center justify-between">
		<div class="flex items-center gap-4">
			<Avatar.Root class="size-12 rounded-full border border-border bg-muted">
				{#if authState.user?.avatarUrl}
					<Avatar.Image
						src={authState.user.avatarUrl}
						alt={authState.user.name}
					/>
				{/if}
				<Avatar.Fallback class="text-sm font-medium">
					<UserIcon class="size-6 text-muted-foreground" />
				</Avatar.Fallback>
			</Avatar.Root>
			<div class="flex flex-col justify-center">
				<div class="font-medium text-sm text-foreground leading-snug">
					{authState.user?.name || "User"}
				</div>
				<div class="text-xs text-muted-foreground">
					{authState.user?.email || "user@example.com"}
				</div>
			</div>
		</div>

		<Button
			variant="outline"
			size="sm"
			class="gap-2 text-destructive hover:text-destructive hover:bg-destructive/10 border-border cursor-pointer"
			onclick={() => {
				authState.logout();
				pageState.setPage("hero");
			}}
		>
			<LogOutIcon class="size-3.5" />
			<span>Sign out</span>
		</Button>
	</div>
</div>

<style>
	h2 {
		margin: 0px;
	}
</style>
