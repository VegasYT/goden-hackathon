import {
	Box,
	ButtonBase,
	Collapse,
	IconButton,
	Stack,
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableRow,
	Typography,
} from '@mui/material';
import { Fragment, useId } from 'react';

import { batchesCount } from '@entities/delivery';
import type { StockSort } from '@entities/stock';
import { formatAmount } from '@shared/lib/quantity';
import { CardList, CardListItem } from '@shared/ui/card-list';
import { EmptyValue } from '@shared/ui/empty-value';
import { ExpiryLabel } from '@shared/ui/expiry-label';
import { DisclosureIcon } from '@shared/ui/icon';
import { SortableHeaderCell } from '@shared/ui/sortable-header-cell';

import type { Batch, StockLevel } from '../api/stock';
import { BatchList, type BatchAction, type BatchListProps } from './batch-list';
import { AvailableAmount, OutOfStockChip } from './parts';

interface StockRowsProps extends Omit<
	BatchListProps,
	'product' | 'onHistory' | 'onAction'
> {
	items: StockLevel[];
	expanded: ReadonlySet<string>;
	onToggle: (productId: string) => void;
	onHistory: (productId: string) => void;
	onAction: (product: StockLevel, action: BatchAction, batch: Batch) => void;
}

function Expired({ item }: { item: StockLevel }) {
	if (item.expired_quantity === 0) {
		return <EmptyValue />;
	}

	return (
		<Stack sx={{ gap: 0.25 }}>
			<Typography variant='body2' sx={{ color: 'error.ink', fontWeight: 500 }}>
				{formatAmount(item.expired_quantity, item.product.unit)}
			</Typography>
			<Typography variant='caption' sx={{ color: 'text.secondary' }}>
				нужно списать
			</Typography>
		</Stack>
	);
}

/** Заголовок колонки, по которой сервер умеет сортировать остатки. */
function SortableCell({
	label,
	field,
	width,
	sort,
	onSortChange,
}: {
	label: string;
	field: 'name' | 'available' | 'nearest_expiry';
	width?: number;
	sort: StockSort;
	onSortChange: (sort: StockSort) => void;
}) {
	const active = sort.replace('-', '') === field;
	const descending = sort.startsWith('-');

	return (
		<SortableHeaderCell
			sx={width ? { width } : undefined}
			direction={active ? (descending ? 'desc' : 'asc') : undefined}
			onSort={() => onSortChange(active && !descending ? `-${field}` : field)}
		>
			{label}
		</SortableHeaderCell>
	);
}

/** Остатки на ноутбуке: строка продукта раскрывается в таблицу его партий. */
export function StockTable({
	items,
	expanded,
	onToggle,
	onHistory,
	onAction,
	sort,
	onSortChange,
	...batchProps
}: StockRowsProps & {
	sort: StockSort;
	onSortChange: (sort: StockSort) => void;
}) {
	return (
		<Table aria-label='Остатки по продуктам' sx={{ tableLayout: 'fixed' }}>
			<TableHead>
				<TableRow>
					<TableCell sx={{ width: 56 }} />
					<SortableCell
						label='Продукт'
						field='name'
						sort={sort}
						onSortChange={onSortChange}
					/>
					<TableCell sx={{ width: 190 }}>Категория</TableCell>
					<SortableCell
						label='Доступно'
						field='available'
						width={140}
						sort={sort}
						onSortChange={onSortChange}
					/>
					<TableCell sx={{ width: 90 }}>Партий</TableCell>
					<SortableCell
						label='Ближайший срок'
						field='nearest_expiry'
						width={190}
						sort={sort}
						onSortChange={onSortChange}
					/>
					<TableCell sx={{ width: 190 }}>Просрочено</TableCell>
				</TableRow>
			</TableHead>
			<TableBody>
				{items.map((item) => {
					const open = expanded.has(item.product.id);
					const expandable = item.batches > 0;

					return (
						<Fragment key={item.product.id}>
							<TableRow
								hover={expandable}
								onClick={
									expandable ? () => onToggle(item.product.id) : undefined
								}
								sx={{
									cursor: expandable ? 'pointer' : undefined,
									'& > td': open ? { borderBottom: 0 } : undefined,
								}}
							>
								<TableCell>
									{expandable && (
										<IconButton
											aria-expanded={open}
											aria-label={`Партии продукта ${item.product.name}`}
											onClick={(event) => {
												event.stopPropagation();
												onToggle(item.product.id);
											}}
										>
											<DisclosureIcon open={open} />
										</IconButton>
									)}
								</TableCell>
								<TableCell sx={{ fontWeight: 500, color: 'text.strong' }}>
									{item.product.name}
								</TableCell>
								<TableCell>{item.product.category || <EmptyValue />}</TableCell>
								<TableCell>
									<AvailableAmount
										quantity={item.available_quantity}
										unit={item.product.unit}
									/>
								</TableCell>
								<TableCell
									sx={{
										color: item.batches === 0 ? 'text.secondary' : undefined,
									}}
								>
									{item.batches}
								</TableCell>
								<TableCell>
									{item.nearest_expires_at ? (
										<ExpiryLabel
											expiresAt={item.nearest_expires_at}
											now={batchProps.now}
											lead='remaining'
										/>
									) : (
										<EmptyValue />
									)}
								</TableCell>
								<TableCell>
									<Expired item={item} />
								</TableCell>
							</TableRow>
							{open && (
								<TableRow>
									<TableCell colSpan={7} sx={{ pt: 0, pb: 2, pl: 7 }}>
										<BatchList
											{...batchProps}
											wide
											product={item}
											onHistory={() => onHistory(item.product.id)}
											onAction={(action, batch) =>
												onAction(item, action, batch)
											}
										/>
									</TableCell>
								</TableRow>
							)}
						</Fragment>
					);
				})}
			</TableBody>
		</Table>
	);
}

