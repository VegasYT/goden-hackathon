import {
	Box,
	Button,
	FormControl,
	FormControlLabel,
	FormHelperText,
	FormLabel,
	Radio,
	RadioGroup,
	Stack,
	TextField,
	Typography,
} from '@mui/material';
import type { DeepKeys, DeepValue } from '@tanstack/react-form';
import { useId, useState } from 'react';

import {
	reasonLabel,
	reasonNeedsComment,
	writeOffReasons,
	type WriteOffReason,
} from '@entities/delivery';
import type { Product } from '@entities/product';
import { isExpired } from '@shared/lib/expiry';
import { formatAmount } from '@shared/lib/quantity';
import { ExpiryLabel } from '@shared/ui/expiry-label';
import { FormDialog } from '@shared/ui/form-dialog';
import { Icon } from '@shared/ui/icon';
import { UnitQuantityField } from '@shared/ui/quantity-field';
import { radius } from '@shared/ui/theme';

import { writeOffBatch, type Batch, type WriteOff } from '../api/stock';
import { useBatchWrite } from '../api/use-batch-write';
import {
	hasProblems,
	liveQuantity,
	productStock,
	toWriteOff,
	stockedBatches,
	writeOffProblems,
	type WriteOffValues,
} from '../model/actions';
import { useWriteOffForm } from '../model/write-off-form';
import { ProductLine, StaleBatchAlert } from './batch-dialog';

interface WriteOffDialogProps {
	pointId: string;
	product: Product;
	batch: Batch;
	now: Date;
	onClose: () => void;
	onDone: (before: Batch, after: Batch, quantity: number) => void;
}

