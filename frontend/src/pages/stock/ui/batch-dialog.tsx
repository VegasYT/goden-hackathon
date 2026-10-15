import { Alert, Typography } from '@mui/material';

import type { Product } from '@entities/product';
import { formatAmount, type QuantityUnit } from '@shared/lib/quantity';
import { Icon } from '@shared/ui/icon';

/** Подзаголовок диалога: продукт и категория. */
export function ProductLine({ product }: { product: Product }) {
	return (
		<Typography sx={{ color: 'text.secondary' }}>
			{[product.name, product.category].filter(Boolean).join(' · ')}
		</Typography>
	);
}

/** Остаток партии уменьшился, пока диалог был открыт. */
export function StaleBatchAlert({
	opened,
	current,
	unit,
	hint,
}: {
	/** Остаток на момент открытия диалога. */
	opened: number;
	current: number;
	unit: QuantityUnit;
	hint?: string;
}) {
	if (current >= opened) {
		return null;
	}

	return (
		<Alert severity='warning' icon={<Icon name='refresh' />}>
			Сейчас в партии {formatAmount(current, unit)}, было{' '}
			{formatAmount(opened, unit)}. Пока окно было открыто, заказы израсходовали{' '}
			{formatAmount(opened - current, unit)}.{hint && ` ${hint}`}
		</Alert>
	);
}
