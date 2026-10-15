import { act, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';

import { mockSystemColorScheme, renderWithTheme } from '@shared/lib/test-utils';

import { ModeToggle } from './mode-toggle';
import { colorModeStorageKey } from './theme';

describe('ModeToggle', () => {
	it('по умолчанию следует системной теме', async () => {
		mockSystemColorScheme('dark');
		renderWithTheme(<ModeToggle />);

		expect(
			await screen.findByRole('button', { name: 'Включить светлую тему' })
		).toBeInTheDocument();
	});

	it('выбор сохраняется и важнее последующей смены системной темы', async () => {
		const system = mockSystemColorScheme('dark');

		renderWithTheme(<ModeToggle />);

		await userEvent.click(
			await screen.findByRole('button', { name: 'Включить светлую тему' })
		);
		expect(window.localStorage.getItem(colorModeStorageKey)).toBe('light');

		// Система переключается на светлую тему и обратно, а выбор пользователя остаётся.
		act(() => system.set('light'));
		act(() => system.set('dark'));
		expect(
			screen.getByRole('button', { name: 'Включить тёмную тему' })
		).toBeInTheDocument();
	});
});
