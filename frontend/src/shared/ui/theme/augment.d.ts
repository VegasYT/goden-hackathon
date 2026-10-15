import '@mui/material/styles';
import '@mui/material/Typography';
import '@mui/x-date-pickers/themeAugmentation';

// Типы MUI для токенов из tokens.ts.

interface RadiusScale {
	/** Чипы. */
	xs: number;
	/** Кнопки и поля на ноутбуке. */
	sm: number;
	/** Поля на планшете, пункты навигации. */
	md: number;
	/** Карточки. */
	lg: number;
	/** Диалоги. */
	xl: number;
}

declare module '@mui/material/styles' {
	interface PaletteColor {
		/** Цвет текста и иконок этого оттенка на фоне страницы. */
		ink?: string;
		/** Подложка плашек и чипов. */
		tint?: string;
		/** Текст на подложке `tint`. */
		onTint?: string;
	}

	interface SimplePaletteColorOptions {
		ink?: string;
		tint?: string;
		onTint?: string;
	}

	interface TypeText {
		/** Заголовки и значения. */
		strong: string;
		/** Метаданные и подсказки. */
		tertiary: string;
	}

	interface Palette {
		field: string;
		track: string;
		lineStrong: string;
		lineInput: string;
		neutralFill: string;
		overlay: string;
	}

	interface PaletteOptions {
		field?: string;
		track?: string;
		lineStrong?: string;
		lineInput?: string;
		neutralFill?: string;
		overlay?: string;
	}

	interface Shape {
		radius: RadiusScale;
	}

	interface ShapeOptions {
		radius?: RadiusScale;
	}
}

// Лишние варианты отключены, потому что иерархию дают h1–h3 и body.
declare module '@mui/material/Typography' {
	interface TypographyPropsVariantOverrides {
		h4: false;
		h5: false;
		h6: false;
		subtitle1: false;
		subtitle2: false;
		overline: false;
	}
}
