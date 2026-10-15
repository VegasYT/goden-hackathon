import {
	Box,
	Card,
	Chip,
	Stack,
	TableCell,
	TableRow,
	Typography,
} from '@mui/material';

import {
	allocationText,
	itemName,
	itemUnit,
	type DeliveryItem,
} from '@entities/delivery';
import { formatDateTime } from '@shared/lib/datetime';
import { formatQuantity, formatQuantityValue } from '@shared/lib/quantity';
import { Icon } from '@shared/ui/icon';
import { UnitQuantityField } from '@shared/ui/quantity-field';

import {
	isItemExpired,
	plannedChange,
	reductionLimit,
	type CorrectionValue,
	type ServerHint,
} from '../model/correction';
import {
	correctionState,
	useCorrectionValue,
	type CorrectionForm,
	type CorrectionState,
} from '../model/correction-form';
import { allocated, parts, quantityOf, type Part } from '../model/decision';
import { ItemCardHeader, ItemTitle } from './item-blocks';
import { formatPartValue, partLabels } from './parts';

const dash = '—';

interface CorrectionItemProps {
	form: CorrectionForm;
	index: number;
	item: DeliveryItem;
	now: Date;
	hint?: ServerHint;
	/** Ошибка позиции от сервера, которую не покрывают правила формы. */
	serverError?: string;
}

/** Текущее распределение позиции: «принято 4,0 · откл. — · кар. —». */
function Current({ item }: { item: DeliveryItem }) {
	const show = (quantity: number) =>
		quantity > 0 ? formatQuantityValue(quantity, itemUnit(item)) : dash;

	return (
		<Stack sx={{ gap: 0.5, alignItems: 'flex-start' }}>
			<Typography variant='body2' sx={{ color: 'text.secondary' }}>
				принято {show(item.accepted_quantity)} · откл.{' '}
				{show(item.rejected_quantity)} · кар. {show(item.quarantined_quantity)}
			</Typography>
			{item.quarantined_quantity > 0 && (
				<Chip
					color='warning'
					icon={<Icon name='quarantine' size='sm' />}
					label='ждёт решения'
				/>
			)}
		</Stack>
	);
}

function Term({ item, now }: { item: DeliveryItem; now: Date }) {
	return (
		<Typography
			variant='caption'
			component='div'
			sx={{ color: 'text.secondary' }}
		>
			{isItemExpired(item, now) ? 'срок истёк' : 'срок до'}{' '}
			{formatDateTime(item.expires_at, now)}
		</Typography>
	);
}

/** Остаток партий позиции на складе: на столько можно уменьшить принятое. */
function Stock({ item, hint }: { item: DeliveryItem; hint?: ServerHint }) {
	if (item.batch_ids.length === 0) {
		return (
			<Typography variant='body2' sx={{ color: 'text.secondary' }}>
				нет партии
			</Typography>
		);
	}

	return (
		<Typography variant='body2'>
			{formatQuantityValue(reductionLimit(item, hint), itemUnit(item))} из{' '}
			{formatQuantity(item.accepted_quantity, itemUnit(item))}
		</Typography>
	);
}

function PartField({
	form,
	index,
	item,
	now,
	hint,
	part,
	hideLabel,
	current,
}: CorrectionItemProps & {
	part: Part;
	hideLabel: boolean;
	current: CorrectionValue;
}) {
	const name = itemName(item);
	const before = item[quantityOf[part]];
	const expired = part === 'accepted' && isItemExpired(item, now, hint);
	const locked = expired && before === 0;
	const changed = (current[part] ?? 0) !== before;

	return (
		<form.Field name={`items[${index}].${part}`}>
			{(field) => (
				<UnitQuantityField
					label={partLabels[part]}
					accessibleLabel={`${name}: ${partLabels[part].toLowerCase()}`}
					hideLabel={hideLabel}
					fullWidth
					unit={itemUnit(item)}
					max={expired ? before : item.declared_quantity}
					value={field.state.value}
					onValueChange={(next) => field.handleChange(next)}
					disabled={locked}
					helperText={
						locked
							? 'срок истёк'
							: changed
								? `было ${formatPartValue(before, itemUnit(item))}`
								: undefined
					}
				/>
			)}
		</form.Field>
	);
}

