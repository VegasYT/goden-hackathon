import {
	Box,
	Button,
	Chip,
	Stack,
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableRow,
	Typography,
} from '@mui/material';
import { useQuery } from '@tanstack/react-query';

import type { WriteAccess } from '@entities/profile';
import { BatchExpiryLabel } from '@entities/stock';
import { formatDateTime } from '@shared/lib/datetime';
import { isExpired } from '@shared/lib/expiry';
import { ActionMenu } from '@shared/ui/action-menu';
import { QueryState } from '@shared/ui/empty-state';
import { ExpiryLabel } from '@shared/ui/expiry-label';
import { Icon } from '@shared/ui/icon';
import { SkeletonList } from '@shared/ui/skeleton-list';
import { radius } from '@shared/ui/theme';

import { batchesQueryOptions, type Batch, type StockLevel } from '../api/stock';
import { firstInLine } from '../model/actions';
import {
	BatchRemainder,
	DeliveryLink,
	InventoryReadOnlyNote,
	WriteOffButton,
} from './parts';

export type BatchAction = 'writeOff' | 'adjust';

export interface BatchListProps {
	pointId: string;
	product: StockLevel;
	now: Date;
	/** Списывать и исправлять остаток можно только с `allowed`. */
	access: WriteAccess;
	onAction: (action: BatchAction, batch: Batch) => void;
	onHistory: () => void;
}

/** Действия партии; у просроченной — только списание, его и ждут. */
function BatchActions({
	batch,
	expired,
	onAction,
	touch,
}: {
	batch: Batch;
	expired: boolean;
	onAction: BatchListProps['onAction'];
	touch: boolean;
}) {
	const name = `партия № ${batch.delivery_number}`;
	const writeOff = (
		<WriteOffButton
			expired={expired}
			touch={touch}
			label={`Списать: ${name}`}
			onClick={() => onAction('writeOff', batch)}
		/>
	);

	if (expired) {
		return writeOff;
	}

	return (
		<Stack direction='row' sx={{ gap: 0.5, alignItems: 'center' }}>
			{writeOff}
			<ActionMenu
				label={`Ещё действия: ${name}`}
				items={[
					{
						icon: 'edit',
						label: 'Исправить остаток',
						onClick: () => onAction('adjust', batch),
					},
				]}
			/>
		</Stack>
	);
}

function HistoryButton({
	product,
	onHistory,
}: Pick<BatchListProps, 'product' | 'onHistory'>) {
	return (
		<Button
			startIcon={<Icon name='history' />}
			aria-haspopup='dialog'
			aria-label={`История продукта ${product.product.name}`}
			onClick={onHistory}
		>
			История продукта
		</Button>
	);
}

/** Партии продукта в порядке расхода: таблица на ноутбуке, список на планшете. */
export function BatchList({
	pointId,
	product,
	now,
	access,
	onAction,
	onHistory,
	wide,
}: BatchListProps & { wide: boolean }) {
	const batches = useQuery(batchesQueryOptions(pointId, product.product.id));
	const unit = product.product.unit;

	const content = (data: Batch[]) => {
		const first = firstInLine(data, now);
		const status = (batch: Batch) =>
			isExpired(batch.expires_at, now) ? (
				<BatchExpiryLabel expiresAt={batch.expires_at} now={now} />
			) : (
				<Stack
					direction='row'
					sx={{ gap: 0.75, alignItems: 'center', flexWrap: 'wrap' }}
				>
					<ExpiryLabel
						expiresAt={batch.expires_at}
						now={now}
						lead='remaining'
						withDate={!wide}
					/>
					{batch.id === first && (
						<Chip color='primary' label='расходуется первой' />
					)}
				</Stack>
			);
		const actions = (batch: Batch) =>
			access === 'allowed' ? (
				<BatchActions
					batch={batch}
					expired={isExpired(batch.expires_at, now)}
					onAction={onAction}
					touch={!wide}
				/>
			) : null;

		if (wide) {
			return (
				<Table
					size='small'
					aria-label={`Партии продукта ${product.product.name}, в порядке расхода`}
					sx={{ tableLayout: 'fixed' }}
				>
					<TableHead>
						<TableRow>
							<TableCell sx={{ width: 200 }}>Партия</TableCell>
							<TableCell sx={{ width: 130 }}>Принята</TableCell>
							<TableCell sx={{ width: 180 }}>Остаток</TableCell>
							<TableCell sx={{ width: 130 }}>Годна до</TableCell>
							<TableCell>Срок</TableCell>
							<TableCell sx={{ width: 200 }} align='right'>
								<HistoryButton product={product} onHistory={onHistory} />
							</TableCell>
						</TableRow>
					</TableHead>
					<TableBody>
						{data.map((batch) => (
							<TableRow key={batch.id}>
								<TableCell>
									из поставки <DeliveryLink pointId={pointId} batch={batch} />
								</TableCell>
								<TableCell>{formatDateTime(batch.received_at, now)}</TableCell>
								<TableCell>
									<BatchRemainder batch={batch} unit={unit} />
								</TableCell>
								<TableCell>{formatDateTime(batch.expires_at, now)}</TableCell>
								<TableCell>{status(batch)}</TableCell>
								<TableCell align='right'>{actions(batch)}</TableCell>
							</TableRow>
						))}
					</TableBody>
				</Table>
			);
		}

		return (
			<Stack
				component='ol'
				aria-label='Партии в порядке расхода'
				sx={{ m: 0, p: 0, listStyle: 'none', gap: 0.75 }}
			>
				{data.map((batch) => (
					<Box
						component='li'
						key={batch.id}
						sx={(theme) => ({
							display: 'flex',
							flexWrap: 'wrap',
							alignItems: 'center',
							gap: 1.5,
							p: 1.5,
							borderRadius: radius(theme, 'md'),
							bgcolor: 'background.default',
						})}
					>
						<Stack sx={{ gap: 0.5, flex: '1 1 150px', minWidth: 0 }}>
							<Typography variant='body2'>
								из поставки <DeliveryLink pointId={pointId} batch={batch} />
							</Typography>
							<Typography variant='caption' sx={{ color: 'text.secondary' }}>
								принята {formatDateTime(batch.received_at, now)}
							</Typography>
						</Stack>
						<Box sx={{ flex: '0 0 130px' }}>
							<BatchRemainder batch={batch} unit={unit} />
						</Box>
						<Box sx={{ flex: '1 1 140px' }}>{status(batch)}</Box>
						{actions(batch)}
					</Box>
				))}
			</Stack>
		);
	};

	return (
		<Stack sx={{ gap: 1.5 }}>
			{!wide && (
				<Stack
					direction='row'
					sx={{
						alignItems: 'center',
						justifyContent: 'space-between',
						gap: 1,
						flexWrap: 'wrap',
					}}
				>
					<Typography variant='body2' sx={{ color: 'text.secondary' }}>
						Партии в порядке расхода: сначала ближайший срок
					</Typography>
					<HistoryButton product={product} onHistory={onHistory} />
				</Stack>
			)}
			<QueryState
				query={batches}
				errorTitle='Не удалось загрузить партии'
				loading={
					<SkeletonList label='Загружаем партии' count={2} height={36} />
				}
			>
				{content}
			</QueryState>
			<InventoryReadOnlyNote access={access} />
		</Stack>
	);
}
