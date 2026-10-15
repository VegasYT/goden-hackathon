import {
	Box,
	Paper,
	Stack,
	Table,
	TableBody,
	TableHead,
	TableRow,
	Typography,
} from '@mui/material';
import type { ReactNode } from 'react';

import { itemName, itemUnit, type DeliveryItem } from '@entities/delivery';
import { formatQuantity } from '@shared/lib/quantity';
import { Icon, type IconName } from '@shared/ui/icon';
import { PanelHeader } from '@shared/ui/panel-header';
import { useWideLayout } from '@shared/ui/theme';

/** Название продукта позиции и его категория. */
export function ItemTitle({ item }: { item: DeliveryItem }) {
	return (
		<>
			<Typography sx={{ fontWeight: 500, color: 'text.strong' }}>
				{itemName(item)}
			</Typography>
			{item.product_category && (
				<Typography variant='caption' sx={{ color: 'text.secondary' }}>
					{item.product_category}
				</Typography>
			)}
		</>
	);
}

/** Шапка карточки позиции: название и срок слева, заявленное количество справа. */
export function ItemCardHeader({
	item,
	expiry,
	status,
}: {
	item: DeliveryItem;
	expiry: ReactNode;
	status?: ReactNode;
}) {
	return (
		<Stack direction='row' sx={{ gap: 2, alignItems: 'flex-start' }}>
			<Box sx={{ flex: 1, minWidth: 0 }}>
				<ItemTitle item={item} />
				{expiry}
			</Box>
			<Box sx={{ textAlign: 'right' }}>
				<Typography sx={{ fontWeight: 600, color: 'text.strong' }}>
					{formatQuantity(item.declared_quantity, itemUnit(item))}
				</Typography>
				<Typography variant='caption' sx={{ color: 'text.secondary' }}>
					заявлено
				</Typography>
			</Box>
			{status}
		</Stack>
	);
}

/** Исход приёмки: иконка, подпись и значение; `framed` — отдельной плашкой с пояснением. */
export function OutcomeStat({
	icon,
	label,
	value,
	note,
	framed = false,
}: {
	icon: IconName;
	label: string;
	value: ReactNode;
	note?: ReactNode;
	framed?: boolean;
}) {
	const content = (
		<>
			<Icon name={icon} sx={{ color: 'text.secondary', mt: 0.25 }} />
			<Box sx={{ minWidth: 0 }}>
				<Typography
					variant='caption'
					component='div'
					sx={{ color: 'text.secondary' }}
				>
					{label}
				</Typography>
				<Typography
					variant={framed ? 'body1' : 'body2'}
					sx={{ fontWeight: 600, color: 'text.strong' }}
				>
					{value}
				</Typography>
				{note && (
					<Typography
						variant='caption'
						component='div'
						sx={{ color: 'text.secondary' }}
					>
						{note}
					</Typography>
				)}
			</Box>
		</>
	);

	return framed ? (
		<Paper sx={{ flex: 1, minWidth: 220, p: 2, display: 'flex', gap: 1.5 }}>
			{content}
		</Paper>
	) : (
		<Stack direction='row' sx={{ gap: 1, alignItems: 'flex-start' }}>
			{content}
		</Stack>
	);
}

interface ItemsPanelProps {
	/** Справа от заголовка: фильтр позиций или пометка. */
	aside?: ReactNode;
	/** Ячейки заголовка таблицы на широком экране. */
	columns: ReactNode;
	/** Строки таблицы на широком экране. */
	rows: ReactNode;
	/** Карточки позиций (`li`) на планшете. */
	cards: ReactNode;
	footer?: ReactNode;
}

/** Панель «Позиции» поставки: таблица на широком экране, карточки на планшете. */
export function ItemsPanel({
	aside,
	columns,
	rows,
	cards,
	footer,
}: ItemsPanelProps) {
	const wide = useWideLayout();

	return (
		<Paper sx={{ overflow: 'hidden' }}>
			<PanelHeader>
				<Typography variant='h3' component='h2'>
					Позиции
				</Typography>
				{aside}
			</PanelHeader>
			{wide ? (
				<Table aria-label='Позиции поставки' sx={{ tableLayout: 'fixed' }}>
					<TableHead>
						<TableRow>{columns}</TableRow>
					</TableHead>
					<TableBody>{rows}</TableBody>
				</Table>
			) : (
				<Stack
					component='ul'
					aria-label='Позиции поставки'
					sx={{ gap: 1.5, p: 2, m: 0, listStyle: 'none' }}
				>
					{cards}
				</Stack>
			)}
			{footer}
		</Paper>
	);
}
