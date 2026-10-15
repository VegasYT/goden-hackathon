import { reasonNeedsComment, type WriteOffReason } from '@entities/delivery';
import { isExpired } from '@shared/lib/expiry';
import { formatAmount, type QuantityUnit } from '@shared/lib/quantity';

import type { Adjustment, Batch, WriteOff } from '../api/stock';

export interface WriteOffValues {
	batchId: string;
	/** Количество в единице продукта: граммы, миллилитры или штуки. */
	quantity: number | null;
	reason: WriteOffReason | null;
	comment: string;
}

interface WriteOffErrors {
	quantity?: string;
	reason?: string;
	comment?: string;
}

export interface AdjustValues {
	/** Сколько партии лежит на самом деле, в единице продукта. */
	actual: number | null;
	reason: string;
	comment: string;
}

interface AdjustErrors {
	actual?: string;
	reason?: string;
}

/** Частые причины исправления остатка; выбор подставляет текст в поле причины. */
export const adjustReasons = ['Пересчёт', 'Другое'];

/** Годные партии с остатком в порядке расхода: их расходуют заказы, первую — раньше других. */
export function usableBatches(batches: Batch[], now: Date): Batch[] {
	return batches.filter(
		(batch) => batch.quantity > 0 && !isExpired(batch.expires_at, now)
	);
}

/** Партии с остатком, годные и просроченные, в порядке расхода: из них списывают. */
export function stockedBatches(batches: Batch[]): Batch[] {
	return batches.filter((batch) => batch.quantity > 0);
}

/** Начальные значения списания: у просроченной партии — причина «Истёк срок годности» и весь остаток. */
export function initialWriteOff(batch: Batch, now: Date): WriteOffValues {
	const expired = isExpired(batch.expires_at, now);

	return {
		batchId: batch.id,
		quantity: expired ? batch.quantity : null,
		reason: expired ? 'expired' : null,
		comment: '',
	};
}

/** Доступное количество продукта (годные партии) и число партий с остатком. */
export function productStock(batches: Batch[], now: Date) {
	return {
		available: usableBatches(batches, now).reduce(
			(sum, batch) => sum + batch.quantity,
			0
		),
		batches: stockedBatches(batches).length,
	};
}

/** Партия, которую заказы расходуют первой. */
export function firstInLine(batches: Batch[], now: Date): string | undefined {
	return usableBatches(batches, now)[0]?.id;
}

/** Текущий остаток партии по перечитанному списку; израсходованной партии в нём уже нет. */
export function liveQuantity(batch: Batch, live: Batch[] | undefined): number {
	if (!live) {
		return batch.quantity;
	}

	return live.find(({ id }) => id === batch.id)?.quantity ?? 0;
}

export function writeOffProblems(
	values: WriteOffValues,
	batch: Batch,
	unit: QuantityUnit,
	now: Date
): WriteOffErrors {
	const errors: WriteOffErrors = {};

	if (values.quantity === null || values.quantity <= 0) {
		errors.quantity = 'Укажите, сколько списать';
	} else if (values.quantity > batch.quantity) {
		errors.quantity = `В партии осталось ${formatAmount(batch.quantity, unit)} — меньше, чем вы списываете`;
	}

	if (values.reason === null) {
		errors.reason = 'Выберите причину';
	} else if (values.reason === 'expired' && !isExpired(batch.expires_at, now)) {
		errors.reason = 'Срок годности партии ещё не истёк';
	}

	if (reasonNeedsComment(values.reason) && values.comment.trim() === '') {
		errors.comment = 'Напишите, что случилось с продуктом';
	}

	return errors;
}

export function toWriteOff(values: WriteOffValues): WriteOff {
	return {
		quantity: values.quantity ?? 0,
		reason: values.reason ?? 'other',
		comment: values.comment.trim(),
	};
}

/** Совпадающее с остатком количество не проверяется: с ним кнопка исправления неактивна. */
export function adjustProblems(values: AdjustValues): AdjustErrors {
	const errors: AdjustErrors = {};

	if (values.actual === null) {
		errors.actual = 'Укажите, сколько партии лежит на самом деле';
	}

	if (values.reason.trim() === '') {
		errors.reason = 'Укажите причину';
	}

	return errors;
}

/** Изменение остатка, которое даёт введённое фактическое количество. */
export function adjustDelta(values: AdjustValues, batch: Batch): number {
	return values.actual === null ? 0 : values.actual - batch.quantity;
}

export function toAdjustment(values: AdjustValues, batch: Batch): Adjustment {
	const comment = values.comment.trim();

	return {
		delta: adjustDelta(values, batch),
		reason: values.reason.trim(),
		...(comment && { comment }),
	};
}

export function hasProblems(errors: object): boolean {
	return Object.values(errors).some(Boolean);
}
