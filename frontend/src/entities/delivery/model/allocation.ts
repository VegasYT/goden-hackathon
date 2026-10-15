import { formatQuantity, type QuantityUnit } from '@shared/lib/quantity';

const overTexts = {
	declared: 'больше заявленного',
	quarantine: 'больше, чем в карантине',
};

/** Что сказать, если распределено не всё количество позиции (заявленное или в карантине) или больше него; `undefined` — сумма сходится. */
export function allocationText(
	total: number,
	sum: number,
	unit: QuantityUnit,
	limit: keyof typeof overTexts
): string | undefined {
	if (sum < total) {
		return sum === 0
			? `Распределите ${formatQuantity(total, unit)} между полями`
			: `Осталось распределить ${formatQuantity(total - sum, unit)}`;
	}

	if (sum > total) {
		return `Распределено на ${formatQuantity(sum - total, unit)} ${overTexts[limit]}`;
	}

	return undefined;
}
