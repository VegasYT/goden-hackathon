import { describe, expect, it, vi } from 'vitest';

import { startSession } from './start-session';

function fakeKeycloak(init: () => Promise<boolean>) {
	return {
		init: vi.fn(init),
		login: vi.fn(() => Promise.resolve()),
		tokenParsed: { sub: 'user-1', preferred_username: 'cook' },
	};
}

describe('startSession', () => {
	it('у вошедшего пользователя возвращает сессию из токена', async () => {
		const keycloak = fakeKeycloak(() => Promise.resolve(true));

		const result = await startSession(keycloak);

		expect(result).toEqual({
			status: 'signed-in',
			session: { userId: 'user-1', name: 'cook' },
		});
		expect(keycloak.login).not.toHaveBeenCalled();
	});

	it('без входа уводит на страницу входа и сессии не возвращает', async () => {
		const keycloak = fakeKeycloak(() => Promise.resolve(false));

		expect(await startSession(keycloak)).toEqual({ status: 'redirecting' });
		expect(keycloak.login).toHaveBeenCalled();
	});

	it('если Keycloak не ответил, сообщает о недоступности сервера входа', async () => {
		const keycloak = fakeKeycloak(() => Promise.reject(new Error('timeout')));

		expect(await startSession(keycloak)).toEqual({ status: 'unavailable' });
		expect(keycloak.login).not.toHaveBeenCalled();
	});

	it('если вход завис, через 15 секунд сообщает о недоступности сервера входа', async () => {
		vi.useFakeTimers();
		const keycloak = fakeKeycloak(() => new Promise<boolean>(() => {}));

		const result = startSession(keycloak);

		await vi.advanceTimersByTimeAsync(15_000);

		expect(await result).toEqual({ status: 'unavailable' });
	});
});
