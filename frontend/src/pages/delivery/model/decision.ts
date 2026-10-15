import {
	expiredComment,
	type DeliveryItem,
	type Distribution,
	type WriteOffReason,
} from '@entities/delivery';
import type { ApiError, components } from '@shared/api';
import { isExpired } from '@shared/lib/expiry';

export type DeliveryApproval = components['schemas']['DeliveryApproval'];

/** Части решения по позиции в порядке показа. */
export const parts = ['accepted', 'rejected', 'quarantined'] as const;

export type Part = (typeof parts)[number];

/** Поле количества в API для каждой части решения. */
export const quantityOf: Record<Part, keyof Distribution> = {
	accepted: 'accepted_quantity',
	rejected: 'rejected_quantity',
	quarantined: 'quarantined_quantity',
};

/** Решение по позиции; количества целые в единице продукта, `null` — пустое поле. */
export interface Decision {
	accepted: number | null;
	rejected: number | null;
	quarantined: number | null;
	rejectReason: WriteOffReason | '';
	quarantineReason: WriteOffReason | '';
	comment: string;
	/** Позиция просрочена: по часам экрана или по ответу сервера. */
	expired: boolean;
	/** Причина отклонения проставлена автоматически, потому что срок истёк. */
	autoReason: boolean;
	/** Сколько было принято до того, как позиция стала просроченной. */
	previousAccepted?: number;
}

export type DecisionProblem =
	| 'unallocated'
	| 'over'
	| 'accepted-expired'
	| 'reject-reason'
	| 'quarantine-reason'
	| 'comment';

/** Решение при открытии формы: принять заявленное, просроченное — отклонить с причиной и комментарием. */
export function initialDecision(item: DeliveryItem, now: Date): Decision {
	if (isExpired(item.expires_at, now)) {
		return {
			accepted: 0,
			rejected: item.declared_quantity,
			quarantined: 0,
			rejectReason: 'expired',
			quarantineReason: '',
			comment: expiredComment(item.expires_at),
			expired: true,
			autoReason: true,
		};
	}

	return {
		accepted: item.declared_quantity,
		rejected: 0,
		quarantined: 0,
		rejectReason: '',
		quarantineReason: '',
		comment: '',
		expired: false,
		autoReason: false,
	};
}

/** Позиция стала просроченной во время заполнения: принятое обнуляется, прежнее значение запоминается. */
export function expireDecision(decision: Decision): Decision {
	if (decision.expired) {
		return decision;
	}

	const accepted = decision.accepted ?? 0;

	return {
		...decision,
		expired: true,
		accepted: 0,
		previousAccepted: accepted > 0 ? accepted : undefined,
	};
}

/** Сколько распределено между частями; пустое поле считается нулём. */
export function allocated(
	decision: Pick<Decision, 'accepted' | 'rejected' | 'quarantined'>
): number {
	return (
		(decision.accepted ?? 0) +
		(decision.rejected ?? 0) +
		(decision.quarantined ?? 0)
	);
}

/** Что мешает считать позицию решённой; пустой список — позиция решена. */
export function decisionProblems(
	item: DeliveryItem,
	decision: Decision
): DecisionProblem[] {
	const problems: DecisionProblem[] = [];
	const sum = allocated(decision);
	const rejected = decision.rejected ?? 0;
	const quarantined = decision.quarantined ?? 0;

	if (sum < item.declared_quantity) {
		problems.push('unallocated');
	}

	if (sum > item.declared_quantity) {
		problems.push('over');
	}

	if (decision.expired && (decision.accepted ?? 0) > 0) {
		problems.push('accepted-expired');
	}

	if (rejected > 0 && !decision.rejectReason) {
		problems.push('reject-reason');
	}

	if (quarantined > 0 && !decision.quarantineReason) {
		problems.push('quarantine-reason');
	}

	if ((rejected > 0 || quarantined > 0) && !decision.comment.trim()) {
		problems.push('comment');
	}

	return problems;
}

/** Тело запроса приёмки: причины и комментарий только у ненулевых частей. */
export function approvalBody(
	items: DeliveryItem[],
	decisions: Decision[]
): DeliveryApproval {
	return {
		items: items.map((item, index) => {
			const decision = decisions[index];
			const rejected = decision.rejected ?? 0;
			const quarantined = decision.quarantined ?? 0;

			return {
				item_id: item.id,
				accepted_quantity: decision.accepted ?? 0,
				rejected_quantity: rejected,
				quarantined_quantity: quarantined,
				reject_reason:
					rejected > 0 && decision.rejectReason
						? decision.rejectReason
						: undefined,
				quarantine_reason:
					quarantined > 0 && decision.quarantineReason
						? decision.quarantineReason
						: undefined,
				comment:
					rejected > 0 || quarantined > 0 ? decision.comment.trim() : undefined,
			};
		}),
	};
}

/** Позиция из адреса ошибки поля `items[<id>].<поле>`. */
function itemOfField(field: string): string | undefined {
	return /^items\[([^\]]+)\]\./.exec(field)?.[1];
}

/** Ошибки полей позиций из ответа сервера вместе с идентификатором позиции. */
export function itemFieldErrors(error: ApiError) {
	return Object.entries(error.fieldErrors).flatMap(([field, fieldError]) => {
		const id = itemOfField(field);

		return id ? [{ id, ...fieldError }] : [];
	});
}

/** Что форме нужно знать о позиции для фильтра, счётчиков и итога, без самих значений. */
interface DecisionState {
	/** Позиция ещё требует решения. */
	pending: boolean;
	expired: boolean;
	/** Срок истёк, пока заполнялась приёмка, а не был истёкшим при открытии. */
	expiredDuringFill: boolean;
	accepted: boolean;
	rejected: boolean;
	quarantined: boolean;
}

export type DecisionFlag = keyof DecisionState;

export function decisionState(
	item: DeliveryItem,
	decision: Decision
): DecisionState {
	return {
		pending: decisionProblems(item, decision).length > 0,
		expired: decision.expired,
		expiredDuringFill: decision.expired && !decision.autoReason,
		accepted: (decision.accepted ?? 0) > 0,
		rejected: (decision.rejected ?? 0) > 0,
		quarantined: (decision.quarantined ?? 0) > 0,
	};
}
