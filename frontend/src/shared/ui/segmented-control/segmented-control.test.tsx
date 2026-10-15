import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it } from 'vitest';

import { renderWithTheme } from '@shared/lib/test-utils';

import { SegmentedControl } from './segmented-control';

type Filter = 'all' | 'pending' | 'expired';

function Harness() {
	const [value, setValue] = useState<Filter>('all');

	return (
		<SegmentedControl<Filter>
			label='Фильтр позиций'
			value={value}
			onChange={setValue}
			options={[
				{ value: 'all', label: 'Все', count: 8 },
				{ value: 'pending', label: 'Требуют решения', count: 1 },
				{ value: 'expired', label: 'Просрочены', count: 1 },
			]}
		/>
	);
}

describe('SegmentedControl', () => {
	it('выбор отмечает новый вариант и снимает отметку с прежнего', async () => {
		renderWithTheme(<Harness />);
		const all = screen.getByRole('button', { name: 'Все 8' });
		const pending = screen.getByRole('button', { name: 'Требуют решения 1' });

		await userEvent.click(pending);

		expect(pending).toHaveAttribute('aria-pressed', 'true');
		expect(all).toHaveAttribute('aria-pressed', 'false');
	});

	it('повторное нажатие не снимает выбор', async () => {
		renderWithTheme(<Harness />);
		const all = screen.getByRole('button', { name: 'Все 8' });

		await userEvent.click(all);

		expect(all).toHaveAttribute('aria-pressed', 'true');
	});

	it('группа подписана для программ чтения', () => {
		renderWithTheme(<Harness />);
		expect(
			screen.getByRole('group', { name: 'Фильтр позиций' })
		).toBeInTheDocument();
	});
});
