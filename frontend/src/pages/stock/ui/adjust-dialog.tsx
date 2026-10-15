import { Box, Stack, TextField, Typography } from '@mui/material';
import type { DeepKeys, DeepValue } from '@tanstack/react-form';
import { useState } from 'react';

import type { Product } from '@entities/product';
import { formatDateTime } from '@shared/lib/datetime';
import { formatAmount, formatAmountDelta } from '@shared/lib/quantity';
import { FormDialog } from '@shared/ui/form-dialog';
import { Icon } from '@shared/ui/icon';
import { InfoNote } from '@shared/ui/info-note';
import { TextLink } from '@shared/ui/link';
import { UnitQuantityField } from '@shared/ui/quantity-field';
import { SegmentedControl } from '@shared/ui/segmented-control';
import { radius } from '@shared/ui/theme';

import { adjustBatch, type Adjustment, type Batch } from '../api/stock';
import { useBatchWrite } from '../api/use-batch-write';
import {
	adjustDelta,
	adjustProblems,
	adjustReasons,
	hasProblems,
	liveQuantity,
	productStock,
	toAdjustment,
	type AdjustValues,
} from '../model/actions';
import { useAdjustForm } from '../model/adjust-form';
import { ProductLine, StaleBatchAlert } from './batch-dialog';

interface AdjustDialogProps {
	pointId: string;
	product: Product;
	batch: Batch;
	now: Date;
	onClose: () => void;
	onDone: (before: Batch, after: Batch) => void;
}

/** Диалог «Исправить остаток»: фактическое количество партии и причина. */
export function AdjustDialog({
	pointId,
	product,
	batch: opened,
	now,
	onClose,
	onDone,
}: AdjustDialogProps) {
	const unit = product.unit;
	const { batches, mutation, fieldError, otherError } = useBatchWrite({
		pointId,
		productId: product.id,
		quantityField: 'delta',
		write: (batchId, body: Adjustment, key) =>
			adjustBatch(pointId, batchId, body, key),
		onDone: (after) => onDone(current, after),
	});
	const current = { ...opened, quantity: liveQuantity(opened, batches) };
	const { form, values } = useAdjustForm();
	const [submitted, setSubmitted] = useState(false);
	const errors = adjustProblems(values);
	const shown = submitted ? errors : {};
	const delta = adjustDelta(values, current);
	const actualError = shown.actual ?? fieldError;
	const { available } = productStock(batches ?? [current], now);

	const change = <K extends DeepKeys<AdjustValues>>(
		field: K,
		value: DeepValue<AdjustValues, K>
	) => {
		form.setFieldValue(field, value);
		mutation.reset();
	};

	const submit = () => {
		setSubmitted(true);

		if (!hasProblems(errors)) {
			mutation.mutate({
				batchId: current.id,
				body: toAdjustment(values, current),
			});
		}
	};

	return (
		<FormDialog
			title='Исправить остаток'
			submitLabel='Исправить остаток'
			submitDisabled={values.actual === current.quantity}
			pending={mutation.isPending}
			error={otherError}
			maxWidth='sm'
			onSubmit={submit}
			onClose={onClose}
		>
			<ProductLine product={product} />
			<Box
				sx={(theme) => ({
					display: 'flex',
					gap: 1,
					alignItems: 'center',
					p: 1.25,
					borderRadius: radius(theme, 'md'),
					bgcolor: 'background.default',
				})}
			>
				<Icon name='empty' sx={{ color: 'text.secondary' }} />
				<Typography variant='body2'>
					Партия из поставки № {opened.delivery_number} · годна до{' '}
					{formatDateTime(opened.expires_at, now)} · сейчас{' '}
					<Box component='span' sx={{ fontWeight: 600 }}>
						{formatAmount(current.quantity, unit)}
					</Box>
				</Typography>
			</Box>
			<StaleBatchAlert
				opened={opened.quantity}
				current={current.quantity}
				unit={unit}
			/>
			<Box
				sx={{
					display: 'grid',
					gridTemplateColumns: { sm: 'repeat(2, minmax(0, 1fr))' },
					gap: 2,
					alignItems: 'start',
				}}
			>
				<UnitQuantityField
					label='Фактически на складе'
					unit={unit}
					value={values.actual}
					onValueChange={(actual) => change('actual', actual)}
					error={Boolean(actualError)}
					helperText={actualError ?? 'Сколько партии лежит на самом деле'}
					fullWidth
				/>
				<Stack sx={{ gap: 0.5 }}>
					<Typography variant='body2' sx={{ fontWeight: 500 }}>
						Изменение
					</Typography>
					<Typography
						component='output'
						aria-label='Изменение'
						sx={{ fontSize: 17, fontWeight: 600, color: 'text.strong' }}
					>
						{values.actual === null || delta === 0
							? '—'
							: formatAmountDelta(delta, unit)}
					</Typography>
					{values.actual !== null && (
						<Typography variant='caption' sx={{ color: 'text.secondary' }}>
							было {formatAmount(current.quantity, unit)}, станет{' '}
							{formatAmount(values.actual, unit)}
						</Typography>
					)}
				</Stack>
			</Box>
			<Stack sx={{ gap: 1 }}>
				<SegmentedControl
					label='Частые причины'
					options={adjustReasons.map((reason) => ({
						value: reason,
						label: reason,
					}))}
					value={values.reason}
					onChange={(reason) => change('reason', reason)}
				/>
				<TextField
					label='Причина'
					required
					value={values.reason}
					onChange={(event) => change('reason', event.target.value)}
					error={Boolean(shown.reason)}
					helperText={shown.reason}
					slotProps={{ htmlInput: { maxLength: 1000 } }}
				/>
				<InfoNote>
					Ошиблись при приёмке? Исправьте приёмку в{' '}
					<TextLink
						to='/points/$pointId/deliveries/$deliveryId'
						params={{ pointId, deliveryId: opened.delivery_id }}
					>
						поставке № {opened.delivery_number}
					</TextLink>
					: там сохранится распределение позиции и её история.
				</InfoNote>
			</Stack>
			<TextField
				label='Комментарий'
				multiline
				minRows={2}
				placeholder='Например, что нашли при пересчёте'
				value={values.comment}
				onChange={(event) => change('comment', event.target.value)}
				slotProps={{ htmlInput: { maxLength: 1000 } }}
			/>
			{values.actual !== null && delta !== 0 && (
				<Typography variant='body2' sx={{ color: 'text.secondary' }}>
					В партии станет {formatAmount(values.actual, unit)}, доступно по
					продукту — {formatAmount(Math.max(0, available + delta), unit)}.
					Исправление попадёт в историю продукта с вашим именем и причиной.
				</Typography>
			)}
		</FormDialog>
	);
}
