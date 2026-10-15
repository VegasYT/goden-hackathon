import { Alert, Box, Stack, TextField, Typography } from '@mui/material';
import { useState } from 'react';

import { portionsText } from '@entities/dish';
import { actorName, isMigom, type Actor } from '@entities/actor';
import { ActionMenu } from '@shared/ui/action-menu';
import { ConfirmDialog } from '@shared/ui/confirm-dialog';
import { Icon } from '@shared/ui/icon';
import { radius } from '@shared/ui/theme';

import type { Order } from '../api/orders';
import { placedTime } from '../model/order';

/** Кто оформил или отменил заказ: у Мигома — отметка, у остальных — подпись. */
export function OrderActor({ actor }: { actor: Actor }) {
	return isMigom(actor) ? (
		<Stack
			component='span'
			direction='row'
			sx={{
				alignItems: 'center',
				gap: 0.75,
				color: 'text.strong',
				fontWeight: 500,
			}}
		>
			<Icon name='migom' size='sm' sx={{ color: 'primary.ink' }} />
			Мигом
		</Stack>
	) : (
		<Typography component='span' noWrap sx={{ color: 'text.strong' }}>
			{actorName(actor)}
		</Typography>
	);
}

/** Меню действий принятого заказа; пока в нём одна отмена. */
export function OrderActions({
	order,
	onCancel,
}: {
	order: Order;
	onCancel: (order: Order) => void;
}) {
	return (
		<ActionMenu
			label={`Действия: заказ ${placedTime(order)}`}
			items={[
				{
					icon: 'cancelOrder',
					label: 'Отменить заказ',
					hint: 'продукты вернутся на склад',
					danger: true,
					onClick: () => onCancel(order),
				},
			]}
		/>
	);
}

/** Состав заказа: блюда с числом порций. */
export function OrderItems({ items }: { items: Order['items'] }) {
	return items.map((item) => (
		<Stack
			key={item.dish_id}
			direction='row'
			sx={{ justifyContent: 'space-between', gap: 2, py: 0.5 }}
		>
			<Typography sx={{ color: 'text.strong' }}>{item.dish_name}</Typography>
			<Typography>{portionsText(item.portions)}</Typography>
		</Stack>
	));
}

/** Подтверждение отмены: состав, кто оформил и необязательный комментарий. */
export function CancelOrderDialog({
	order,
	pending,
	error,
	onSubmit,
	onClose,
}: {
	order: Order;
	pending: boolean;
	error?: string;
	onSubmit: (comment: string) => void;
	onClose: () => void;
}) {
	const [comment, setComment] = useState('');

	return (
		<ConfirmDialog
			open
			title={`Отменить заказ от ${placedTime(order)}?`}
			description='Продукты вернутся на склад в те же партии. Отменяйте, пока заказ не начали готовить.'
			confirmLabel='Отменить заказ'
			cancelLabel='Не отменять'
			confirmColor='error'
			pending={pending}
			error={error}
			onConfirm={() => onSubmit(comment)}
			onClose={onClose}
		>
			<Stack sx={{ gap: 2 }}>
				<Box
					sx={(theme) => ({
						px: 1.5,
						py: 1.25,
						borderRadius: radius(theme, 'md'),
						bgcolor: 'field',
					})}
				>
					<Typography
						variant='body2'
						sx={{ color: 'text.secondary', mb: 0.25 }}
					>
						Оформил {actorName(order.placed_by)}
					</Typography>
					<OrderItems items={order.items} />
				</Box>
				{isMigom(order.placed_by) && (
					<Alert severity='info' icon={<Icon name='migom' />}>
						Заказ из Мигома: покупатель получит отмену с этим комментарием.
					</Alert>
				)}
				<TextField
					label='Комментарий'
					placeholder='Например, гость ушёл'
					multiline
					minRows={2}
					value={comment}
					onChange={(event) => setComment(event.target.value)}
					helperText='Необязательно, до 1000 знаков. Его увидят в истории заказов.'
					slotProps={{ htmlInput: { maxLength: 1000 } }}
				/>
			</Stack>
		</ConfirmDialog>
	);
}
