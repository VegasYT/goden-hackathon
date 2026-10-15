import { useForm, useSelector } from '@tanstack/react-form';
import { useState } from 'react';

import type { Delivery, DeliveryItem } from '@entities/delivery';
import { shallowEqual } from '@shared/lib/shallow-equal';

import {
	correctionProblems,
	initialCorrection,
	isChanged,
	type CorrectionProblem,
	type CorrectionValue,
	type ServerHint,
} from './correction';

interface CorrectionValues {
	items: CorrectionValue[];
	reason: string;
}

/** Форма исправления: распределение по позициям в порядке позиций и причина; начальные значения считаются один раз. */
export function useCorrectionForm(delivery: Delivery) {
	const [defaultValues] = useState<CorrectionValues>(() => ({
		items: delivery.items.map(initialCorrection),
		reason: '',
	}));

	return useForm({ defaultValues });
}

export type CorrectionForm = ReturnType<typeof useCorrectionForm>;

/** Значение одной позиции: строка перерисовывается, только когда меняется оно. */
export function useCorrectionValue(
	form: CorrectionForm,
	index: number
): CorrectionValue {
	return useSelector(form.store, (state) => state.values.items[index]);
}

/** Позиция изменена и с ошибкой ли она: строка и страница считают это по одному правилу. */
export interface CorrectionState {
	changed: boolean;
	problems: CorrectionProblem[];
}

export function correctionState(
	item: DeliveryItem,
	value: CorrectionValue,
	now: Date,
	hint?: ServerHint
): CorrectionState {
	return {
		changed: isChanged(item, value),
		problems: correctionProblems(item, value, now, hint),
	};
}

interface SummaryScope {
	form: CorrectionForm;
	items: DeliveryItem[];
	now: Date;
	hints: Record<string, ServerHint>;
}

function useCorrectionIndexes(
	{ form, items, now, hints }: SummaryScope,
	matches: (state: CorrectionState) => boolean
): number[] {
	return useSelector(
		form.store,
		(state) =>
			state.values.items.flatMap((value, index) =>
				matches(
					correctionState(items[index], value, now, hints[items[index].id])
				)
					? [index]
					: []
			),
		{ compare: shallowEqual }
	);
}

/** Номера изменённых позиций и позиций с ошибкой; страница перерисовывается, только когда они меняются. */
export function useCorrectionSummary(
	form: CorrectionForm,
	items: DeliveryItem[],
	now: Date,
	hints: Record<string, ServerHint>
) {
	const scope = { form, items, now, hints };
	const changed = useCorrectionIndexes(scope, (state) => state.changed);
	const errors = useCorrectionIndexes(
		scope,
		(state) => state.problems.length > 0
	);

	return { changed, errors };
}
