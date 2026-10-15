import {
	fromDisplay,
	quantityInput,
	toDisplay,
	type QuantityUnit,
} from '@shared/lib/quantity';

import { QuantityField, type QuantityFieldProps } from './quantity-field';

export interface UnitQuantityFieldProps extends Omit<
	QuantityFieldProps,
	| 'unit'
	| 'step'
	| 'format'
	| 'value'
	| 'defaultValue'
	| 'onValueChange'
	| 'min'
	| 'max'
> {
	unit: QuantityUnit;
	/** Целое количество в единице продукта; `null` — пустое поле. */
	value: number | null;
	onValueChange: (value: number | null) => void;
	/** Максимум в единице продукта. */
	max?: number;
}

/** Поле количества в единице продукта: показывает и принимает кг, л и шт, отдаёт целые граммы, миллилитры и штуки. */
export function UnitQuantityField({
	unit,
	value,
	onValueChange,
	max,
	...props
}: UnitQuantityFieldProps) {
	const input = quantityInput(unit);

	return (
		<QuantityField
			{...props}
			unit={input.unit}
			step={input.step}
			format={input.format}
			max={max === undefined ? undefined : toDisplay(max, unit)}
			value={value === null ? null : toDisplay(value, unit)}
			onValueChange={(next) =>
				onValueChange(next === null ? null : fromDisplay(next, unit))
			}
		/>
	);
}
