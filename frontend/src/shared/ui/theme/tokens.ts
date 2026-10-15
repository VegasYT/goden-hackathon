// Тема MUI, тест контраста и витрина читают цвета отсюда; компоненты берут их из темы.

export type ColorSchemeName = 'light' | 'dark';

export interface ColorTokens {
	/** Фон страницы и навигации. */
	canvas: string;
	/** Карточки, таблицы, панели. */
	paper: string;
	/** Фон полей ввода. */
	field: string;
	/** Дорожки прогресса, фон сегментированного переключателя. */
	track: string;
	/** Разделители и рамки карточек. */
	line: string;
	/** Контур вторичных кнопок и чипов. */
	lineStrong: string;
	/** Граница полей ввода, контраст не ниже 3:1. */
	lineInput: string;
	/** Заголовки и значения. */
	ink1: string;
	/** Основной текст. */
	ink2: string;
	/** Вторичный текст. */
	ink3: string;
	/** Метаданные и подсказки. Не используется на фоне `track`. */
	ink4: string;
	/** Основная кнопка и активные элементы. */
	brand: string;
	/** Текст на `brand`. */
	onBrand: string;
	/** Зелёный текст, иконки, кольцо фокуса. */
	brandInk: string;
	brandTint: string;
	brandOnTint: string;
	/** Графика внимания: шкалы, точки. */
	warnFill: string;
	/** Текст внимания. */
	warnInk: string;
	warnTint: string;
	warnOnTint: string;
	/** Графика опасности. */
	dangerFill: string;
	/** Текст опасности. */
	dangerInk: string;
	dangerTint: string;
	dangerOnTint: string;
	/** Нейтральная графика, например прошедшая часть шкалы срока. */
	neutralFill: string;
	/** Затемнение под диалогом и drawer. */
	overlay: string;
}

export type ColorTokenName = keyof ColorTokens;

export const colorTokens: Record<ColorSchemeName, ColorTokens> = {
	light: {
		canvas: '#F3F6F5',
		paper: '#FFFFFF',
		field: '#F7F9F8',
		track: '#E7ECEA',
		line: 'rgba(15,30,25,.09)',
		lineStrong: 'rgba(15,30,25,.18)',
		lineInput: '#848F8A',
		ink1: '#131A17',
		ink2: '#2B3531',
		ink3: '#56625D',
		ink4: '#67726D',
		brand: '#22C55E',
		onBrand: '#052E16',
		brandInk: '#15803D',
		brandTint: '#DCFCE7',
		brandOnTint: '#166534',
		warnFill: '#D97706',
		warnInk: '#B45309',
		warnTint: '#FEF3C7',
		warnOnTint: '#92400E',
		dangerFill: '#DC2626',
		dangerInk: '#B91C1C',
		dangerTint: '#FEE2E2',
		dangerOnTint: '#991B1B',
		neutralFill: '#848F8A',
		overlay: 'rgba(15,30,25,.32)',
	},
	dark: {
		canvas: '#0F1117',
		paper: '#1A1D27',
		field: '#141720',
		track: '#262A36',
		line: 'rgba(255,255,255,.08)',
		lineStrong: 'rgba(255,255,255,.16)',
		lineInput: '#667385',
		ink1: '#F1F5F9',
		ink2: '#D5DCE5',
		ink3: '#A3ADBA',
		ink4: '#8896A7',
		brand: '#34D399',
		onBrand: '#052E1C',
		brandInk: '#34D399',
		brandTint: 'rgba(52,211,153,.16)',
		brandOnTint: '#6EE7B7',
		warnFill: '#F59E0B',
		warnInk: '#FBBF24',
		warnTint: 'rgba(245,158,11,.14)',
		warnOnTint: '#FCD34D',
		dangerFill: '#EF4444',
		dangerInk: '#F87171',
		dangerTint: 'rgba(239,68,68,.16)',
		dangerOnTint: '#FCA5A5',
		neutralFill: '#5F6B7A',
		overlay: 'rgba(0,0,0,.56)',
	},
};

/** Тень всплывающих слоёв: меню, popover, диалога и drawer. */
export const overlayShadow: Record<ColorSchemeName, string> = {
	light: '0 0 0 1px rgba(15,30,25,.09), 0 12px 32px rgba(15,30,25,.16)',
	dark: '0 0 0 1px rgba(255,255,255,.08), 0 16px 40px rgba(0,0,0,.5)',
};
