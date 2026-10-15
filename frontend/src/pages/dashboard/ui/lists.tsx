import {
	Box,
	Stack,
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableRow,
	Typography,
} from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import type { ReactNode } from 'react';

import { batchesCount } from '@entities/delivery';
import {
	hotCaption,
	PinnedChip,
	suggestedPortionsText,
	menuQueryOptions,
	stopCaption,
	stoppedDishes,
} from '@entities/dish';
import { ProductHistoryLink } from '@entities/product';
import {
	BatchExpiryLabel,
	urgentExpiringHours,
	type ExpiringBatch,
} from '@entities/stock';
import { formatAmount } from '@shared/lib/quantity';
import { TextLink } from '@shared/ui/link';
import { useWideLayout } from '@shared/ui/theme';

import { expiringQueryOptions, hotQueryOptions } from '../api/queries';
import { blockQuery } from '../model/block-query';
import { CalmRow, Panel, Rows } from './panel';

interface ListProps {
	pointId: string;
	now: Date;
}

function ProductCell({
	item,
	pointId,
}: {
	item: ExpiringBatch;
	pointId: string;
}) {
	return (
		<>
			<Box sx={{ fontWeight: 500, color: 'text.strong' }}>
				<ProductHistoryLink product={item.product} pointId={pointId} linked />
			</Box>
			<Typography variant='body2' sx={{ color: 'text.secondary' }}>
				из поставки № {item.batch.delivery_number}
			</Typography>
		</>
	);
}

const amount = (item: ExpiringBatch) =>
	formatAmount(item.batch.quantity, item.product.unit);

function ExpiringRows({
	items,
	pointId,
	now,
}: ListProps & { items: ExpiringBatch[] }) {
	const wide = useWideLayout();

	if (wide) {
		return (
			<Table aria-label='Ближайшие сроки' sx={{ tableLayout: 'fixed' }}>
				<TableHead>
					<TableRow>
						<TableCell>Продукт</TableCell>
						<TableCell align='right' sx={{ width: 110 }}>
							Остаток
						</TableCell>
						<TableCell sx={{ width: 190 }}>Срок</TableCell>
					</TableRow>
				</TableHead>
				<TableBody>
					{items.map((item) => (
						<TableRow key={item.batch.id}>
							<TableCell>
								<ProductCell item={item} pointId={pointId} />
							</TableCell>
							<TableCell align='right'>{amount(item)}</TableCell>
							<TableCell>
								<BatchExpiryLabel expiresAt={item.batch.expires_at} now={now} />
							</TableCell>
						</TableRow>
					))}
				</TableBody>
			</Table>
		);
	}

	return (
		<Rows label='Ближайшие сроки'>
			{items.map((item) => (
				<Stack
					component='li'
					key={item.batch.id}
					direction='row'
					sx={{ alignItems: 'center', gap: 1.5, px: 2, py: 1.25 }}
				>
					<Box sx={{ flex: 1, minWidth: 0 }}>
						<ProductCell item={item} pointId={pointId} />
					</Box>
					<Stack sx={{ alignItems: 'flex-end', gap: 0.5 }}>
						<Typography>{amount(item)}</Typography>
						<BatchExpiryLabel
							expiresAt={item.batch.expires_at}
							now={now}
							inline
						/>
					</Stack>
				</Stack>
			))}
		</Rows>
	);
}

