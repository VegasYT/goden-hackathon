import type { components } from '@shared/api';
import { formatDateTime } from '@shared/lib/datetime';

export type WriteOffReason = components['schemas']['WriteOffReason'];

const labels: Record<WriteOffReason, string> = {
	expired: 'Истёк срок годности',
	transport_damage: 'Повреждено при перевозке',
	storage_violation: 'Нарушен температурный режим',
	physical_damage: 'Повреждена упаковка',
	quality_issue: 'Не соответствует качеству',
	other: 'Другое',
};

/** Причины в порядке показа в списке выбора. */
export const writeOffReasons = Object.keys(labels) as WriteOffReason[];

/** Подпись причины; свободный текст из ранних поставок возвращается как есть. */
export function reasonLabel(reason: string): string {
	return Object.hasOwn(labels, reason)
		? labels[reason as WriteOffReason]
		: reason;
}

/** Нужен ли комментарий к списанию: у истёкшего срока причина понятна и без него. */
export function reasonNeedsComment(reason: WriteOffReason | null): boolean {
	return reason !== 'expired';
}

/** Комментарий, который проставляется к отклонению просроченной позиции. */
export function expiredComment(expiresAt: string): string {
	return `Срок годности истёк ${formatDateTime(expiresAt)}`;
}
