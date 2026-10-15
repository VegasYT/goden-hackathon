/** Единица продукта в API: количество всегда целое в ней. */
export type QuantityUnit = 'g' | 'ml' | 'pcs';

const nbsp = ' ';

const displayUnits: Record<QuantityUnit, string> = {
	g: 'кг',
	ml: 'л',
	pcs: 'шт',
};

const fractionFormat = new Intl.NumberFormat('ru-RU', {
	minimumFractionDigits: 1,
	maximumFractionDigits: 3,
});
const integerFormat = new Intl.NumberFormat('ru-RU', {
	maximumFractionDigits: 0,
});

const smallUnits: Record<Exclude<QuantityUnit, 'pcs'>, string> = {
	g: 'г',
	ml: 'мл',
};

const amountFormat = new Intl.NumberFormat('ru-RU', {
	maximumFractionDigits: 3,
});

/** Подпись единицы для показа и полей ввода: «кг», «л», «шт». */
function displayUnit(unit: QuantityUnit): string {
	return displayUnits[unit];
}

/** Количество из единицы продукта в единицу показа: 2450 г — 2,45 кг. */
export function toDisplay(value: number, unit: QuantityUnit): number {
	return unit === 'pcs' ? value : value / 1000;
}

/** Ввод в единице показа обратно в целую единицу продукта; дробные штуки не принимаются. */
export function fromDisplay(value: number, unit: QuantityUnit): number | null {
	if (unit === 'pcs') {
		return Number.isInteger(value) ? value : null;
	}

	return Math.round(value * 1000);
}

/** Число без единицы: «2,45», «4,0», «360». */
export function formatQuantityValue(value: number, unit: QuantityUnit): string {
	const shown = toDisplay(value, unit);

	return unit === 'pcs'
		? integerFormat.format(shown)
		: fractionFormat.format(shown);
}

/** Количество с единицей: «2,45 кг», «20,0 л», «360 шт». */
export function formatQuantity(value: number, unit: QuantityUnit): string {
	return `${formatQuantityValue(value, unit)}${nbsp}${displayUnit(unit)}`;
}

/** Настройки поля ввода количества в единице показа. */
export function quantityInput(unit: QuantityUnit) {
	return unit === 'pcs'
		? { unit: displayUnit(unit), step: 1, format: { maximumFractionDigits: 0 } }
		: {
				unit: displayUnit(unit),
				step: 0.1,
				format: { minimumFractionDigits: 1, maximumFractionDigits: 3 },
			};
}

/** Показывать ли количество в граммах и миллилитрах: меньше 1 кг или 1 л. */
function isSmall(value: number, unit: QuantityUnit): boolean {
	return unit !== 'pcs' && Math.abs(value) < 1000;
}

/** Число и подпись количества в выбранном масштабе. */
function amountIn(value: number, unit: QuantityUnit, small: boolean) {
	if (unit !== 'pcs' && small) {
		return { number: integerFormat.format(value), unit: smallUnits[unit] };
	}

	return {
		number: amountFormat.format(toDisplay(value, unit)),
		unit: displayUnit(unit),
	};
}

/** Количество на складе: меньше 1 кг или 1 л — в граммах и миллилитрах, иначе без нулей в конце: «180 г», «2,45 кг», «22 кг». */
export function formatAmount(value: number, unit: QuantityUnit): string {
	const amount = amountIn(value, unit, isSmall(value, unit));

	return `${amount.number}${nbsp}${amount.unit}`;
}

/** Остаток из принятого в единице принятого: «1,6 из 2 кг», «300 из 800 г». */
export function formatAmountOf(
	part: number,
	whole: number,
	unit: QuantityUnit
): string {
	const small = isSmall(whole, unit);
	const of = amountIn(whole, unit, small);

	return `${amountIn(part, unit, small).number} из ${of.number}${nbsp}${of.unit}`;
}

/** Изменение остатка со знаком: «−200 г», «+1 кг». */
export function formatAmountDelta(delta: number, unit: QuantityUnit): string {
	return `${delta < 0 ? '−' : '+'}${formatAmount(Math.abs(delta), unit)}`;
}

const baseUnits: Record<QuantityUnit, string> = { ...smallUnits, pcs: 'шт' };

/** Подпись единицы продукта без пересчёта: «г», «мл», «шт». */
export function baseUnitLabel(unit: QuantityUnit): string {
	return baseUnits[unit];
}

/** Количество на порцию в единице продукта без пересчёта: «120 г», «1 200 г», «3 шт». */
export function formatPortion(value: number, unit: QuantityUnit): string {
	return `${integerFormat.format(value)}${nbsp}${baseUnits[unit]}`;
}
