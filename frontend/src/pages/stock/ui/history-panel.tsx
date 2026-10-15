import { Box, Button, Stack, Typography } from '@mui/material';
import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { useState } from 'react';

import { batchesCount } from '@entities/delivery';
import { productQueryOptions, type Product } from '@entities/product';
import { actorName } from '@entities/actor';
import { formatTime } from '@shared/lib/datetime';
import { formatAmount, formatAmountDelta } from '@shared/lib/quantity';
import { QueryState } from '@shared/ui/empty-state';
import type { IconName } from '@shared/ui/icon';
import { TextLink } from '@shared/ui/link';
import { SidePanel } from '@shared/ui/side-panel';
import { SkeletonList } from '@shared/ui/skeleton-list';
import { TimelineItem } from '@shared/ui/timeline';

import {
	batchesQueryOptions,
	movementsQueryOptions,
	type Movement,
} from '../api/stock';
import { productStock } from '../model/actions';
import {
	movementBatches,
	movementDays,
	movementDelta,
	movementDetails,
	movementTitle,
} from '../model/history';

interface HistoryPanelProps {
	pointId: string;
	companyId: string;
	productId: string | undefined;
	now: Date;
	onClose: () => void;
}

const looks: Record<
	Movement['type'],
	{ icon: IconName; color: string; bg: string }
> = {
	received: { icon: 'toStock', color: 'success.ink', bg: 'success.tint' },
	consumed: { icon: 'consumed', color: 'text.secondary', bg: 'track' },
	written_off: { icon: 'writtenOff', color: 'error.ink', bg: 'error.tint' },
	adjusted: { icon: 'edit', color: 'text.secondary', bg: 'track' },
	expiry_warning: {
		icon: 'expiryWarning',
		color: 'warning.ink',
		bg: 'warning.tint',
	},
	expired: { icon: 'expired', color: 'error.ink', bg: 'error.tint' },
	returned: { icon: 'toStock', color: 'text.secondary', bg: 'track' },
};

function deltaColor(movement: Movement): string {
	if (movement.type === 'written_off') {
		return 'error.ink';
	}

	return movement.type === 'received' ? 'success.ink' : 'text.strong';
}

function MovementItem({
	pointId,
	movement,
	unit,
	now,
}: {
	pointId: string;
	movement: Movement;
	unit: Product['unit'];
	now: Date;
}) {
	const look = looks[movement.type];
	const delta = movementDelta(movement);
	const details = movementDetails(movement, now);

	return (
		<TimelineItem
			icon={look.icon}
			tone={look}
			sx={{ py: 1.25, borderTop: '1px solid', borderColor: 'divider' }}
		>
			<Stack sx={{ gap: 0.25 }}>
				<Stack
					direction='row'
					sx={{
						justifyContent: 'space-between',
						gap: 1.5,
						alignItems: 'baseline',
					}}
				>
					<Typography sx={{ fontWeight: 500, color: 'text.strong' }}>
						{movementTitle(movement)}
					</Typography>
					{delta === 0 ? (
						<Typography variant='body2' sx={{ color: 'text.secondary' }}>
							остаток не менялся
						</Typography>
					) : (
						<Typography
							sx={{
								fontWeight: 600,
								color: deltaColor(movement),
								whiteSpace: 'nowrap',
							}}
						>
							{formatAmountDelta(delta, unit)}
						</Typography>
					)}
				</Stack>
				<Stack
					direction='row'
					sx={{ justifyContent: 'space-between', gap: 1.5 }}
				>
					<Typography variant='caption' sx={{ color: 'text.secondary' }}>
						{formatTime(movement.occurred_at)} · {actorName(movement.actor)}
					</Typography>
					<Typography
						variant='caption'
						sx={{ color: 'text.secondary', whiteSpace: 'nowrap' }}
					>
						{movementBatches(movement)}
					</Typography>
				</Stack>
				{movement.type === 'received' && movement.delivery_id ? (
					<Typography variant='body2'>
						Из поставки{' '}
						<TextLink
							to='/points/$pointId/deliveries/$deliveryId'
							params={{ pointId, deliveryId: movement.delivery_id }}
							underline='hover'
						>
							№ {movement.changes[0]?.delivery_number}
						</TextLink>
						{details && ` · ${details}`}
					</Typography>
				) : (
					details && <Typography variant='body2'>{details}</Typography>
				)}
			</Stack>
		</TimelineItem>
	);
}

