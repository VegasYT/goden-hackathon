import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { renderWithTheme } from '@shared/lib/test-utils';

import { ConnectionBanner } from './connection-banner';
import { ConnectionIndicator } from './connection-indicator';

const at1432 = new Date(2026, 9, 10, 14, 32);

describe('состояние связи', () => {
	it('индикатор показывает время обновления и объявляется как status', () => {
		renderWithTheme(
			<ConnectionIndicator state={{ status: 'online', updatedAt: at1432 }} />
		);

		expect(screen.getByRole('status')).toHaveTextContent(
			'в сети · обновлено 14:32'
		);
	});

	it('плашка показывает время данных и вызывает повтор', async () => {
		const onRetry = vi.fn();

		renderWithTheme(
			<ConnectionBanner
				state={{ status: 'offline', dataAt: at1432, onRetry }}
			/>
		);

		expect(screen.getByRole('alert')).toHaveTextContent(
			'Нет связи с сервером. Данные на 14:32 — переподключаемся…'
		);
		await userEvent.click(screen.getByRole('button', { name: 'Повторить' }));
		expect(onRetry).toHaveBeenCalledOnce();
	});
});
