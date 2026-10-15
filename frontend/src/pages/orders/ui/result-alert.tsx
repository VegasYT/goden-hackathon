import { Alert, AlertTitle, Box } from '@mui/material';

import type { DishRef } from '@entities/dish';
import { formatTime } from '@shared/lib/datetime';
import { Icon } from '@shared/ui/icon';
import { TextLink } from '@shared/ui/link';

import type { Order } from '../api/orders';
import { dishesText, shortageAmounts, shortageName } from '../model/order';

export type OrderResult =
	| { kind: 'accepted' | 'rejected'; order: Order }
	| { kind: 'dropped'; dishes: DishRef[] };

/** Итог оформления над меню: принят, отклонён из-за нехватки или блюдо выключили на точке. */
export function ResultAlert({
	result,
	pointId,
}: {
	result: OrderResult;
	pointId: string;
}) {
	if (result.kind === 'dropped') {
		const names = result.dishes.map((dish) => dish.name).join(', ');

		return (
			<Alert severity='error' icon={<Icon name='stopList' />}>
				<AlertTitle>
					{names} больше не{' '}
					{result.dishes.length > 1 ? 'продаются' : 'продаётся'} на точке
				</AlertTitle>
				Блюдо убрано из заказа. Проверьте порции и оформите снова.
			</Alert>
		);
	}

	const { order } = result;

	if (result.kind === 'accepted') {
		return (
			<Alert
				severity='success'
				action={
					<TextLink
						to='/points/$pointId/orders'
						params={{ pointId }}
						sx={{ color: 'inherit', alignSelf: 'center' }}
					>
						В историю
					</TextLink>
				}
			>
				<AlertTitle>
					Заказ принят
					{order.placed_at ? ` в ${formatTime(order.placed_at)}` : ''}
				</AlertTitle>
				{dishesText(order.items)}. Продукты списаны со склада.
			</Alert>
		);
	}

	return (
		<Alert severity='error'>
			<AlertTitle>Заказ отклонён: не хватило продуктов</AlertTitle>
			{order.shortages.map((shortage) => {
				const amounts = shortageAmounts(shortage);

				return (
					<Box key={shortage.product_id}>
						<Box component='span' sx={{ fontWeight: 500 }}>
							{shortageName(shortage)}
						</Box>{' '}
						— нужно {amounts.required}, было {amounts.available}
					</Box>
				);
			})}
			<Box sx={{ mt: 0.5 }}>
				Пока заполнялся заказ, остатки изменились. Склад не тронут, меню
				обновлено. Исправьте порции и оформите снова.
			</Box>
		</Alert>
	);
}
