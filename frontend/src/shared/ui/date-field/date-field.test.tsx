import { screen } from '@testing-library/react';
import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';

import { renderWithTheme, typeDate } from '@shared/lib/test-utils';

import { DateField, DateTimeField } from './date-field';

function Harness({
	withTime,
	onChange,
}: {
	withTime?: boolean;
	onChange: (value: Date | null) => void;
}) {
	const [value, setValue] = useState<Date | null>(null);
	const Field = withTime ? DateTimeField : DateField;

	return (
		<Field
			label='Годен до'
			value={value}
			onChange={(next) => {
				setValue(next);
				onChange(next);
			}}
			error
			helperText='Укажите срок годности'
		/>
	);
}

describe('DateField', () => {
	it('ввод «17.10.2026 23:59» даёт дату со временем', async () => {
		const onChange = vi.fn();

		renderWithTheme(<Harness withTime onChange={onChange} />);
		await typeDate('Годен до', '17.10.2026 23:59');

		expect(onChange).toHaveBeenLastCalledWith(new Date(2026, 9, 17, 23, 59));
	});

	it('неполный ввод не даёт дату', async () => {
		const onChange = vi.fn();

		renderWithTheme(<Harness onChange={onChange} />);
		await typeDate('Годен до', '17.10');

		const values = onChange.mock.calls.map(([value]) => value as Date | null);

		expect(
			values.every((value) => value === null || Number.isNaN(value.getTime()))
		).toBe(true);
	});

	it('показывает ошибку у поля', () => {
		renderWithTheme(<Harness onChange={vi.fn()} />);

		expect(screen.getByText('Укажите срок годности')).toBeVisible();
	});
});
