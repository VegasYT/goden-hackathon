import { createGetKcContextMock } from 'keycloakify/login/KcContext';

import { kcEnvDefaults, themeNames } from '../kc.gen';
import type { KcContext } from './KcContext';

type PageId = KcContext['pageId'];
type MessageType = 'info' | 'warning' | 'success' | 'error';

const { getKcContextMock } = createGetKcContextMock({
	kcContextExtension: { themeName: themeNames[0], properties: kcEnvDefaults },
	kcContextExtensionPerPage: {},
	overrides: {
		locale: { currentLanguageTag: 'ru' },
		realm: {
			name: 'goden',
			displayName: 'Goden',
			displayNameHtml: 'Goden',
			registrationEmailAsUsername: false,
		},
	},
	overridesPerPage: {
		// Как в realm goden: регистрация есть, восстановления пароля и «Запомнить меня» нет.
		'login.ftl': {
			realm: {
				password: true,
				registrationAllowed: true,
				loginWithEmailAllowed: true,
				resetPasswordAllowed: false,
				rememberMe: false,
			},
		},
		'login-verify-email.ftl': { user: { email: 'anna@example.com' } },
		'info.ftl': {
			messageHeader: 'Почта подтверждена',
			message: { type: 'success', summary: 'Теперь можно войти в Goden.' },
		},
		'error.ftl': {
			message: {
				type: 'error',
				summary: 'Ссылка недействительна или срок её действия истёк.',
			},
			client: { baseUrl: '/' },
		},
	},
});

const sampleMessages: Record<MessageType, string> = {
	info: 'Мы отправили письмо со ссылкой на вашу почту.',
	warning: 'Вам нужно сменить пароль, чтобы продолжить.',
	success: 'Письмо отправлено ещё раз.',
	error: 'Не удалось выполнить действие. Попробуйте ещё раз.',
};

/**
 * Тестовые данные страницы темы: `state` выбирает состояние.
 * `field-errors` — ошибки полей входа, регистрации и смены пароля, `resent` — письмо отправлено повторно,
 * `message-info`, `message-warning`, `message-success`, `message-error` — сообщение Keycloak над формой.
 */
export function getMockKcContext(pageId: PageId, state: string | null = null) {
	return getKcContextMock({
		pageId,
		overrides: stateOverrides(pageId, state) as never,
	});
}

/** Тестовые данные по параметрам адреса `kcPage` и `kcState`; без `kcPage` данных нет. */
export function kcContextFromSearch(search: string): KcContext | undefined {
	const params = new URLSearchParams(search);
	const pageId = params.get('kcPage');

	if (!pageId) {
		return undefined;
	}

	return getMockKcContext(pageId as PageId, params.get('kcState'));
}

function stateOverrides(pageId: PageId, state: string | null) {
	if (state?.startsWith('message-')) {
		const type = state.slice('message-'.length) as MessageType;

		return { message: { type, summary: sampleMessages[type] } };
	}

	if (state === 'resent') {
		return { message: { type: 'success', summary: sampleMessages.success } };
	}

	if (state !== 'field-errors') {
		return {};
	}

	switch (pageId) {
		case 'login.ftl':
			return {
				login: { username: 'manager' },
				messagesPerField: fieldErrors({
					username: 'Неверный логин или пароль.',
				}),
			};

		case 'register.ftl':
			return {
				profile: {
					attributesByName: {
						username: { value: 'anna' },
						email: { value: 'anna@example.com' },
						firstName: { value: 'Анна' },
						lastName: { value: 'Смирнова' },
					},
				},
				messagesPerField: fieldErrors({
					username: 'Этот логин уже занят.',
					'password-confirm': 'Пароли не совпадают.',
				}),
			};

		case 'login-update-password.ftl':
			return {
				messagesPerField: fieldErrors({
					'password-confirm': 'Пароли не совпадают.',
				}),
			};

		default:
			return {};
	}
}

function fieldErrors(
	errors: Record<string, string>
): KcContext['messagesPerField'] {
	const get = (name: string) => errors[name] ?? '';
	const has = (name: string) => name in errors;

	return {
		get,
		exists: has,
		existsError: (...names: string[]) => names.some(has),
		printIfExists: <T extends string>(name: string, text: T) =>
			has(name) ? text : undefined,
		getFirstError: (...names: string[]) => get(names.find(has) ?? ''),
	};
}
