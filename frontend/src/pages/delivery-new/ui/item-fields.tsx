import { IconButton, Stack, Typography } from '@mui/material';

import { useState } from 'react';

import { ProductPicker, productNote, type Product } from '@entities/product';
import { quantityInput } from '@shared/lib/quantity';
import { DateField, DateTimeField } from '@shared/ui/date-field';
import { Icon } from '@shared/ui/icon';
import { QuantityField } from '@shared/ui/quantity-field';

/** Общие свойства поля позиции; `value` и `error` у каждого поля свои. */
interface FieldProps<T> {
	value: T;
	error?: string;
	/** Номер позиции с единицы. */
	position: number;
	/** Таблица: подписи скрыты, их дают заголовки колонок. */
	compact: boolean;
	onChange: (value: T) => void;
}

/** Имя поля для программ чтения с экрана, когда видимой подписи нет. */
const spokenLabel = (position: number, text: string) =>
	`Позиция ${position}: ${text.toLowerCase()}`;

const fieldLabel = (compact: boolean, position: number, text: string) =>
	compact ? spokenLabel(position, text) : text;

export function ProductField({
	value,
	error,
	position,
	compact,
	onChange,
	companyId,
}: FieldProps<Product | undefined> & { companyId: string }) {
	return (
		<Stack sx={{ gap: 0.25, minWidth: 0 }}>
			<ProductPicker
				companyId={companyId}
				value={value ?? null}
				onChange={(next) => onChange(next ?? undefined)}
				label={compact ? undefined : 'Продукт'}
				ariaLabel={compact ? spokenLabel(position, 'Продукт') : undefined}
				error={error}
			/>
			{value && !error && (
				<Typography variant='caption' sx={{ color: 'text.secondary' }}>
					{productNote(value)}
				</Typography>
			)}
		</Stack>
	);
}

export function QuantityInput({
	value,
	error,
	position,
	compact,
	onChange,
	product,
}: FieldProps<number | null> & { product?: Product }) {
	const input = product ? quantityInput(product.unit) : undefined;

	return (
		<QuantityField
			label='Количество'
			unit={input?.unit}
			accessibleLabel={
				compact ? spokenLabel(position, 'Количество') : undefined
			}
			hideLabel={compact}
			fullWidth
			value={value}
			onValueChange={onChange}
			step={input?.step}
			format={input?.format}
			disabled={!product}
			error={Boolean(error)}
			helperText={error}
		/>
	);
}

/** «Годен до»: выбор дня в календаре даёт 23:59, истёкший срок предупреждает. */
export function ExpiresField({
	value,
	error,
	position,
	compact,
	onChange,
	expired,
}: FieldProps<Date | null> & { expired: boolean }) {
	const [endOfToday] = useState(() => {
		const date = new Date();

		date.setHours(23, 59, 0, 0);

		return date;
	});

	return (
		<Stack sx={{ gap: 0.25 }}>
			<DateTimeField
				label={fieldLabel(compact, position, 'Годен до')}
				hideLabel={compact}
				fullWidth
				value={value}
				onChange={onChange}
				referenceDate={endOfToday}
				error={Boolean(error)}
				helperText={error}
			/>
			{expired && !error && (
				<Stack
					direction='row'
					sx={{ gap: 0.5, alignItems: 'center', color: 'warning.ink' }}
				>
					<Icon name='warning' size='sm' />
					<Typography variant='caption' sx={{ color: 'inherit' }}>
						срок уже истёк: при приёмке позицию отклонят
					</Typography>
				</Stack>
			)}
		</Stack>
	);
}

export function ManufacturedField({
	value,
	error,
	position,
	compact,
	onChange,
}: FieldProps<Date | null>) {
	return (
		<DateField
			label={fieldLabel(compact, position, 'Изготовлено')}
			hideLabel={compact}
			fullWidth
			value={value}
			onChange={onChange}
			error={Boolean(error)}
			helperText={error}
		/>
	);
}

export function RemoveItemButton({
	position,
	disabled,
	onRemove,
}: {
	position: number;
	disabled: boolean;
	onRemove: () => void;
}) {
	return (
		<IconButton
			aria-label={`Удалить позицию ${position}`}
			disabled={disabled}
			onClick={onRemove}
		>
			<Icon name='close' />
		</IconButton>
	);
}
