import type { Component } from 'svelte';
import HeroPage from './pages/HeroPage.svelte';
import SettingsPage from './pages/SettingsPage.svelte';

export type PageId = 'hero' | 'settings';

export interface PageDefinition {
	id: PageId;
	title: string;
	component: Component;
}

export const pages: Record<PageId, PageDefinition> = {
	hero: {
		id: 'hero',
		title: 'Home',
		component: HeroPage
	},
	settings: {
		id: 'settings',
		title: 'Settings',
		component: SettingsPage
	}
};

class PageState {
	current = $state<PageId>('hero');
	private history: PageId[] = [];

	get activePage(): PageDefinition {
		return pages[this.current] ?? pages.hero;
	}

	setPage(pageId: PageId) {
		if (pages[pageId] && pageId !== this.current) {
			this.history.push(this.current);
			this.current = pageId;
		}
	}

	goBack() {
		const prev = this.history.pop();
		this.current = prev ?? 'hero';
	}
}

export const pageState = new PageState();
