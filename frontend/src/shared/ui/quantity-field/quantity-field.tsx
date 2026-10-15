import { NumberField as BaseNumberField } from '@base-ui/react/number-field';
import {
	Box,
	FormControl,
	FormHelperText,
	IconButton,
	InputBase,
	InputLabel,
} from '@mui/material';
import { useId, type ReactNode } from 'react';

import { Icon } from '@shared/ui/icon';
import { coarsePointer, fieldFocusRing, radius } from '@shared/ui/theme';

export interface QuantityFieldProps {
	label: string;
	/** Единица измерения, добавляется к подписи: «Принять, кг». */
	unit?: string;
	/** Имя поля для программ чтения с экрана, когда видимой подписи мало: «Пармезан: принять». */
	accessibleLabel?: string;
	/** Видимую подпись даёт окружение, например заголовок колонки таблицы. */
	hideLabel?: boolean;
	fullWidth?: boolean;
	value?: number | null;
	defaultValue?: number;
	onValueChange?: (value: number | null) => void;
	/** Минимум, по умолчанию 0. */
	min?: number;
	max?: number;
	step?: number;
	/** Форматирование, по умолчанию до трёх знаков после запятой. */
	format?: Intl.NumberFormatOptions;
	disabled?: boolean;
	readOnly?: boolean;
	error?: boolean;
	helperText?: ReactNode;
}

const defaultFormat: Intl.NumberFormatOptions = { maximumFractionDigits: 3 };

const stepperSx = {
	borderRadius: 0,
	width: 32,
	height: '100%',
	[coarsePointer]: { width: 44 },
};

/** Поле количества с кнопками −/+ и десятичной запятой. */
export function QuantityField({
	label,
	unit,
	accessibleLabel,
	hideLabel = false,
	fullWidth = false,
	value,
	defaultValue,
	onValueChange,
	min = 0,
	max,
	step = 1,
	format = defaultFormat,
	disabled,
	readOnly,
	error,
	helperText,
}: QuantityFieldProps) {
	const id = useId();
	const helperId = helperText ? `${id}-helper` : undefined;
	const withUnit = (text: string) => (unit ? `${text}, ${unit}` : text);
	const spokenName = accessibleLabel ?? label;
	// Подпись над полем озвучивается сама; своё имя полю нужно, только если подпись скрыта или неполна.
	const inputName =
		hideLabel || accessibleLabel ? withUnit(spokenName) : undefined;

	return (
		<BaseNumberField.Root
			value={value}
			defaultValue={defaultValue}
			onValueChange={(next) => onValueChange?.(next)}
			min={min}
			max={max}
			step={step}
			locale='ru-RU'
			format={format}
			disabled={disabled}
			readOnly={readOnly}
			render={(props, state) => (
				<FormControl
					ref={props.ref}
					disabled={state.disabled}
					error={error}
					fullWidth={fullWidth}
					sx={{ minWidth: 0 }}
				>
					{props.children}
				</FormControl>
			)}
		>
			{!hideLabel && (
				<InputLabel htmlFor={id} shrink>
					{withUnit(label)}
				</InputLabel>
			)}
			<BaseNumberField.Group
				render={
					<Box
						sx={(theme) => ({
							display: 'flex',
							alignItems: 'stretch',
							height: 32,
							borderRadius: radius(theme, 'sm'),
							border: '1px solid',
							borderColor: error ? 'error.main' : 'lineInput',
							bgcolor: 'field',
							overflow: 'hidden',
							'&:focus-within': {
								borderColor: 'primary.ink',
								boxShadow: fieldFocusRing(theme),
							},
							[coarsePointer]: {
								height: 44,
								borderRadius: radius(theme, 'md'),
							},
						})}
					/>
				}
			>
				<BaseNumberField.Decrement
					render={
						<IconButton aria-label={`${spokenName}: меньше`} sx={stepperSx} />
					}
				>
					<Icon name='remove' />
				</BaseNumberField.Decrement>
				<BaseNumberField.Input
					id={id}
					aria-label={inputName}
					aria-describedby={helperId}
					render={(props, state) => (
						<InputBase
							inputRef={props.ref}
							value={state.inputValue}
							slotProps={{ input: props }}
							sx={(theme) => ({
								flex: 1,
								minWidth: 0,
								color: 'text.strong',
								'& input': { textAlign: 'center', padding: 0 },
								[coarsePointer]: { fontSize: theme.typography.h3.fontSize },
							})}
						/>
					)}
				/>
				<BaseNumberField.Increment
					render={
						<IconButton aria-label={`${spokenName}: больше`} sx={stepperSx} />
					}
				>
					<Icon name='add' />
				</BaseNumberField.Increment>
			</BaseNumberField.Group>
			{helperText && (
				<FormHelperText id={helperId}>{helperText}</FormHelperText>
			)}
		</BaseNumberField.Root>
	);
}
