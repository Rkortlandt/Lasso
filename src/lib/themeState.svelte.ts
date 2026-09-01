export interface AccentOption {
	id: string;
	name: string;
	swatch: string;
	lightPrimary: string;
	darkPrimary: string;
}

export const ACCENT_PALETTE: AccentOption[] = [
	{
		id: 'emerald',
		name: 'Emerald',
		swatch: '#10b981',
		lightPrimary: 'oklch(0.508 0.118 165.612)',
		darkPrimary: 'oklch(0.696 0.17 162.48)'
	},
	{
		id: 'indigo',
		name: 'Indigo',
		swatch: '#6366f1',
		lightPrimary: 'oklch(0.511 0.262 276.966)',
		darkPrimary: 'oklch(0.68 0.22 276.966)'
	},
	{
		id: 'blue',
		name: 'Blue',
		swatch: '#3b82f6',
		lightPrimary: 'oklch(0.546 0.245 262.881)',
		darkPrimary: 'oklch(0.68 0.22 260)'
	},
	{
		id: 'rose',
		name: 'Rose',
		swatch: '#f43f5e',
		lightPrimary: 'oklch(0.577 0.245 15)',
		darkPrimary: 'oklch(0.68 0.22 15)'
	},
	{
		id: 'amber',
		name: 'Amber',
		swatch: '#f59e0b',
		lightPrimary: 'oklch(0.65 0.2 60)',
		darkPrimary: 'oklch(0.75 0.18 60)'
	},
	{
		id: 'purple',
		name: 'Purple',
		swatch: '#a855f7',
		lightPrimary: 'oklch(0.55 0.27 305)',
		darkPrimary: 'oklch(0.7 0.24 305)'
	},
	{
		id: 'teal',
		name: 'Teal',
		swatch: '#06b6d4',
		lightPrimary: 'oklch(0.55 0.16 195)',
		darkPrimary: 'oklch(0.72 0.16 195)'
	}
];

export type ThemeMode = 'dark' | 'light' | 'system';

class ThemeState {
	mode = $state<ThemeMode>('system');
	systemDark = $state(false);
	accentId = $state<string>('emerald');
	calendarOffset = $state<number>(1);

	constructor() {
		if (typeof window !== 'undefined') {
			const mql = window.matchMedia('(prefers-color-scheme: dark)');
			this.systemDark = mql.matches;
			mql.addEventListener('change', (e) => {
				this.systemDark = e.matches;
				if (this.mode === 'system') {
					this.applyTheme();
				}
			});

			const savedMode = localStorage.getItem('lasso_theme_mode') as ThemeMode | null;
			if (savedMode === 'light' || savedMode === 'dark' || savedMode === 'system') {
				this.mode = savedMode;
			} else {
				this.mode = 'system';
			}

			const savedAccent = localStorage.getItem('lasso_accent_id');
			if (savedAccent && ACCENT_PALETTE.some((a) => a.id === savedAccent)) {
				this.accentId = savedAccent;
			}

			const savedOffset = localStorage.getItem('lasso_calendar_offset');
			if (savedOffset !== null) {
				const parsed = parseInt(savedOffset, 10);
				if (!isNaN(parsed) && parsed >= 0 && parsed <= 3) {
					this.calendarOffset = parsed;
				}
			}

			this.applyTheme();
		}
	}

	get resolvedMode(): 'dark' | 'light' {
		if (this.mode === 'system') {
			return this.systemDark ? 'dark' : 'light';
		}
		return this.mode;
	}

	get currentAccent(): AccentOption {
		return ACCENT_PALETTE.find((a) => a.id === this.accentId) || ACCENT_PALETTE[0];
	}

	setMode(newMode: ThemeMode) {
		this.mode = newMode;
		localStorage.setItem('lasso_theme_mode', newMode);
		this.applyTheme();
	}

	setAccent(accentId: string) {
		if (ACCENT_PALETTE.some((a) => a.id === accentId)) {
			this.accentId = accentId;
			localStorage.setItem('lasso_accent_id', accentId);
			this.applyTheme();
		}
	}

	setCalendarOffset(offset: number) {
		if (offset >= 0 && offset <= 3) {
			this.calendarOffset = offset;
			if (typeof window !== 'undefined') {
				localStorage.setItem('lasso_calendar_offset', String(offset));
			}
		}
	}

	applyTheme() {
		if (typeof document === 'undefined') return;

		const active = this.resolvedMode;

		// 1. Toggle dark class on html root
		if (active === 'dark') {
			document.documentElement.classList.add('dark');
		} else {
			document.documentElement.classList.remove('dark');
		}

		// 2. Apply accent color CSS variables
		const accent = this.currentAccent;
		const colorValue = active === 'dark' ? accent.darkPrimary : accent.lightPrimary;

		document.documentElement.style.setProperty('--primary', colorValue);
		document.documentElement.style.setProperty('--sidebar-primary', colorValue);
		document.documentElement.style.setProperty('--ring', colorValue);
	}
}

export const themeState = new ThemeState();
