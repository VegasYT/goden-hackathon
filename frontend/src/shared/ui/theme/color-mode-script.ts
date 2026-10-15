// Без импортов: модуль читает и приложение, и vite.config.ts.

/** Ключ сохранённого выбора темы. */
export const colorModeStorageKey = 'goden-color-mode';

/**
 * Скрипт в `<head>` до загрузки приложения: ставит на `<html>` класс сохранённой темы, без него — системной,
 * иначе светлой. Строка на ES5, а не функция: в конфиге Vite нет типов DOM.
 */
export const colorModeInitScript = `(function () {
	var mode = 'light';
	var saved = null;
	try {
		saved = window.localStorage.getItem('${colorModeStorageKey}');
	} catch (e) {}
	if (saved === 'light' || saved === 'dark') {
		mode = saved;
	} else {
		try {
			if (window.matchMedia('(prefers-color-scheme: dark)').matches) {
				mode = 'dark';
			}
		} catch (e) {}
	}
	var root = document.documentElement;
	root.classList.remove('light', 'dark');
	root.classList.add(mode);
	root.style.colorScheme = mode;
})();`;