/** Ближайшие истекающие партии; полный список — во вкладке «Сроки». */
export function ExpiringList({ pointId, now }: ListProps) {
	const expiring = useQuery(expiringQueryOptions(pointId));
	const page = expiring.data;
	const more = page ? page.total - page.items.length : 0;
	const allLink = (text: string) => (
		<TextLink
			to='/points/$pointId/stock/expiring'
			params={{ pointId }}
			search={{ within: urgentExpiringHours }}
		>
			{text}
		</TextLink>
	);

	return (
		<Panel
			title='Ближайшие сроки'
			action={allLink('Все сроки')}
			query={blockQuery(expiring)}
			loading={!page}
			loadingLabel='Загружаем ближайшие сроки'
			errorTitle='Не удалось загрузить сроки'
		>
			{page &&
				(page.items.length === 0 ? (
					<CalmRow>За сутки ничего не истекает</CalmRow>
				) : (
					<>
						<ExpiringRows items={page.items} pointId={pointId} now={now} />
						{more > 0 && (
							<Typography variant='body2' sx={{ px: 2, py: 1.25 }}>
								{allLink(`и ещё ${batchesCount(more)} за сутки`)}
							</Typography>
						)}
					</>
				))}
		</Panel>
	);
}

function DishRow({
	name,
	tag,
	caption,
	aside,
}: {
	name: string;
	/** Отметка рядом с названием. */
	tag?: ReactNode;
	caption: string | undefined;
	aside?: string;
}) {
	return (
		<Stack
			component='li'
			direction='row'
			sx={{ alignItems: 'center', gap: 1.5, px: 2, py: 1.25 }}
		>
			<Box sx={{ flex: 1, minWidth: 0 }}>
				<Stack direction='row' sx={{ gap: 1, alignItems: 'center' }}>
					<Typography sx={{ fontWeight: 500, color: 'text.strong' }}>
						{name}
					</Typography>
					{tag}
				</Stack>
				{caption && (
					<Typography variant='body2' sx={{ color: 'text.secondary' }}>
						{caption}
					</Typography>
				)}
			</Box>
			{aside && (
				<Typography
					sx={{ flexShrink: 0, fontWeight: 600, color: 'text.strong' }}
				>
					{aside}
				</Typography>
			)}
		</Stack>
	);
}

/** Все блюда, которые сейчас нельзя приготовить, с причиной. */
export function StopList({ pointId }: { pointId: string }) {
	const menu = useQuery(menuQueryOptions(pointId));
	const stopped = menu.data && stoppedDishes(menu.data);

	return (
		<Panel
			title='Стоп-лист'
			action={
				<TextLink to='/points/$pointId/stop-list' params={{ pointId }}>
					Открыть
				</TextLink>
			}
			query={blockQuery(menu)}
			loading={!stopped}
			loadingLabel='Загружаем стоп-лист'
			errorTitle='Не удалось загрузить стоп-лист'
		>
			{menu.data?.length === 0 ? (
				<CalmRow>В меню точки пока нет блюд</CalmRow>
			) : stopped?.length === 0 ? (
				<CalmRow>Все блюда меню можно приготовить</CalmRow>
			) : (
				<Rows label='Стоп-лист'>
					{stopped?.map((dish) => (
						<DishRow
							key={dish.dish.id}
							name={dish.dish.name}
							caption={stopCaption(dish)}
						/>
					))}
				</Rows>
			)}
		</Panel>
	);
}

/** Первые горячие предложения: блюдо, продукт под риском и сколько порций продать. */
export function HotList({ pointId, now }: ListProps) {
	const hot = useQuery(hotQueryOptions(pointId));
	const items = hot.data?.items;

	return (
		<Panel
			title='Предложить гостям'
			action={
				<TextLink to='/points/$pointId/orders/hot' params={{ pointId }}>
					Причины
				</TextLink>
			}
			query={blockQuery(hot)}
			loading={!items}
			loadingLabel='Загружаем горячие предложения'
			errorTitle='Не удалось загрузить предложения'
		>
			{items &&
				(items.length === 0 ? (
					<CalmRow>Продвигать нечего</CalmRow>
				) : (
					<Rows label='Предложить гостям'>
						{items.map((dish) => (
							<DishRow
								key={dish.dish.id}
								name={dish.dish.name}
								tag={dish.manual && <PinnedChip />}
								caption={hotCaption(dish, now)}
								aside={suggestedPortionsText(dish)}
							/>
						))}
					</Rows>
				))}
		</Panel>
	);
}
