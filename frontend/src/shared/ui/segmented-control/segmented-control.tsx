import { ToggleButton, ToggleButtonGroup } from '@mui/material';

export interface SegmentedOption<T extends string> {
	value: T;
	label: string;
	/** Количество рядом с подписью, например «Требуют решения 1». */
	count?: number;
}

export interface SegmentedControlProps<T extends string> {
	/** Подпись группы для программ чтения с экрана. */
	label: string;
	options: SegmentedOption<T>[];
	value: T;
	onChange: (value: T) => void;
	fullWidth?: boolean;
	disabled?: boolean;
}

/** Выбор ровно одного варианта из нескольких. */
export function SegmentedControl<T extends string>({
	label,
	options,
	value,
	onChange,
	fullWidth = false,
	disabled = false,
}: SegmentedControlProps<T>) {
	return (
		<ToggleButtonGroup
			aria-label={label}
			exclusive
			fullWidth={fullWidth}
			disabled={disabled}
			value={value}
			// Повторное нажатие на выбранный вариант даёт null: выбор не снимаем.
			onChange={(_, next: T | null) => {
				if (next !== null) onChange(next);
			}}
		>
			{options.map((option) => (
				<ToggleButton key={option.value} value={option.value}>
					{option.count === undefined
						? option.label
						: `${option.label} ${option.count}`}
				</ToggleButton>
			))}
		</ToggleButtonGroup>
	);
}
