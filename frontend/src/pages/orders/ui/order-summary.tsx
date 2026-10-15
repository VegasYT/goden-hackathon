import { Typography } from '@mui/material';

import { portionsText } from '@entities/dish';
import { plural } from '@shared/lib/plural';

import { dishesText } from '../model/order';
import type { OrderLine } from '../model/order-form';

/** Итог заказа в `ActionBar`: сколько блюд и порций, состав и предупреждение о превышении. */
export function OrderSummary({
	lines,
	over,
}: {
	lines: OrderLine[];
	/** Порций хотя бы одного блюда больше, чем можно приготовить. */
	over: boolean;
}) {
	if (lines.length === 0) {
		return (
			<>
				<Typography sx={{ fontWeight: 600, color: 'text.strong' }}>
					Заказ пуст
				</Typography>
				<Typography variant='body2' sx={{ color: 'text.secondary' }}>
					Добавьте порции блюд в меню.
				</Typography>
			</>
		);
	}

	const totalPortions = lines.reduce((sum, line) => sum + line.portions, 0);

	return (
		<>
			<Typography sx={{ fontWeight: 600, color: 'text.strong' }}>
				{lines.length} {plural(lines.length, 'блюдо', 'блюда', 'блюд')} ·{' '}
				{portionsText(totalPortions)}
			</Typography>
			<Typography variant='body2' noWrap sx={{ color: 'text.secondary' }}>
				{dishesText(
					lines.map((line) => ({
						dish_name: line.dish.name,
						portions: line.portions,
					}))
				)}
			</Typography>
			{over && (
				<Typography role='alert' variant='body2' sx={{ color: 'error.ink' }}>
					Порций больше, чем можно приготовить. Исправьте заказ.
				</Typography>
			)}
		</>
	);
}
