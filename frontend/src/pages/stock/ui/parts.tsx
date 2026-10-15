import {
	Button,
	Chip,
	LinearProgress,
	Stack,
	Typography,
	type SxProps,
	type Theme,
} from '@mui/material';

import { AccessNote, type WriteAccess } from '@entities/profile';
import {
	formatAmount,
	formatAmountOf,
	type QuantityUnit,
} from '@shared/lib/quantity';
import { TextLink } from '@shared/ui/link';

import type { Batch } from '../api/stock';

export function OutOfStockChip() {
	return <Chip color='error' label='нет в наличии' />;
}

/** Доступное количество продукта; ноль показан чипом «нет в наличии». */
export function AvailableAmount({
	quantity,
	unit,
}: {
	quantity: number;
	unit: QuantityUnit;
}) {
	return quantity > 0 ? (
		<>{formatAmount(quantity, unit)}</>
	) : (
		<OutOfStockChip />
	);
}

/** Остаток партии из принятого с полоской. */
export function BatchRemainder({
	batch,
	unit,
}: {
	batch: Batch;
	unit: QuantityUnit;
}) {
	const share =
		batch.received_quantity > 0
			? Math.min(100, (batch.quantity / batch.received_quantity) * 100)
			: 0;

	return (
		<Stack sx={{ gap: 0.5, minWidth: 0 }}>
			<Typography variant='body2' sx={{ color: 'text.strong' }}>
				{formatAmountOf(batch.quantity, batch.received_quantity, unit)}
			</Typography>
			<LinearProgress
				variant='determinate'
				value={share}
				aria-hidden
				sx={{ maxWidth: 140 }}
			/>
		</Stack>
	);
}

/** Номер поставки партии со ссылкой на неё. */
export function DeliveryLink({
	pointId,
	batch,
}: {
	pointId: string;
	batch: Batch;
}) {
	return (
		<TextLink
			to='/points/$pointId/deliveries/$deliveryId'
			params={{ pointId, deliveryId: batch.delivery_id }}
			underline='hover'
			aria-label={`Поставка № ${batch.delivery_number}`}
			sx={{ fontWeight: 500 }}
		>
			№ {batch.delivery_number}
		</TextLink>
	);
}

/** Почему у партий нет «Списать» и исправления остатка. */
export function InventoryReadOnlyNote({ access }: { access: WriteAccess }) {
	return (
		<AccessNote
			access={access}
			disabled='Точка отключена — списывать и исправлять остаток нельзя'
			forbidden='Списывать и исправлять остаток могут управляющий и владелец'
		/>
	);
}

/** «Списать» у партии; у просроченной кнопка залита цветом ошибки: этого списания ждут. */
export function WriteOffButton({
	expired,
	touch,
	label,
	onClick,
	sx,
}: {
	expired: boolean;
	touch: boolean;
	/** Подпись для программ чтения с экрана с номером партии. */
	label: string;
	onClick: () => void;
	sx?: SxProps<Theme>;
}) {
	return (
		<Button
			variant={expired ? 'contained' : 'outlined'}
			color={expired ? 'error' : 'primary'}
			size={touch ? 'large' : 'small'}
			aria-label={label}
			onClick={onClick}
			sx={sx}
		>
			Списать
		</Button>
	);
}
