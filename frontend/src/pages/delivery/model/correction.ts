import type { DeliveryItem } from '@entities/delivery';
import type { components } from '@shared/api';
import { isExpired } from '@shared/lib/expiry';

import { allocated, parts, quantityOf, type Part } from './decision';
import { distributionChange } from './history';

export type ApprovalCorrection = components['schemas']['ApprovalCorrection'];

/** Новое распределение позиции; количества целые в единице продукта, `null` — пустое поле. */
export type CorrectionValue = Record<Part, number | null>;

export type CorrectionProblem =
	'unallocated' | 'over' | 'stock-consumed' | 'accepted-expired';

/** Начальные значения — текущее распределение позиции. */
export function initialCorrection(item: DeliveryItem): CorrectionValue {
	return {
		accepted: item.accepted_quantity,
		rejected: item.rejected_quantity,
		quarantined: item.quarantined_quantity,
	};
}

/** Что сервер сообщил о позиции при отказе: предел уменьшения принятого и истёкший срок. */
export interface ServerHint {
	limit?: number;
	expired?: boolean;
}

/** На сколько можно уменьшить принятое: остаток партий позиции, но не больше предела сервера. */
export function reductionLimit(
	item: DeliveryItem,
	hint: ServerHint = {}
): number {
	return Math.min(item.in_stock_quantity, hint.limit ?? Infinity);
}

/** Срок позиции истёк по часам экрана или по ответу сервера. */
export function isItemExpired(
	item: DeliveryItem,
	now: Date,
	hint: ServerHint = {}
): boolean {
	return hint.expired === true || isExpired(item.expires_at, now);
}

/** Что мешает сохранить позицию; пустой список — позиция в порядке. */
export function correctionProblems(
	item: DeliveryItem,
	value: CorrectionValue,
	now: Date,
	hint: ServerHint = {}
): CorrectionProblem[] {
	const problems: CorrectionProblem[] = [];
	const sum = allocated(value);
	const accepted = value.accepted ?? 0;

	if (sum < item.declared_quantity) {
		problems.push('unallocated');
	}

	if (sum > item.declared_quantity) {
		problems.push('over');
	}

	if (item.accepted_quantity - accepted > reductionLimit(item, hint)) {
		problems.push('stock-consumed');
	}

	if (accepted > item.accepted_quantity && isItemExpired(item, now, hint)) {
		problems.push('accepted-expired');
	}

	return problems;
}

export function isChanged(item: DeliveryItem, value: CorrectionValue): boolean {
	return parts.some((part) => (value[part] ?? 0) !== item[quantityOf[part]]);
}

/** Распределение в форме ответа API: пустое поле считается нулём. */
function asDistribution(value: CorrectionValue) {
	return {
		accepted_quantity: value.accepted ?? 0,
		rejected_quantity: value.rejected ?? 0,
		quarantined_quantity: value.quarantined ?? 0,
	};
}

/** Что изменится у позиции, если сохранить новое распределение. */
export function plannedChange(item: DeliveryItem, value: CorrectionValue) {
	return distributionChange(item, asDistribution(value));
}

/** Тело запроса исправления: только изменённые позиции и причина. */
export function correctionBody(
	items: DeliveryItem[],
	values: CorrectionValue[],
	reason: string
): ApprovalCorrection {
	return {
		reason: reason.trim(),
		items: items.flatMap((item, index) =>
			isChanged(item, values[index])
				? [
						{
							item_id: item.id,
							...asDistribution(values[index]),
						},
					]
				: []
		),
	};
}