function MovementFeed({
	pointId,
	productId,
	unit,
	now,
}: Pick<HistoryPanelProps, 'pointId' | 'now'> & {
	productId: string;
	unit: Product['unit'] | undefined;
}) {
	const history = useInfiniteQuery(movementsQueryOptions(pointId, productId));
	const loading = (
		<SkeletonList label='Загружаем историю' count={4} height={48} />
	);

	const feed = ({ pages }: NonNullable<typeof history.data>) => {
		// Движения без единицы продукта не показать, поэтому ждём и продукт.
		if (unit === undefined) {
			return loading;
		}

		const movements = pages.flatMap((page) => page.items);
		const total = pages.at(-1)?.total ?? movements.length;

		if (movements.length === 0) {
			return (
				<Typography sx={{ color: 'text.secondary' }}>
					Движений по продукту пока нет
				</Typography>
			);
		}

		return (
			<Stack sx={{ gap: 1 }}>
				{movementDays(movements, now).map((day) => (
					<Box component='section' key={day.key} aria-label={day.label}>
						<Typography
							component='h3'
							variant='caption'
							sx={{
								display: 'block',
								fontWeight: 600,
								color: 'text.secondary',
								py: 1,
							}}
						>
							{day.label}
						</Typography>
						<Box component='ol' sx={{ m: 0, p: 0, listStyle: 'none' }}>
							{day.movements.map((movement) => (
								<MovementItem
									key={movement.id}
									pointId={pointId}
									movement={movement}
									unit={unit}
									now={now}
								/>
							))}
						</Box>
					</Box>
				))}
				<Stack
					direction='row'
					sx={{ justifyContent: 'space-between', alignItems: 'center', pt: 1 }}
				>
					<Typography variant='body2' sx={{ color: 'text.secondary' }}>
						{movements.length} из {total}
					</Typography>
					{history.hasNextPage && (
						<Button
							variant='outlined'
							loading={history.isFetchingNextPage}
							onClick={() => void history.fetchNextPage()}
						>
							Показать ещё
						</Button>
					)}
				</Stack>
			</Stack>
		);
	};

	return (
		<QueryState
			query={history}
			errorTitle='Не удалось загрузить историю'
			loading={loading}
		>
			{feed}
		</QueryState>
	);
}

/** История продукта в боковой панели: движения по партиям от новых к старым. */
export function HistoryPanel({
	pointId,
	companyId,
	productId,
	now,
	onClose,
}: HistoryPanelProps) {
	// Закрываясь, панель показывает последний продукт, а не пустоту.
	const [shownId, setShownId] = useState(productId);

	if (productId !== undefined && productId !== shownId) {
		setShownId(productId);
	}

	const { data: product } = useQuery({
		...productQueryOptions(companyId, shownId ?? ''),
		enabled: shownId !== undefined,
	});
	const batches = useQuery({
		...batchesQueryOptions(pointId, shownId ?? ''),
		enabled: shownId !== undefined,
	});
	const stock = batches.data && productStock(batches.data, now);

	return (
		<SidePanel
			open={productId !== undefined}
			title={product ? `История · ${product.name}` : 'История продукта'}
			closeLabel='Закрыть историю'
			onClose={onClose}
			width={520}
		>
			<Stack sx={{ gap: 2, p: { xs: 2, sm: 3 }, pt: { xs: 1, sm: 1 } }}>
				{product && stock && (
					<Typography variant='body2' sx={{ color: 'text.secondary' }}>
						{[
							product.category,
							`доступно ${formatAmount(stock.available, product.unit)}`,
							batchesCount(stock.batches),
						]
							.filter(Boolean)
							.join(' · ')}
					</Typography>
				)}
				{shownId && (
					<MovementFeed
						pointId={pointId}
						productId={shownId}
						unit={product?.unit}
						now={now}
					/>
				)}
			</Stack>
		</SidePanel>
	);
}
