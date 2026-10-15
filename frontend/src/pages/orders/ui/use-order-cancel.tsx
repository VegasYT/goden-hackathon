import { useState } from 'react';

import { useNotice } from '@shared/ui/notice';

import { useCancelOrder } from '../api/order-writes';
import type { Order } from '../api/orders';
import { placedTime } from '../model/order';
import { CancelOrderDialog } from './order-cancel';

/** Отмена заказа из истории: `open` открывает подтверждение, `elements` — диалог и сообщение, которые страница ставит у себя. */
export function useOrderCancel(pointId: string) {
	const mutation = useCancelOrder(pointId);
	const [target, setTarget] = useState<Order | null>(null);
	const notice = useNotice();

	const elements = (
		<>
			{target && (
				<CancelOrderDialog
					order={target}
					pending={mutation.isPending}
					error={mutation.error?.message}
					onSubmit={(comment) =>
						mutation.mutate(
							{ orderId: target.id, comment },
							{
								onSuccess: () => {
									setTarget(null);
									notice.show(
										`Заказ от ${placedTime(target)} отменён, продукты вернулись на склад`
									);
								},
							}
						)
					}
					onClose={() => {
						mutation.reset();
						setTarget(null);
					}}
				/>
			)}
			{notice.element}
		</>
	);

	return { open: setTarget, elements };
}
