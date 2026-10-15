import {
	Box,
	Card,
	Chip,
	Stack,
	TableCell,
	TableRow,
	Typography,
} from '@mui/material';
import type { ReactNode } from 'react';

import {
	batchCount,
	batchesCount,
	itemName,
	itemOutcomes,
	itemUnit,
	positionsCount,
	reasonLabel,
	type Delivery,
	type DeliveryItem,
} from '@entities/delivery';
import { formatDateTime, formatShortDate } from '@shared/lib/datetime';
import {
	formatQuantity,
	formatQuantityValue,
	type QuantityUnit,
} from '@shared/lib/quantity';
import { EmptyValue } from '@shared/ui/empty-value';
import { ExpiryLabel } from '@shared/ui/expiry-label';
import { InfoNote } from '@shared/ui/info-note';
import { TextLink } from '@shared/ui/link';

import { withoutCorrections } from '../model/snapshot';
import {
	ItemCardHeader,
	ItemsPanel,
	ItemTitle,
	OutcomeStat,
} from './item-blocks';

interface ItemView {
	item: DeliveryItem;
	/** Принятое и отклонённое без исправлений; есть, только если поставку исправляли. */
	before?: { accepted: number; rejected: number };
}

function Amount({
	value,
	before,
	unit,
}: {
	value: number;
	before?: number;
	unit: QuantityUnit;
}) {
	const changed = before !== undefined && before !== value;

	return (
		<Box>
			{changed && (
				<Typography
					variant='caption'
					component='del'
					aria-label={`было ${formatQuantityValue(before, unit)}`}
					sx={{ display: 'block', color: 'warning.ink' }}
				>
					{formatQuantityValue(before, unit)}
				</Typography>
			)}
			<Typography
				variant='body2'
				component='span'
				sx={{ color: 'text.strong' }}
			>
				{value > 0 ? formatQuantity(value, unit) : <EmptyValue />}
			</Typography>
		</Box>
	);
}

function Accepted({ item, before }: ItemView) {
	return (
		<Amount
			value={item.accepted_quantity}
			before={before?.accepted}
			unit={itemUnit(item)}
		/>
	);
}

function Rejected({ item, before }: ItemView) {
	return (
		<Amount
			value={item.rejected_quantity}
			before={before?.rejected}
			unit={itemUnit(item)}
		/>
	);
}

function Quarantine({
	pointId,
	item,
}: {
	pointId: string;
	item: DeliveryItem;
}) {
	const unit = itemUnit(item);
	const last = item.quarantine_resolutions.at(-1);

	if (item.quarantined_quantity > 0) {
		return (
			<Stack sx={{ gap: 0.5, alignItems: 'flex-start' }}>
				<Typography variant='body2' sx={{ color: 'text.strong' }}>
					{formatQuantity(item.quarantined_quantity, unit)}
				</Typography>
				<Chip color='warning' label='ждёт решения' />
				<TextLink
					to='/points/$pointId/deliveries'
					params={{ pointId }}
					search={{ panel: 'quarantine' }}
					variant='caption'
					underline='hover'
				>
					Открыть карантин
				</TextLink>
			</Stack>
		);
	}

	if (last) {
		return (
			<Typography
				variant='caption'
				component='div'
				sx={{ color: 'text.secondary' }}
			>
				решён {formatShortDate(last.decided_at)}:{' '}
				{formatQuantityValue(last.accepted_quantity, unit)} принято,{' '}
				{formatQuantityValue(last.rejected_quantity, unit)} отклонено
			</Typography>
		);
	}

	return <EmptyValue />;
}

function Reasons({ item }: { item: DeliveryItem }) {
	const reasons = [item.reject_reason, item.quarantine_reason].filter(
		(reason): reason is string => Boolean(reason)
	);

	if (reasons.length === 0) {
		return <EmptyValue />;
	}

	return (
		<Box>
			{reasons.map((reason) => (
				<Typography key={reason} variant='body2' sx={{ color: 'text.strong' }}>
					{reasonLabel(reason)}
				</Typography>
			))}
			{item.comment && (
				<Typography
					variant='caption'
					component='div'
					sx={{ color: 'text.secondary' }}
				>
					{item.comment}
				</Typography>
			)}
		</Box>
	);
}

function Batches({ item }: { item: DeliveryItem }) {
	if (item.batch_ids.length === 0) {
		return <EmptyValue />;
	}

	return <>{batchesCount(item.batch_ids.length)}</>;
}

function Expiry({ item, now }: { item: DeliveryItem; now: Date }) {
	return (
		<ExpiryLabel
			expiresAt={item.expires_at}
			now={now}
			expiredLabel={item.expired ? 'был просрочен при приёмке' : 'просрочен'}
		/>
	);
}

const snapshotColumns = (
	<>
		<TableCell>Продукт</TableCell>
		<TableCell sx={{ width: 96 }} align='right'>
			Заявлено
		</TableCell>
		<TableCell sx={{ width: 220 }}>Годен до</TableCell>
		<TableCell sx={{ width: 100 }}>Принято</TableCell>
		<TableCell sx={{ width: 100 }}>Отклонено</TableCell>
		<TableCell sx={{ width: 170 }}>Карантин</TableCell>
		<TableCell sx={{ width: 210 }}>Причина и комментарий</TableCell>
		<TableCell sx={{ width: 88 }}>Партии</TableCell>
	</>
);