/** Сообщения позиции: распределение, предел уменьшения, срок и ошибка сервера. */
function Messages({
	item,
	current,
	state,
	hint,
	serverError,
}: {
	item: DeliveryItem;
	current: CorrectionValue;
	state: CorrectionState;
	hint?: ServerHint;
	serverError?: string;
}) {
	const unit = itemUnit(item);
	const format = (quantity: number) => formatQuantity(quantity, unit);
	const allocation = allocationText(
		item.declared_quantity,
		allocated(current),
		unit,
		'declared'
	);
	const messages = allocation ? [allocation] : [];

	if (state.problems.includes('stock-consumed')) {
		messages.push(
			`Принятое можно уменьшить не больше чем на ${format(reductionLimit(item, hint))}: остальное уже израсходовано`
		);
	}

	if (state.problems.includes('accepted-expired')) {
		messages.push(
			`Срок истёк — принять больше ${format(item.accepted_quantity)} нельзя`
		);
	}

	if (serverError) {
		messages.push(serverError);
	}

	const stock =
		messages.length === 0 && state.changed
			? plannedChange(item, current).stock
			: 0;

	if (messages.length === 0 && stock === 0) {
		return null;
	}

	return (
		<Stack sx={{ gap: 0.5 }}>
			{messages.map((message) => (
				<Typography
					key={message}
					variant='body2'
					sx={{
						color: 'error.ink',
						display: 'flex',
						gap: 0.75,
						alignItems: 'center',
					}}
				>
					<Icon name='error' size='sm' />
					{message}
				</Typography>
			))}
			{stock !== 0 && (
				<Typography variant='body2' sx={{ color: 'text.secondary' }}>
					{stock > 0
						? `+${format(stock)} новой партией`
						: `−${format(-stock)} со склада`}
				</Typography>
			)}
		</Stack>
	);
}

function Status({ state }: { state: CorrectionState }) {
	if (state.problems.length > 0) {
		return <Icon name='error' label='Ошибка' sx={{ color: 'error.ink' }} />;
	}

	return state.changed ? (
		<Icon name='corrected' label='Изменено' sx={{ color: 'primary.ink' }} />
	) : null;
}

// Строка и карточка читают только своё значение, поэтому ввод в одной позиции не перерисовывает остальные.
export function CorrectionRow(props: CorrectionItemProps) {
	const { form, index, item, now, hint, serverError } = props;
	const current = useCorrectionValue(form, index);
	const state = correctionState(item, current, now, hint);

	return (
		<TableRow sx={{ verticalAlign: 'top' }}>
			<TableCell>
				<ItemTitle item={item} />
				<Term item={item} now={now} />
			</TableCell>
			<TableCell align='right'>
				{formatQuantity(item.declared_quantity, itemUnit(item))}
			</TableCell>
			<TableCell>
				<Current item={item} />
			</TableCell>
			{parts.map((part) => (
				<TableCell key={part}>
					<PartField {...props} part={part} hideLabel current={current} />
				</TableCell>
			))}
			<TableCell>
				<Stack sx={{ gap: 0.75 }}>
					<Stock item={item} hint={hint} />
					<Messages
						item={item}
						current={current}
						state={state}
						hint={hint}
						serverError={serverError}
					/>
				</Stack>
			</TableCell>
			<TableCell>
				<Status state={state} />
			</TableCell>
		</TableRow>
	);
}

export function CorrectionCard(props: CorrectionItemProps) {
	const { form, index, item, now, hint, serverError } = props;
	const current = useCorrectionValue(form, index);
	const state = correctionState(item, current, now, hint);

	return (
		<Card
			component='li'
			aria-label={itemName(item)}
			sx={{ p: 2.5, display: 'grid', gap: 1.5 }}
		>
			<ItemCardHeader
				item={item}
				expiry={<Term item={item} now={now} />}
				status={<Status state={state} />}
			/>
			<Current item={item} />
			<Box
				sx={{
					display: 'grid',
					gridTemplateColumns: 'repeat(3, minmax(0, 1fr))',
					gap: 1.5,
				}}
			>
				{parts.map((part) => (
					<PartField
						key={part}
						{...props}
						part={part}
						hideLabel={false}
						current={current}
					/>
				))}
			</Box>
			<Stack direction='row' sx={{ gap: 1, alignItems: 'baseline' }}>
				<Typography variant='caption' sx={{ color: 'text.secondary' }}>
					Остаток партии
				</Typography>
				<Stock item={item} hint={hint} />
			</Stack>
			<Messages
				item={item}
				current={current}
				state={state}
				hint={hint}
				serverError={serverError}
			/>
		</Card>
	);
}
