import { useForm, useSelector } from '@tanstack/react-form';
import { useState } from 'react';

import type { Delivery, DeliveryItem } from '@entities/delivery';
import { shallowEqual } from '@shared/lib/shallow-equal';

import {
	decisionState,
	initialDecision,
	type Decision,
	type DecisionFlag,
	type Part,
} from './decision';

interface ApprovalValues {
	items: Decision[];
}

/** Форма решений по позициям поставки в порядке позиций; начальные решения считаются один раз при открытии. */
export function useApprovalForm(delivery: Delivery, now: Date) {
	// Новые значения по умолчанию сбросили бы нетронутую форму, поэтому они не пересчитываются со временем.
	const [defaultValues] = useState<ApprovalValues>(() => ({
		items: delivery.items.map((item) => initialDecision(item, now)),
	}));

	return useForm({ defaultValues });
}

export type ApprovalForm = ReturnType<typeof useApprovalForm>;

/** Решение одной позиции: компонент перерисовывается, только когда меняется оно. */
export function useDecision(form: ApprovalForm, index: number): Decision {
	return useSelector(form.store, (state) => state.values.items[index]);
}

const decisionStates = (items: DeliveryItem[], decisions: Decision[]) =>
	decisions.map((decision, index) => decisionState(items[index], decision));

/** Номера позиций с признаком `flag`: форма перерисовывается, только когда меняется список, а не при каждом вводе. */
export function useDecisionIndexes(
	form: ApprovalForm,
	items: DeliveryItem[],
	flag: DecisionFlag
): number[] {
	return useSelector(
		form.store,
		(state) =>
			decisionStates(items, state.values.items).flatMap((item, index) =>
				item[flag] ? [index] : []
			),
		{ compare: shallowEqual }
	);
}

/** Сколько позиций с каждым исходом: на склад, отклонено, в карантин. */
export function useOutcomeCounts(
	form: ApprovalForm,
	items: DeliveryItem[]
): Record<Part, number> {
	return useSelector(
		form.store,
		(state) => {
			const states = decisionStates(items, state.values.items);
			const count = (part: Part) => states.filter((item) => item[part]).length;

			return {
				accepted: count('accepted'),
				rejected: count('rejected'),
				quarantined: count('quarantined'),
			};
		},
		{ compare: shallowEqual }
	);
}

/** Все решения: для итога в открытом диалоге подтверждения. */
export function useDecisions(form: ApprovalForm): Decision[] {
	return useSelector(form.store, (state) => state.values.items);
}
