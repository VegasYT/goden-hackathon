import { describe, expect, it } from 'vitest';

import {
	blend,
	contrastRatio,
	findContrastViolations,
	formatViolation,
	parseColor,
} from './contrast';
import { colorTokens } from './tokens';

describe('contrast', () => {
	it('чёрный на белом даёт 21:1', () => {
		expect(contrastRatio('#000000', '#FFFFFF')).toBeCloseTo(21, 5);
	});

	it('полупрозрачный чёрный на белом смешивается в серый', () => {
		expect(blend('rgba(0,0,0,.5)', '#FFFFFF')).toBe('#808080');
	});

	it('полупрозрачный фон накладывается на подложку', () => {
		expect(contrastRatio('#FFFFFF', 'rgba(0,0,0,.5)', '#FFFFFF')).toBeCloseTo(
			contrastRatio('#FFFFFF', '#808080'),
			5
		);
	});

	it('разбирает короткий hex', () => {
		expect(parseColor('#fff')).toEqual({ r: 255, g: 255, b: 255, a: 1 });
	});

	it('не принимает неизвестный формат', () => {
		expect(() => parseColor('white')).toThrow(/Неподдерживаемый формат/);
	});
});

describe('контраст палитры', () => {
	it.each(['light', 'dark'] as const)(
		'все пары темы %s проходят пороги WCAG AA',
		(scheme) => {
			const violations = findContrastViolations(scheme, colorTokens[scheme]);

			expect(violations.map(formatViolation)).toEqual([]);
		}
	);

	it('пара ниже порога попадает в нарушения с именем и значением', () => {
		const tokens = { ...colorTokens.light, ink3: '#B0B8B4' };
		const [violation] = findContrastViolations('light', tokens, [
			{ name: 'ink3 на paper', fg: 'ink3', bg: 'paper', min: 4.5 },
		]);

		expect(violation).toMatchObject({ scheme: 'light', name: 'ink3 на paper' });
		expect(formatViolation(violation)).toMatch(
			/^light: ink3 на paper — \d+\.\d{2}:1, нужно не ниже 4\.5:1$/
		);
	});
});
