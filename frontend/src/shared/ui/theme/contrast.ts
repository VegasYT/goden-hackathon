// Расчёт контраста по WCAG 2.x: https://www.w3.org/TR/WCAG21/#dfn-contrast-ratio

import type { ColorSchemeName, ColorTokenName, ColorTokens } from './tokens';

export interface Rgba {
	r: number;
	g: number;
	b: number;
	a: number;
}

const HEX = /^#([0-9a-f]{3}|[0-9a-f]{6})$/i;
const RGBA =
	/^rgba?\(\s*([\d.]+)\s*,\s*([\d.]+)\s*,\s*([\d.]+)\s*(?:,\s*([\d.]+)\s*)?\)$/i;

export function parseColor(color: string): Rgba {
	const value = color.trim();
	const hex = HEX.exec(value);

	if (hex) {
		const digits =
			hex[1].length === 3 ? [...hex[1]].map((d) => d + d).join('') : hex[1];

		return {
			r: parseInt(digits.slice(0, 2), 16),
			g: parseInt(digits.slice(2, 4), 16),
			b: parseInt(digits.slice(4, 6), 16),
			a: 1,
		};
	}

	const rgba = RGBA.exec(value);

	if (rgba) {
		return {
			r: Number(rgba[1]),
			g: Number(rgba[2]),
			b: Number(rgba[3]),
			a: rgba[4] === undefined ? 1 : Number(rgba[4]),
		};
	}

	throw new Error(`Неподдерживаемый формат цвета: ${color}`);
}

function toHex({ r, g, b }: Rgba): string {
	return `#${[r, g, b]
		.map((c) => Math.round(c).toString(16).padStart(2, '0'))
		.join('')
		.toUpperCase()}`;
}

/** Накладывает (полупрозрачный) цвет на непрозрачный фон и возвращает итоговый hex. */
export function blend(color: string, background: string): string {
	const fg = parseColor(color);
	const bg = parseColor(background);

	if (bg.a !== 1) {
		throw new Error(`Фон должен быть непрозрачным: ${background}`);
	}

	return toHex({
		r: fg.r * fg.a + bg.r * (1 - fg.a),
		g: fg.g * fg.a + bg.g * (1 - fg.a),
		b: fg.b * fg.a + bg.b * (1 - fg.a),
		a: 1,
	});
}

function channel(value: number): number {
	const c = value / 255;

	return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
}

export function relativeLuminance(color: string): number {
	const { r, g, b, a } = parseColor(color);

	if (a !== 1) {
		throw new Error(
			`Яркость считается только для непрозрачного цвета: ${color}`
		);
	}

	return 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b);
}

/** Контраст цвета на фоне; полупрозрачный фон сначала кладётся на `base`, по умолчанию белый. */
export function contrastRatio(
	foreground: string,
	background: string,
	base?: string
): number {
	const bg =
		parseColor(background).a === 1
			? background
			: blend(background, base ?? '#FFFFFF');
	const fg =
		parseColor(foreground).a === 1 ? foreground : blend(foreground, bg);
	const [hi, lo] = [relativeLuminance(fg), relativeLuminance(bg)].sort(
		(x, y) => y - x
	);

	return (hi + 0.05) / (lo + 0.05);
}

export interface ContrastPair {
	name: string;
	fg: ColorTokenName;
	bg: ColorTokenName;
	min: number;
}

const TEXT = 4.5;
const GRAPHIC = 3;

const pairsOn =
	(min: number) =>
	(bg: ColorTokenName, fgs: ColorTokenName[]): ContrastPair[] =>
		fgs.map((fg) => ({ name: `${fg} на ${bg}`, fg, bg, min }));
const textOn = pairsOn(TEXT);
const graphicOn = pairsOn(GRAPHIC);

/** Пары «цвет на фоне» из интерфейса, которые автотест держит не ниже `min`. */
export const contrastPairs: ContrastPair[] = [
	...textOn('paper', [
		'ink1',
		'ink2',
		'ink3',
		'ink4',
		'brandInk',
		'warnInk',
		'dangerInk',
	]),
	...textOn('canvas', ['ink1', 'ink2', 'ink3', 'ink4']),
	...textOn('track', ['ink1', 'ink2', 'ink3']),
	...textOn('field', ['ink1']),
	...textOn('brand', ['onBrand']),
	...textOn('brandTint', ['brandOnTint']),
	...textOn('warnTint', ['warnOnTint']),
	...textOn('dangerTint', ['dangerOnTint']),
	...graphicOn('paper', ['lineInput', 'warnFill', 'dangerFill', 'neutralFill']),
	...graphicOn('field', ['lineInput', 'ink3']),
	...graphicOn('canvas', ['lineInput', 'brandInk']),
];

/** Контраст пары в палитре; полупрозрачный фон кладётся на `paper`. */
export function pairRatio(pair: ContrastPair, tokens: ColorTokens): number {
	return contrastRatio(tokens[pair.fg], tokens[pair.bg], tokens.paper);
}

export interface ContrastViolation {
	scheme: ColorSchemeName;
	name: string;
	ratio: number;
	min: number;
}

export function findContrastViolations(
	scheme: ColorSchemeName,
	tokens: ColorTokens,
	pairs: ContrastPair[] = contrastPairs
): ContrastViolation[] {
	return pairs.flatMap((pair) => {
		const ratio = pairRatio(pair, tokens);

		return ratio < pair.min
			? [{ scheme, name: pair.name, ratio, min: pair.min }]
			: [];
	});
}

export function formatViolation(v: ContrastViolation): string {
	return `${v.scheme}: ${v.name} — ${v.ratio.toFixed(2)}:1, нужно не ниже ${v.min}:1`;
}
