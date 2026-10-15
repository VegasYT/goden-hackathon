import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { ApiError } from '@shared/api';
import { renderWithTheme } from '@shared/lib/test-utils';

import { ProfileUnavailablePage } from './profile-unavailable-page';

describe('ProfileUnavailablePage', () => {
	it('показывает текст ошибки сервера, а «Повторить» запрашивает профиль снова', async () => {
		const onRetry = vi.fn();

		renderWithTheme(
			<ProfileUnavailablePage
				error={new ApiError('Сервис временно недоступен')}
				onRetry={onRetry}
			/>
		);

		expect(screen.getByText('Сервис временно недоступен')).toBeInTheDocument();
		await userEvent.click(screen.getByRole('button', { name: 'Повторить' }));
		expect(onRetry).toHaveBeenCalled();
	});
});
