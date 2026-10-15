import { ruRU } from '@mui/material/locale';
import { createTheme, type PaletteOptions } from '@mui/material/styles';

import { colorModeStorageKey } from './color-mode-script';
import { components } from './components';
import { colorTokens, type ColorSchemeName } from './tokens';
import { typography } from './typography';

export { colorModeStorageKey };

function palette(scheme: ColorSchemeName): PaletteOptions {
	const t = colorTokens[scheme];
	const brand = {
		main: t.brand,
		contrastText: t.onBrand,
		ink: t.brandInk,
		tint: t.brandTint,
		onTint: t.brandOnTint,
	};

	return {
		primary: brand,
		success: brand,
		warning: {
			main: t.warnFill,
			ink: t.warnInk,
			tint: t.warnTint,
			onTint: t.warnOnTint,
		},
		error: {
			main: t.dangerFill,
			ink: t.dangerInk,
			tint: t.dangerTint,
			onTint: t.dangerOnTint,
		},
		background: { default: t.canvas, paper: t.paper },
		text: {
			primary: t.ink2,
			secondary: t.ink3,
			strong: t.ink1,
			tertiary: t.ink4,
		},
		divider: t.line,
		field: t.field,
		track: t.track,
		lineStrong: t.lineStrong,
		lineInput: t.lineInput,
		neutralFill: t.neutralFill,
		overlay: t.overlay,
	};
}

export const theme = createTheme(
	{
		cssVariables: { colorSchemeSelector: 'class' },
		colorSchemes: {
			light: { palette: palette('light') },
			dark: { palette: palette('dark') },
		},
		typography,
		shape: {
			borderRadius: 6,
			radius: { xs: 4, sm: 6, md: 8, lg: 12, xl: 14 },
		},
		// С ширины lg оболочка показывает боковую панель вместо рельса.
		breakpoints: { values: { xs: 0, sm: 600, md: 900, lg: 1200, xl: 1536 } },
		components,
	},
	// Русские подписи встроенных компонентов: пагинация, закрытие, пустые списки.
	ruRU
);
