import { keycloak } from './keycloak';

let loginRedirect: Promise<void> | undefined;

/** Отдаёт действующий access token, заранее обновив его, если он скоро истечёт. */
export async function getAccessToken(): Promise<string> {
	try {
		await keycloak.updateToken(30);
	} catch (error) {
		// keycloak-js стирает refresh token, только когда Keycloak отказал продлить сессию.
		// Тогда запрос ждёт ухода на страницу входа, а при обрыве связи сразу получает ошибку.
		if (!keycloak.refreshToken) {
			await relogin();
		}

		throw error;
	}

	if (!keycloak.token) {
		throw new Error('Нет токена доступа');
	}

	return keycloak.token;
}

function relogin(): Promise<void> {
	return (loginRedirect ??= keycloak.login({
		redirectUri: window.location.href,
	}));
}

/** Завершает сессию Keycloak и возвращает на корень приложения. */
export function logout(): Promise<void> {
	return keycloak.logout({ redirectUri: `${window.location.origin}/` });
}