/** Диалог «Списать порчу»: партия, количество, причина и обязательный комментарий. */
export function WriteOffDialog({
	pointId,
	product,
	batch: initial,
	now,
	onClose,
	onDone,
}: WriteOffDialogProps) {
	const id = useId();
	const unit = product.unit;
	const { batches, mutation, fieldError, otherError } = useBatchWrite({
		pointId,
		productId: product.id,
		quantityField: 'quantity',
		write: (batchId, body: WriteOff, key) =>
			writeOffBatch(pointId, batchId, body, key),
		onDone: (after, body) => onDone(current, after, body.quantity),
	});
	// Партии на момент загрузки: выбор не прыгает, пока заказы расходуют остаток.
	const [loaded, setLoaded] = useState(
		() => batches && stockedBatches(batches)
	);

	if (loaded === undefined && batches) {
		setLoaded(stockedBatches(batches));
	}

	const choices = loaded ?? [initial];
	const { form, values } = useWriteOffForm(initial, now);
	const [submitted, setSubmitted] = useState(false);

	const opened = choices.find(({ id }) => id === values.batchId) ?? initial;
	const current = { ...opened, quantity: liveQuantity(opened, batches) };
	const errors = writeOffProblems(values, current, unit, now);
	const shown = submitted ? errors : {};
	const quantityError = shown.quantity ?? fieldError;
	const amount = Math.max(0, values.quantity ?? 0);
	const { available } = productStock(batches ?? [current], now);

	const change = <K extends DeepKeys<WriteOffValues>>(
		field: K,
		value: DeepValue<WriteOffValues, K>
	) => {
		form.setFieldValue(field, value);
		mutation.reset();
	};

	const submit = () => {
		setSubmitted(true);

		if (!hasProblems(errors)) {
			mutation.mutate({ batchId: current.id, body: toWriteOff(values) });
		}
	};

	return (
		<FormDialog
			title='Списать порчу'
			submitLabel={
				amount > 0 ? `Списать ${formatAmount(amount, unit)}` : 'Списать'
			}
			submitColor='error'
			pending={mutation.isPending}
			error={otherError}
			maxWidth='sm'
			onSubmit={submit}
			onClose={onClose}
		>
			<ProductLine product={product} />
			<StaleBatchAlert
				opened={opened.quantity}
				current={current.quantity}
				unit={unit}
				hint={`Спишите не больше ${formatAmount(current.quantity, unit)} или выберите другую партию.`}
			/>
			<FormControl>
				<FormLabel id={`${id}-batch`}>Партия · в порядке расхода</FormLabel>
				<RadioGroup
					aria-labelledby={`${id}-batch`}
					value={values.batchId}
					onChange={(event) => change('batchId', event.target.value)}
				>
					{choices.map((choice) => (
						<FormControlLabel
							key={choice.id}
							value={choice.id}
							control={<Radio />}
							label={
								<Stack
									direction='row'
									sx={{ gap: 1.25, alignItems: 'center', flexWrap: 'wrap' }}
								>
									<Typography sx={{ fontWeight: 500 }}>
										№ {choice.delivery_number}
									</Typography>
									<Typography>
										{formatAmount(liveQuantity(choice, batches), unit)}
									</Typography>
									<ExpiryLabel
										expiresAt={choice.expires_at}
										now={now}
										lead='remaining'
										inline
									/>
								</Stack>
							}
						/>
					))}
				</RadioGroup>
			</FormControl>
			<Stack direction='row' sx={{ gap: 1, alignItems: 'flex-start' }}>
				<UnitQuantityField
					label='Количество'
					unit={unit}
					value={values.quantity}
					onValueChange={(quantity) => change('quantity', quantity)}
					error={Boolean(quantityError)}
					helperText={
						quantityError ?? `из ${formatAmount(current.quantity, unit)}`
					}
				/>
				<Button
					variant='outlined'
					sx={{ mt: 3 }}
					aria-label={`Всё: ${formatAmount(current.quantity, unit)}`}
					onClick={() => change('quantity', current.quantity)}
				>
					Всё
				</Button>
			</Stack>
			<FormControl error={Boolean(shown.reason)}>
				<FormLabel id={`${id}-reason`}>Причина</FormLabel>
				<RadioGroup
					aria-labelledby={`${id}-reason`}
					value={values.reason ?? ''}
					onChange={(event) =>
						change('reason', event.target.value as WriteOffReason)
					}
					sx={{
						display: 'grid',
						gridTemplateColumns: { sm: 'repeat(2, minmax(0, 1fr))' },
					}}
				>
					{writeOffReasons.map((reason) => (
						<FormControlLabel
							key={reason}
							value={reason}
							control={<Radio />}
							label={reasonLabel(reason)}
							disabled={
								reason === 'expired' && !isExpired(current.expires_at, now)
							}
						/>
					))}
				</RadioGroup>
				<FormHelperText>
					{shown.reason ??
						'«Истёк срок годности» — только для партии, у которой срок уже истёк.'}
				</FormHelperText>
			</FormControl>
			<TextField
				label='Комментарий'
				required={reasonNeedsComment(values.reason)}
				multiline
				minRows={2}
				placeholder='Что случилось с продуктом'
				value={values.comment}
				onChange={(event) => change('comment', event.target.value)}
				error={Boolean(shown.comment)}
				helperText={shown.comment}
				slotProps={{ htmlInput: { maxLength: 1000 } }}
			/>
			<Box
				sx={(theme) => ({
					p: 1.75,
					borderRadius: radius(theme, 'md'),
					bgcolor: 'background.default',
				})}
			>
				<Stack
					direction='row'
					sx={{ justifyContent: 'space-between', gap: 2, flexWrap: 'wrap' }}
				>
					<Typography sx={{ fontWeight: 600, color: 'text.strong' }}>
						В партии останется{' '}
						{formatAmount(Math.max(0, current.quantity - amount), unit)}
					</Typography>
					<Typography variant='body2' sx={{ color: 'text.secondary' }}>
						доступно по продукту станет{' '}
						{formatAmount(Math.max(0, available - amount), unit)}
					</Typography>
				</Stack>
				<Stack
					direction='row'
					sx={{ gap: 1, mt: 1.25, alignItems: 'flex-start' }}
				>
					<Icon name='info' sx={{ color: 'error.ink', mt: 0.25 }} />
					<Typography variant='body2'>
						<Box component='span' sx={{ fontWeight: 500 }}>
							Отменить списание будет нельзя.
						</Box>{' '}
						Оно попадёт в историю продукта с вашим именем.
					</Typography>
				</Stack>
			</Box>
		</FormDialog>
	);
}
