import { CssBaseline, ThemeProvider } from '@mui/material';
import type { ReactNode } from 'react';

import { colorModeStorageKey, theme } from './theme';

/** Провайдер темы, оборачивает приложение один раз в корне. */
export function ThemeRoot({ children }: { children: ReactNode }) {
	return (
		<ThemeProvider
			theme={theme}
			defaultMode='system'
			modeStorageKey={colorModeStorageKey}
			disableTransitionOnChange
			noSsr
		>
			<CssBaseline />
			{children}
		</ThemeProvider>
	);
}
