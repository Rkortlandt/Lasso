import { pb } from './pocketbase';
import type { RecordModel } from 'pocketbase';

export interface AuthUser {
	id: string;
	email: string;
	name: string;
	avatarUrl?: string;
}

class AuthState {
	isValid = $state(pb.authStore.isValid);
	record = $state<RecordModel | null>(pb.authStore.record);
	isLoading = $state(false);
	error = $state<string | null>(null);

	constructor() {
		pb.authStore.onChange((token, record) => {
			this.isValid = pb.authStore.isValid;
			this.record = record;
		});

		if (typeof window !== 'undefined' && pb.authStore.isValid) {
			pb.collection('users')
				.authRefresh()
				.catch((err) => {
					console.warn('Initial auth refresh failed:', err);
				});
		}
	}

	get isAuthenticated(): boolean {
		return this.isValid && this.record !== null;
	}

	get user(): AuthUser | null {
		if (!this.record) return null;
		return {
			id: this.record.id,
			email: this.record.email || '',
			name: this.record.name || (this.record.email ? this.record.email.split('@')[0] : 'User'),
			avatarUrl: this.record.avatar ? pb.files.getURL(this.record, this.record.avatar) : undefined
		};
	}

	async loginWithGoogle() {
		this.isLoading = true;
		this.error = null;
		try {
			// Force Google OAuth with Google Calendar scopes
			const authData = await pb.collection('users').authWithOAuth2({
				provider: 'google',
				scopes: [
					'https://www.googleapis.com/auth/userinfo.profile',
					'https://www.googleapis.com/auth/userinfo.email',
					'https://www.googleapis.com/auth/calendar',
					'https://www.googleapis.com/auth/calendar.events'
				],
				urlCallback: (url) => {
					const authUrl = new URL(url);
					authUrl.searchParams.set('access_type', 'offline');
					window.open(authUrl.toString(), 'popup_window', 'width=1024,height=768');
				}
			});
			this.isValid = pb.authStore.isValid;
			this.record = pb.authStore.record;
			if (authData?.meta?.accessToken) {
				import('./googleCalendarState.svelte').then(({ googleCalendarState }) => {
					googleCalendarState.initFromAuth(authData);
				});
			}
			return authData;
		} catch (err: any) {
			console.error('Google OAuth error:', err);
			this.error = err?.message || 'Failed to authenticate with Google';
			throw err;
		} finally {
			this.isLoading = false;
		}
	}

	loginDevMock(email = 'alex.lasso@gmail.com', name = 'Alex Rivera') {
		pb.authStore.save('mock-lasso-token-' + Date.now(), {
			id: 'mock_user_1',
			collectionId: 'users',
			collectionName: 'users',
			email,
			name,
			created: new Date().toISOString(),
			updated: new Date().toISOString()
		} as any);
		this.isValid = true;
		this.record = pb.authStore.record;
		this.error = null;
	}

	logout() {
		pb.authStore.clear();
		this.isValid = false;
		this.record = null;
	}
}

export const authState = new AuthState();
