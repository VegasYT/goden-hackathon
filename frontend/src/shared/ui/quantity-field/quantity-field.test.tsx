import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { renderWithTheme } from '@shared/lib/test-utils';

import { QuantityField } from './quantity-field';

describe('QuantityField', () => {
	it('принимает число с запятой и показывает его с запятой', async () => {
		const onValueChange = vi.fn();

		renderWithTheme(
			<QuantityField label='Принять' unit='кг' onValueChange={onValueChange} />
		);
		const input = screen.getByLabelText('Принять, кг');

		await userEvent.type(input, '2,5');
		await userEvent.tab();

		expect(onValueChange).toHaveBeenLastCalledWith(2.5);
		expect(input).toHaveValue('2,5');
	});

	it('уменьшение на минимуме не меняет значение', async () => {
		const onValueChange = vi.fn();

		renderWithTheme(
			<QuantityField
				label='Принять'
				defaultValue={0}
				onValueChange={onValueChange}
			/>
		);

		// Кнопка на минимуме неактивна, поэтому нажимаем её в обход проверки pointer-events.
		const user = userEvent.setup({ pointerEventsCheck: 0 });

		await user.click(screen.getByRole('button', { name: 'Принять: меньше' }));

		expect(screen.getByLabelText('Принять')).toHaveValue('0');
		expect(onValueChange).not.toHaveBeenCalled();
	});

	it('увеличение прибавляет шаг', async () => {
		const onValueChange = vi.fn();

		renderWithTheme(
			<QuantityField
				label='Принять'
				defaultValue={1.5}
				step={0.5}
				onValueChange={onValueChange}
			/>
		);

		await userEvent.click(
			screen.getByRole('button', { name: 'Принять: больше' })
		);

		expect(onValueChange).toHaveBeenLastCalledWith(2);
		expect(screen.getByLabelText('Принять')).toHaveValue('2');
	});
});
