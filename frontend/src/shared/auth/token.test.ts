import { beforeEach, describe, expect, it, vi } from 'vitest';

import type * as tokenModule from './token';

const keycloak = vi.hoisted(() => ({
	token: 'access-token',
	refreshToken: 'refresh-token' as string | undefined,
	updateToken: vi.fn<() => Promise<boolean>>(),
	login: vi.fn(() => Promise.resolve()),
}));

vi.mock('./keycloak', () => ({ keycloak }));

let getAccessToken: typeof tokenModule.getAccessToken;

beforeEach(async () => {
	keycloak.refreshToken = 'refresh-token';
	// Свежий модуль, чтобы уход на вход из прошлого теста не мешал.
	vi.resetModules();
	({ getAccessToken } = await import('./token'));
	window.history.replaceState(null, '', '/kitchen?tab=queue');
});

function refuseRefresh() {
	keycloak.updateToken.mockImplementation(() => {
		keycloak.refreshToken = undefined;
		return Promise.reject(new Error('400'));
	});
}

describe('getAccessToken', () => {
	it('при отказе Keycloak уводит на вход с возвратом на текущий адрес', async () => {
		refuseRefresh();

		await expect(getAccessToken()).rejects.toThrow();
		expect(keycloak.login).toHaveBeenCalledWith({
			redirectUri: `${window.location.origin}/kitchen?tab=queue`,
		});
	});

	it('при одновременных отказах уводит на вход один раз', async () => {
		refuseRefresh();

		await Promise.allSettled([getAccessToken(), getAccessToken()]);

		expect(keycloak.login).toHaveBeenCalledTimes(1);
	});

	it('при обрыве связи на вход не уводит', async () => {
		keycloak.updateToken.mockRejectedValue(new TypeError('Failed to fetch'));

		await expect(getAccessToken()).rejects.toThrow('Failed to fetch');
		expect(keycloak.login).not.toHaveBeenCalled();
	});
});