function ProductCard({
	item,
	open,
	onToggle,
	onHistory,
	onAction,
	...batchProps
}: Omit<BatchListProps, 'product'> & {
	item: StockLevel;
	open: boolean;
	onToggle: () => void;
}) {
	const regionId = useId();
	const expandable = item.batches > 0;
	const summary = (
		<Stack
			direction='row'
			sx={{ gap: 1.5, alignItems: 'center', width: '100%' }}
		>
			<Stack sx={{ flex: 1, minWidth: 0 }}>
				<Typography component='h3' variant='h3' noWrap>
					{item.product.name}
				</Typography>
				<Typography variant='body2' sx={{ color: 'text.secondary' }}>
					{[
						item.product.category,
						expandable ? batchesCount(item.batches) : 'нет партий',
					]
						.filter(Boolean)
						.join(' · ')}
				</Typography>
			</Stack>
			<Stack sx={{ alignItems: 'flex-end', gap: 0.5, flexShrink: 0 }}>
				{item.available_quantity > 0 ? (
					<Typography sx={{ whiteSpace: 'nowrap' }}>
						<Typography
							component='span'
							variant='body2'
							sx={{ color: 'text.secondary' }}
						>
							доступно{' '}
						</Typography>
						<Typography
							component='span'
							sx={{ fontWeight: 600, fontSize: 18, color: 'text.strong' }}
						>
							{formatAmount(item.available_quantity, item.product.unit)}
						</Typography>
					</Typography>
				) : (
					<OutOfStockChip />
				)}
				{item.nearest_expires_at && (
					<ExpiryLabel
						expiresAt={item.nearest_expires_at}
						now={batchProps.now}
						lead='remaining'
						inline
					/>
				)}
			</Stack>
			<Box sx={{ width: 24, flexShrink: 0, display: 'flex' }}>
				{expandable && <DisclosureIcon open={open} sx={{ fontSize: 24 }} />}
			</Box>
		</Stack>
	);

	return (
		<CardListItem label={item.product.name} sx={{ display: 'block', p: 0 }}>
			{expandable ? (
				<ButtonBase
					aria-expanded={open}
					aria-controls={regionId}
					onClick={onToggle}
					sx={{ width: '100%', textAlign: 'left', p: 1.5, pl: 2 }}
				>
					{summary}
				</ButtonBase>
			) : (
				<Box sx={{ p: 1.5, pl: 2 }}>{summary}</Box>
			)}
			{item.expired_quantity > 0 && (
				<Typography
					variant='body2'
					sx={{ px: 2, pb: 1.5, mt: -0.5, color: 'error.ink' }}
				>
					просрочено {formatAmount(item.expired_quantity, item.product.unit)} ·
					нужно списать
				</Typography>
			)}
			<Collapse in={open} unmountOnExit>
				<Box
					id={regionId}
					role='region'
					aria-label={`Партии «${item.product.name}»`}
					sx={{ px: 1.5, pb: 1.5 }}
				>
					<BatchList
						{...batchProps}
						wide={false}
						product={item}
						onHistory={onHistory}
						onAction={onAction}
					/>
				</Box>
			</Collapse>
		</CardListItem>
	);
}

/** Остатки на планшете: карточка продукта раскрывается в список партий. */
export function StockCards({
	items,
	expanded,
	onToggle,
	onHistory,
	onAction,
	...batchProps
}: StockRowsProps) {
	return (
		<CardList label='Продукты'>
			{items.map((item) => (
				<ProductCard
					key={item.product.id}
					{...batchProps}
					item={item}
					open={expanded.has(item.product.id)}
					onToggle={() => onToggle(item.product.id)}
					onHistory={() => onHistory(item.product.id)}
					onAction={(action, batch) => onAction(item, action, batch)}
				/>
			))}
		</CardList>
	);
}
