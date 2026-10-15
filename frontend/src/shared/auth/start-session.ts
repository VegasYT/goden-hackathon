import type Keycloak from 'keycloak-js';

import { keycloak as defaultKeycloak } from './keycloak';
import { sessionFromToken, type Session, type TokenClaims } from './session';

export type StartSessionResult =
	| { status: 'signed-in'; session: Session }
	| { status: 'redirecting' }
	| { status: 'unavailable' };

type KeycloakClient = Pick<Keycloak, 'init' | 'login' | 'tokenParsed'>;

// Тихая проверка входа в keycloak-js ждёт ответа iframe без таймаута, поэтому ограничиваем весь вход.
// Срок больше штатных 10 с keycloak-js, чтобы его собственный таймаут срабатывал первым.
const initTimeoutMs = 15_000;

function withTimeout<T>(promise: Promise<T>, ms: number): Promise<T> {
	return new Promise((resolve, reject) => {
		const timer = setTimeout(
			() => reject(new Error('Вход не завершился вовремя')),
			ms
		);

		promise.then(resolve, reject).finally(() => clearTimeout(timer));
	});
}

/** Поднимает сессию до рендера приложения; без входа уводит на страницу Keycloak. */
export async function startSession(
	keycloak: KeycloakClient = defaultKeycloak
): Promise<StartSessionResult> {
	let authenticated: boolean;

	try {
		authenticated = await withTimeout(
			keycloak.init({
				onLoad: 'check-sso',
				silentCheckSsoRedirectUri: `${window.location.origin}/silent-check-sso.html`,
				checkLoginIframe: false,
				pkceMethod: 'S256',
			}),
			initTimeoutMs
		);
	} catch {
		return { status: 'unavailable' };
	}

	if (!authenticated) {
		await keycloak.login();
		return { status: 'redirecting' };
	}

	return {
		status: 'signed-in',
		session: sessionFromToken((keycloak.tokenParsed ?? {}) as TokenClaims),
	};
}
