import { describe, expect, it } from 'vitest';

import {
	formatAmount,
	formatAmountDelta,
	formatAmountOf,
	formatPortion,
	formatQuantity,
	fromDisplay,
} from './quantity';

const nbsp = ' ';

describe('formatQuantity', () => {
	it.each([
		[2450, 'g', `2,45${nbsp}кг`],
		[4000, 'g', `4,0${nbsp}кг`],
		[20000, 'ml', `20,0${nbsp}л`],
		[1, 'g', `0,001${nbsp}кг`],
		[360, 'pcs', `360${nbsp}шт`],
	] as const)('%i %s показывается как «%s»', (value, unit, text) => {
		expect(formatQuantity(value, unit)).toBe(text);
	});
});

describe('fromDisplay', () => {
	it.each([
		[0.001, 'g', 1],
		[2.45, 'g', 2450],
		[0.3, 'ml', 300],
		[24, 'pcs', 24],
		[2.5, 'pcs', null],
	] as const)('ввод %d в %s даёт %s', (value, unit, expected) => {
		expect(fromDisplay(value, unit)).toBe(expected);
	});
});

describe('количества на складе', () => {
	it.each([
		[180, 'g', `180${nbsp}г`],
		[22000, 'g', `22${nbsp}кг`],
		[2450, 'g', `2,45${nbsp}кг`],
		[999, 'ml', `999${nbsp}мл`],
		[1000, 'ml', `1${nbsp}л`],
		[24, 'pcs', `24${nbsp}шт`],
	] as const)('%i %s показывается как «%s»', (value, unit, text) => {
		expect(formatAmount(value, unit)).toBe(text);
	});

	it.each([
		[1600, 2000, 'g', `1,6 из 2${nbsp}кг`],
		[300, 2000, 'g', `0,3 из 2${nbsp}кг`],
		[300, 800, 'g', `300 из 800${nbsp}г`],
		[12, 24, 'pcs', `12 из 24${nbsp}шт`],
	] as const)('остаток %i из %i %s — «%s»', (part, whole, unit, text) => {
		expect(formatAmountOf(part, whole, unit)).toBe(text);
	});

	it.each([
		[-200, 'g', `−200${nbsp}г`],
		[800, 'g', `+800${nbsp}г`],
		[1000, 'g', `+1${nbsp}кг`],
		[-3, 'pcs', `−3${nbsp}шт`],
	] as const)('изменение %i %s — «%s»', (delta, unit, text) => {
		expect(formatAmountDelta(delta, unit)).toBe(text);
	});
});

describe('количество на порцию', () => {
	it.each([
		[120, 'g', `120${nbsp}г`],
		[1200, 'g', `1${nbsp}200${nbsp}г`],
		[50, 'ml', `50${nbsp}мл`],
		[3, 'pcs', `3${nbsp}шт`],
	] as const)('%i %s — «%s»', (value, unit, text) => {
		expect(formatPortion(value, unit)).toBe(text);
	});
});
