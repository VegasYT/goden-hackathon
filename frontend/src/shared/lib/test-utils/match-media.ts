import { vi } from 'vitest';

import { theme } from '@shared/ui/theme';

type Listener = (event: { matches: boolean }) => void;

/** Подменяет `matchMedia`: каждый запрос отвечает по функции `matches`. */
export function mockMatchMedia(matches: (query: string) => boolean) {
	vi.stubGlobal(
		'matchMedia',
		vi.fn((query: string) => ({
			matches: matches(query),
			media: query,
			onchange: null,
			addListener: () => {},
			removeListener: () => {},
			addEventListener: () => {},
			removeEventListener: () => {},
			dispatchEvent: () => true,
		}))
	);
}

/** Подменяет системную тему (`prefers-color-scheme`) и позволяет менять её на лету. */
export function mockSystemColorScheme(initial: 'light' | 'dark') {
	const listeners = new Set<Listener>();
	const mql = {
		matches: initial === 'dark',
		media: '(prefers-color-scheme: dark)',
		onchange: null,
		addListener: (l: Listener) => listeners.add(l),
		removeListener: (l: Listener) => listeners.delete(l),
		addEventListener: (_: string, l: Listener) => listeners.add(l),
		removeEventListener: (_: string, l: Listener) => listeners.delete(l),
		dispatchEvent: () => true,
	};

	vi.stubGlobal(
		'matchMedia',
		vi.fn(() => mql)
	);
	return {
		set(scheme: 'light' | 'dark') {
			mql.matches = scheme === 'dark';
			listeners.forEach((l) => l({ matches: mql.matches }));
		},
	};
}

/** Широкий экран (от 1200px): боковая панель и таблицы вместо карточек. */
export function mockWideScreen() {
	mockMatchMedia((query) =>
		query.includes(`min-width:${theme.breakpoints.values.lg}px`)
	);
}