interface SnapshotItemProps {
	pointId: string;
	view: ItemView;
	now: Date;
}

function SnapshotRow({ pointId, view, now }: SnapshotItemProps) {
	return (
		<TableRow sx={{ verticalAlign: 'top' }}>
			<TableCell>
				<ItemTitle item={view.item} />
			</TableCell>
			<TableCell align='right'>
				{formatQuantity(view.item.declared_quantity, itemUnit(view.item))}
			</TableCell>
			<TableCell>
				<Expiry item={view.item} now={now} />
			</TableCell>
			<TableCell>
				<Accepted {...view} />
			</TableCell>
			<TableCell>
				<Rejected {...view} />
			</TableCell>
			<TableCell>
				<Quarantine pointId={pointId} item={view.item} />
			</TableCell>
			<TableCell>
				<Reasons item={view.item} />
			</TableCell>
			<TableCell>
				<Batches item={view.item} />
			</TableCell>
		</TableRow>
	);
}

function Labeled({ label, children }: { label: string; children: ReactNode }) {
	return (
		<Box>
			<Typography
				variant='caption'
				component='div'
				sx={{ color: 'text.secondary' }}
			>
				{label}
			</Typography>
			{children}
		</Box>
	);
}

function SnapshotCard({ pointId, view, now }: SnapshotItemProps) {
	return (
		<Card component='li' sx={{ p: 2.5, display: 'grid', gap: 1.5 }}>
			<ItemCardHeader
				item={view.item}
				expiry={<Expiry item={view.item} now={now} />}
			/>
			<Box
				sx={{
					display: 'grid',
					gridTemplateColumns: 'repeat(4, minmax(0, 1fr))',
					gap: 1.5,
				}}
			>
				<Labeled label='Принято'>
					<Accepted {...view} />
				</Labeled>
				<Labeled label='Отклонено'>
					<Rejected {...view} />
				</Labeled>
				<Labeled label='Карантин'>
					<Quarantine pointId={pointId} item={view.item} />
				</Labeled>
				<Labeled label='Партии'>
					<Batches item={view.item} />
				</Labeled>
			</Box>
			{(view.item.reject_reason || view.item.quarantine_reason) && (
				<Reasons item={view.item} />
			)}
		</Card>
	);
}

/** Снимок принятой поставки: исход по каждой позиции, причины, карантин и исправления. */
export function DeliverySnapshot({
	delivery,
	now,
}: {
	delivery: Delivery;
	now: Date;
}) {
	const pointId = delivery.point_id;
	const outcomes = itemOutcomes(delivery.items);
	const resolved = delivery.items.filter(
		(item) => item.quarantine_resolutions.length > 0
	);
	const views: ItemView[] = delivery.items.map((item) => ({
		item,
		before: delivery.corrected_at ? withoutCorrections(item) : undefined,
	}));
	const rejectedNames = delivery.items
		.filter((item) => item.rejected_quantity > 0)
		.map(itemName)
		.join(', ');
	const quarantine =
		outcomes.quarantined > 0 ? (
			`ждёт решения · ${outcomes.quarantined}`
		) : resolved.length > 0 ? (
			'решён'
		) : (
			<EmptyValue />
		);

	return (
		<Stack sx={{ gap: 2 }}>
			<Stack direction='row' sx={{ gap: 1.5, flexWrap: 'wrap' }}>
				<OutcomeStat
					framed
					icon='toStock'
					label='На склад'
					value={positionsCount(outcomes.toStock)}
					note={batchesCount(batchCount(delivery))}
				/>
				<OutcomeStat
					framed
					icon='rejected'
					label='Отклонено'
					value={positionsCount(outcomes.rejected)}
					note={rejectedNames || undefined}
				/>
				<OutcomeStat
					framed
					icon='quarantine'
					label='Карантин'
					value={quarantine}
					note={
						outcomes.quarantined === 0 && resolved.length > 0
							? resolved.map(itemName).join(', ')
							: undefined
					}
				/>
			</Stack>
			<ItemsPanel
				aside={
					delivery.corrected_at && (
						<Typography variant='caption' sx={{ color: 'text.secondary' }}>
							исправлено {formatDateTime(delivery.corrected_at, now)} — прежнее
							значение зачёркнуто
						</Typography>
					)
				}
				columns={snapshotColumns}
				rows={views.map((view) => (
					<SnapshotRow
						key={view.item.id}
						pointId={pointId}
						view={view}
						now={now}
					/>
				))}
				cards={views.map((view) => (
					<SnapshotCard
						key={view.item.id}
						pointId={pointId}
						view={view}
						now={now}
					/>
				))}
				footer={
					<InfoNote
						sx={{
							px: 2,
							py: 1.5,
							borderTop: '1px solid',
							borderColor: 'divider',
						}}
					>
						Приёмка сохранена и не меняется. Ошибку исправляют отдельным
						действием с причиной — исходная приёмка остаётся в истории.
					</InfoNote>
				}
			/>
		</Stack>
	);
}
