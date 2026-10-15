import { fireEvent, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { renderWithTheme } from '@shared/lib/test-utils';

import KcPage from './KcPage';
import type { KcContext } from './KcContext';
import { getMockKcContext } from './mocks';

function renderPage(pageId: KcContext['pageId'], state?: string) {
	return renderWithTheme(
		<KcPage kcContext={getMockKcContext(pageId, state ?? null)} />
	);
}

describe('каркас страниц входа', () => {
	it('страница без своей вёрстки открывается в каркасе с логотипом и формой Keycloak', async () => {
		renderPage('login-otp.ftl');

		expect(await screen.findByText('Goden')).toBeInTheDocument();
		expect(document.querySelector('form')).not.toBeNull();
	});

	it('сообщение об ошибке Keycloak показано как предупреждение с иконкой', async () => {
		renderPage('login.ftl', 'message-error');

		const alert = await screen.findByRole('alert');

		expect(alert).toHaveTextContent(
			'Не удалось выполнить действие. Попробуйте ещё раз.'
		);
		expect(alert.querySelector('svg')).not.toBeNull();
	});
});

describe('вход', () => {
	it('при неверном пароле показывает ошибку, сохраняет логин и очищает пароль', async () => {
		renderPage('login.ftl', 'field-errors');

		expect(await screen.findByRole('alert')).toHaveTextContent(
			'Неверный логин или пароль.'
		);

		const username = screen.getByLabelText('Логин или почта');
		const password = screen.getByLabelText('Пароль');

		expect(username).toHaveValue('manager');
		expect(password).toHaveValue('');
		expect(password).toHaveAccessibleDescription('Неверный логин или пароль.');
		expect(password).toHaveFocus();
	});

	it('фокус стоит в поле логина, а выключенных в realm действий нет', async () => {
		renderPage('login.ftl');

		expect(await screen.findByLabelText('Логин или почта')).toHaveFocus();
		expect(
			screen.getByRole('link', { name: 'Зарегистрироваться' })
		).toBeInTheDocument();
		expect(screen.queryByText('Запомнить меня')).not.toBeInTheDocument();
		expect(
			screen.queryByRole('link', { name: /пароль/i })
		).not.toBeInTheDocument();
	});

	it('повторная отправка формы не уходит на сервер', async () => {
		renderPage('login.ftl');

		const button = await screen.findByRole('button', { name: 'Войти' });
		const form = button.closest('form')!;

		expect(fireEvent.submit(form)).toBe(true);
		expect(fireEvent.submit(form)).toBe(false);
		expect(button).toBeDisabled();
	});
});

describe('регистрация', () => {
	it('показывает ошибки у своих полей и сохраняет введённые значения', async () => {
		renderPage('register.ftl', 'field-errors');

		const username = await screen.findByLabelText('Логин');

		expect(username).toHaveValue('anna');
		expect(username).toHaveAccessibleDescription('Этот логин уже занят.');
		expect(screen.getByLabelText('Почта')).toHaveValue('anna@example.com');
		expect(
			screen.getByLabelText('Подтверждение пароля')
		).toHaveAccessibleDescription('Пароли не совпадают.');
	});
	it('пустое обязательное поле даёт ошибку на русском', async () => {
		renderPage('register.ftl');

		const username = await screen.findByLabelText(/Логин/);

		fireEvent.blur(username);

		expect(await screen.findByText('Заполните это поле.')).toBeInTheDocument();
	});
});

describe('смена пароля', () => {
	it('несовпадение паролей показано у поля подтверждения', async () => {
		renderPage('login-update-password.ftl', 'field-errors');

		const confirm = await screen.findByLabelText('Подтверждение пароля');

		expect(confirm).toHaveAccessibleDescription('Пароли не совпадают.');
		expect(
			within(confirm.closest('form')!).getByLabelText('Новый пароль')
		).not.toHaveAccessibleDescription();
	});
});
