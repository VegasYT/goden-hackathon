import { actorName } from '@entities/actor';
import {
	expiredComment,
	itemName,
	type Delivery,
	type DeliveryItem,
	type WriteOffReason,
} from '@entities/delivery';
import type { components } from '@shared/api';
import { dayMs, formatTime } from '@shared/lib/datetime';
import { isExpired } from '@shared/lib/expiry';
import { countOf } from '@shared/lib/plural';

export type QuarantineResolutionBody =
	components['schemas']['QuarantineResolution'];

/** Позиция в карантине без решения вместе с её поставкой. */
export interface OpenQuarantineItem {
	delivery: Delivery;
	item: DeliveryItem;
	/** Когда открыт карантин, в миллисекундах. */
	openedAt: number;
}

/** Решение по карантину позиции; количества целые в единице продукта, `null` — пустое поле. */
export interface Resolution {
	accepted: number | null;
	rejected: number | null;
	rejectReason: WriteOffReason | '';
	comment: string;
	/** Срок позиции истёк, поэтому причина отклонения проставлена автоматически. */
	expired: boolean;
}

export type ResolutionProblem =
	'unallocated' | 'over' | 'accepted-expired' | 'reject-reason' | 'comment';

/** `accept` — всё на склад, `reject` — всё отклонить, `split` — разделить. */
export type ResolutionOutcome = 'accept' | 'reject' | 'split';

/** Позиции с карантином без решения из страницы списка: первыми те, что ждут дольше. */
export function openQuarantineItems(
	deliveries: Delivery[]
): OpenQuarantineItem[] {
	return deliveries
		.flatMap((delivery) =>
			delivery.items
				.filter((item) => item.quarantined_quantity > 0)
				.map((item) => ({
					delivery,
					item,
					openedAt: new Date(
						item.quarantined_at ?? delivery.approved_at ?? delivery.delivered_at
					).getTime(),
				}))
		)
		.sort((a, b) => a.openedAt - b.openedAt);
}

/** Сколько позиция ждёт решения: «в карантине меньше дня», «в карантине 2 дня». */
export function quarantineAge(entry: OpenQuarantineItem, now: Date): string {
	const days = Math.floor((now.getTime() - entry.openedAt) / dayMs);

	return days < 1
		? 'в карантине меньше дня'
		: `в карантине ${countOf(days, 'день', 'дня', 'дней')}`;
}

/** Срок истёк: принять нельзя, поэтому всё количество отклоняется с причиной и комментарием о сроке. */
export function expiredResolution(item: DeliveryItem): Resolution {
	return {
		accepted: 0,
		rejected: item.quarantined_quantity,
		rejectReason: 'expired',
		comment: expiredComment(item.expires_at),
		expired: true,
	};
}

/** Решение при открытии: принять всё, у просроченной позиции — отклонить всё. */
export function initialResolution(item: DeliveryItem, now: Date): Resolution {
	if (isExpired(item.expires_at, now)) {
		return expiredResolution(item);
	}

	return {
		accepted: item.quarantined_quantity,
		rejected: 0,
		rejectReason: '',
		comment: '',
		expired: false,
	};
}

/** Что мешает принять решение; пустой список — решение можно отправить. */
export function resolutionProblems(
	item: DeliveryItem,
	resolution: Resolution
): ResolutionProblem[] {
	const problems: ResolutionProblem[] = [];
	const accepted = resolution.accepted ?? 0;
	const rejected = resolution.rejected ?? 0;
	const sum = accepted + rejected;

	if (sum < item.quarantined_quantity) {
		problems.push('unallocated');
	}

	if (sum > item.quarantined_quantity) {
		problems.push('over');
	}

	if (resolution.expired && accepted > 0) {
		problems.push('accepted-expired');
	}

	if (rejected > 0 && !resolution.rejectReason) {
		problems.push('reject-reason');
	}

	if (rejected > 0 && !resolution.comment.trim()) {
		problems.push('comment');
	}

	return problems;
}

export function resolutionOutcome(resolution: Resolution): ResolutionOutcome {
	if ((resolution.rejected ?? 0) === 0) {
		return 'accept';
	}

	return (resolution.accepted ?? 0) === 0 ? 'reject' : 'split';
}

/** Тело запроса решения: причина и комментарий только при отклонённой части. */
export function resolutionBody(
	resolution: Resolution
): QuarantineResolutionBody {
	const rejected = resolution.rejected ?? 0;

	return {
		accepted_quantity: resolution.accepted ?? 0,
		rejected_quantity: rejected,
		reject_reason:
			rejected > 0 && resolution.rejectReason
				? resolution.rejectReason
				: undefined,
		comment: rejected > 0 ? resolution.comment.trim() : undefined,
	};
}

/**
 * Почему карантин уже закрыт: решение, принятое после открытия карантина, или исправление приёмки.
 * `stale` — позиция, по которой отправляли решение, `fresh` — она же из перечитанной поставки.
 */
export function closedQuarantineMessage(
	stale: DeliveryItem,
	fresh: DeliveryItem | undefined
): string {
	const name = itemName(stale);
	const last = fresh?.quarantine_resolutions.at(-1);
	const openedAt = stale.quarantined_at
		? new Date(stale.quarantined_at).getTime()
		: 0;

	if (!last || new Date(last.decided_at).getTime() < openedAt) {
		return `Карантин по «${name}» уже закрыт исправлением приёмки`;
	}

	return `Карантин по «${name}» уже решён: ${actorName(last.decided_by)}, ${formatTime(last.decided_at)}`;
}
