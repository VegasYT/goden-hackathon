import type { ThemeOptions } from '@mui/material/styles';

/** Медиазапрос сенсорного ввода: планшет получает крупные элементы и текст. */
export const coarsePointer = '@media (pointer: coarse)';

const rem = (px: number) => `${px / 16}rem`;

const fontFamily = '"Golos Text Variable", system-ui, sans-serif';

export const typography: ThemeOptions['typography'] = {
	fontFamily,
	htmlFontSize: 16,
	fontSize: 14,
	fontWeightLight: 400,
	fontWeightRegular: 400,
	fontWeightMedium: 500,
	fontWeightBold: 600,
	h1: {
		fontSize: rem(26),
		fontWeight: 600,
		lineHeight: 1.2,
		letterSpacing: '-0.02em',
	},
	h2: {
		fontSize: rem(20),
		fontWeight: 600,
		lineHeight: 1.3,
		letterSpacing: '-0.01em',
	},
	h3: {
		fontSize: rem(15),
		fontWeight: 600,
		lineHeight: 1.4,
		letterSpacing: 0,
		[coarsePointer]: { fontSize: rem(17) },
	},
	body1: {
		fontSize: rem(14),
		lineHeight: 1.45,
		letterSpacing: 0,
		[coarsePointer]: { fontSize: rem(15) },
	},
	body2: {
		fontSize: rem(13),
		lineHeight: 1.45,
		letterSpacing: 0,
		[coarsePointer]: { fontSize: rem(14) },
	},
	caption: {
		fontSize: rem(12),
		lineHeight: 1.4,
		letterSpacing: 0,
		[coarsePointer]: { fontSize: rem(13) },
	},
	button: {
		fontSize: rem(14),
		fontWeight: 600,
		lineHeight: 1.2,
		letterSpacing: 0,
		textTransform: 'none',
		[coarsePointer]: { fontSize: rem(16) },
	},
};
