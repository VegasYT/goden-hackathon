import { Button } from '@mui/material';

import type { Product } from '@entities/product';
import { formatAmount } from '@shared/lib/quantity';
import { useNotice } from '@shared/ui/notice';

import type { Batch } from '../api/stock';
import { AdjustDialog } from './adjust-dialog';
import type { BatchAction } from './batch-list';
import { useHistoryPanel } from './use-history-panel';
import { WriteOffDialog } from './write-off-dialog';

export interface OpenBatchAction {
	kind: BatchAction;
	product: Product;
	batch: Batch;
}

interface BatchActionDialogsProps {
	pointId: string;
	now: Date;
	/** Открытое действие; `null` — диалога нет. */
	action: OpenBatchAction | null;
	onClose: () => void;
}

/** Диалоги списания и исправления остатка и уведомление о результате с переходом в историю. */
export function BatchActionDialogs({
	pointId,
	now,
	action,
	onClose,
}: BatchActionDialogsProps) {
	const notice = useNotice();
	const openHistory = useHistoryPanel();

	const finish = (product: Product, text: string) => {
		onClose();
		notice.show(
			text,
			<Button onClick={() => openHistory(product.id)}>История</Button>
		);
	};

	return (
		<>
			{action?.kind === 'writeOff' && (
				<WriteOffDialog
					pointId={pointId}
					product={action.product}
					batch={action.batch}
					now={now}
					onClose={onClose}
					onDone={(before, after, quantity) => {
						const { name, unit } = action.product;

						finish(
							action.product,
							`Списано ${formatAmount(quantity, unit)} · ${name}, партия № ${before.delivery_number}. Осталось ${formatAmount(after.quantity, unit)}`
						);
					}}
				/>
			)}
			{action?.kind === 'adjust' && (
				<AdjustDialog
					pointId={pointId}
					product={action.product}
					batch={action.batch}
					now={now}
					onClose={onClose}
					onDone={(before, after) =>
						finish(
							action.product,
							`Остаток исправлен · ${action.product.name}, партия № ${before.delivery_number}. Сейчас ${formatAmount(after.quantity, action.product.unit)}`
						)
					}
				/>
			)}
			{notice.element}
		</>
	);
}
