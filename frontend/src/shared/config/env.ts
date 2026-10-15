/** Адрес Keycloak, realm и клиент веб-приложения; без переменных окружения подходят для локального стенда. */
export const keycloakConfig = {
	url: import.meta.env.VITE_KEYCLOAK_URL || 'http://localhost:8180',
	realm: import.meta.env.VITE_KEYCLOAK_REALM || 'goden',
	clientId: import.meta.env.VITE_KEYCLOAK_CLIENT_ID || 'goden-web',
};

/** Показывать ли раздел «Генератор»; скрывает его только `VITE_GENERATOR_ENABLED=false`. */
export function generatorEnabled(): boolean {
	return import.meta.env.VITE_GENERATOR_ENABLED !== 'false';
}
