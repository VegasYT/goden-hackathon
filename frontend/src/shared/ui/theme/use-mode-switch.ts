import { useColorScheme } from '@mui/material';

/** Переключение светлой и тёмной темы для кнопки или пункта меню; `null`, пока тема не определена. */
export function useModeSwitch() {
	const { mode, systemMode, setMode } = useColorScheme();
	const current = mode === 'system' ? systemMode : mode;

	if (!current) {
		return null;
	}

	const next = current === 'dark' ? 'light' : 'dark';

	return {
		label: next === 'dark' ? 'Включить тёмную тему' : 'Включить светлую тему',
		icon: current === 'dark' ? ('lightMode' as const) : ('darkMode' as const),
		toggle: () => setMode(next),
	};
}
