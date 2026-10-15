import { formatQuantityValue, type QuantityUnit } from '@shared/lib/quantity';
import type { IconName } from '@shared/ui/icon';

import type { Part } from '../model/decision';

/** Подписи полей решения по позиции. */
export const partLabels: Record<Part, string> = {
	accepted: 'Принять',
	rejected: 'Отклонить',
	quarantined: 'Карантин',
};

/** Исходы приёмки для итога: часть решения, подпись и иконка. */
export const outcomeParts: { part: Part; label: string; icon: IconName }[] = [
	{ part: 'accepted', label: 'На склад', icon: 'toStock' },
	{ part: 'rejected', label: 'Отклонено', icon: 'rejected' },
	{ part: 'quarantined', label: 'В карантин', icon: 'quarantine' },
];

/** Количество без единицы для «было … → стало»; ноль — просто «0». */
export function formatPartValue(quantity: number, unit: QuantityUnit): string {
	return quantity === 0 ? '0' : formatQuantityValue(quantity, unit);
}
