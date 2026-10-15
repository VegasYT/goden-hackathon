import { afterEach, describe, expect, it, vi } from 'vitest';

import { mockMatchMedia } from '@shared/lib/test-utils';

import { colorModeInitScript, colorModeStorageKey } from './color-mode-script';
import { theme } from './theme';

function runScript({
	saved,
	system,
}: {
	saved?: string;
	system?: 'light' | 'dark' | 'unavailable';
}) {
	if (saved) window.localStorage.setItem(colorModeStorageKey, saved);
	if (system === 'unavailable') {
		vi.stubGlobal('matchMedia', undefined);
	} else {
		mockMatchMedia(() => system === 'dark');
	}

	new Function(colorModeInitScript)();
	return document.documentElement.classList;
}

describe('скрипт выбора темы до загрузки приложения', () => {
	afterEach(() => {
		document.documentElement.className = '';
	});

	it('ставит классы, которые ждёт тема', () => {
		// Селектор 'class' в теме ожидает классы light и dark на <html>, их ставит скрипт.
		expect(
			(theme as unknown as { colorSchemeSelector: string }).colorSchemeSelector
		).toBe('class');
	});

	it('сохранённая светлая тема важнее тёмной системной', () => {
		const classes = runScript({ saved: 'light', system: 'dark' });

		expect(classes.contains('light')).toBe(true);
		expect(classes.contains('dark')).toBe(false);
	});

	it('без сохранённого выбора берёт тёмную системную тему', () => {
		expect(runScript({ system: 'dark' }).contains('dark')).toBe(true);
	});

	it('без matchMedia и сохранённого выбора остаётся светлая', () => {
		expect(runScript({ system: 'unavailable' }).contains('light')).toBe(true);
	});
});
