import { i18nBuilder } from 'keycloakify/login';

import type { ThemeName } from '../kc.gen';

const i18nApi = i18nBuilder
	.withThemeName<ThemeName>()
	// Формулировки страниц входа; поверх переводов Keycloak, поэтому стандартные ключи тоже здесь.
	// Объект записан прямо в вызове: Keycloakify читает его при сборке темы и кладёт в сообщения
	// Keycloak, из переменной он его не достаёт. Язык один: для любого другого берётся он же.
	.withCustomTranslations({
		ru: {
			username: 'Логин',
			usernameOrEmail: 'Логин или почта',
			email: 'Почта',
			firstName: 'Имя',
			lastName: 'Фамилия',
			password: 'Пароль',
			passwordNew: 'Новый пароль',
			passwordConfirm: 'Подтверждение пароля',
			invalidUserMessage: 'Неверный логин или пароль.',
			usernameExistsMessage: 'Этот логин уже занят.',
			emailExistsMessage: 'Эта почта уже зарегистрирована.',
			invalidPasswordConfirmMessage: 'Пароли не совпадают.',
			// Проверки полей профиля: в русском наборе Keycloak и Keycloakify их нет, без них текст английский.
			'error-user-attribute-required': 'Заполните это поле.',
			'error-invalid-blank': 'Заполните это поле.',
			'error-empty': 'Заполните это поле.',
			'error-invalid-value': 'Недопустимое значение.',
			'error-pattern-no-match': 'Недопустимое значение.',
			'error-invalid-length': 'Длина должна быть от {1} до {2} символов.',
			'error-invalid-length-too-short': 'Не короче {1} символов.',
			'error-invalid-length-too-long': 'Не длиннее {2} символов.',
			'error-invalid-email': 'Некорректный адрес почты.',
			invalidEmailMessage: 'Некорректный адрес почты.',
			'error-invalid-number': 'Введите число.',
			'error-number-out-of-range': 'Число должно быть от {1} до {2}.',
			'error-number-out-of-range-too-small': 'Число должно быть не меньше {1}.',
			'error-number-out-of-range-too-big': 'Число должно быть не больше {2}.',
			'error-invalid-date': 'Некорректная дата.',
			'error-user-attribute-read-only': 'Это поле нельзя изменить.',
			'error-username-invalid-character': 'Логин содержит недопустимый символ.',
			'error-person-name-invalid-character':
				'Значение содержит недопустимый символ.',
			godenTagline: 'Мониторинг склада, сроков годности и стоп-листа',
			godenLoginTitle: 'Вход в учётную запись',
			godenDoLogIn: 'Войти',
			godenNoAccount: 'Нет учётной записи?',
			godenDoRegister: 'Зарегистрироваться',
			godenRegisterTitle: 'Регистрация',
			godenRegisterSubtitle:
				'После входа вы создадите компанию или дождётесь, когда вас добавят на точку',
			godenHaveAccount: 'Уже есть учётная запись?',
			godenShowPassword: 'Показать пароль',
			godenHidePassword: 'Скрыть пароль',
			godenRestartLogin: 'Войти под другим логином',
			godenTryAnotherWay: 'Выбрать другой способ входа',
			godenVerifyEmailTitle: 'Подтвердите почту',
			godenVerifyEmailSubtitle: 'Остался один шаг до входа в Goden',
			godenVerifyEmailSent:
				'Мы отправили письмо на <b>{0}</b>. Перейдите по ссылке в письме, чтобы завершить регистрацию.',
			godenVerifyEmailNoLetter:
				'Письмо не пришло? Проверьте папку "Спам" или отправьте его ещё раз.',
			godenDoResendEmail: 'Отправить письмо ещё раз',
			godenUpdatePasswordTitle: 'Новый пароль',
			godenUpdateProfileTitle: 'Заполните профиль',
			godenDoSaveAndContinue: 'Сохранить и войти',
			godenDoCancel: 'Отмена',
			godenLogoutTitle: 'Выйти из Goden?',
			godenLogoutText:
				'Сессия завершится на этом устройстве. Чтобы вернуться, понадобится снова ввести пароль.',
			godenDoLogout: 'Выйти',
			godenBackToApp: 'Вернуться в приложение',
			godenPageExpiredTitle: 'Страница устарела',
			godenPageExpiredText:
				'Форму открыли слишком давно, и Keycloak её больше не принимает. Продолжите вход или начните его заново.',
			godenDoContinueLogin: 'Продолжить вход',
			godenDoRestartLogin: 'Начать вход заново',
			godenErrorTitle: 'Не удалось продолжить вход',
			godenDoProceed: 'Продолжить',
		},
	})
	.build();

export const { useI18n } = i18nApi;

export type I18n = typeof i18nApi.ofTypeI18n;
