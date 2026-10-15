import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { renderWithTheme } from '@shared/lib/test-utils';

import { ExpiryLabel } from './expiry-label';

const now = new Date('2026-10-12T14:32:00Z');
const in17Hours = new Date(now.getTime() + 17.5 * 3_600_000).toISOString();

describe('ExpiryLabel', () => {
	it('для оставшегося срока первым ставит «ещё 17 ч» в чипе, затем дату', () => {
		const { container } = renderWithTheme(
			<ExpiryLabel expiresAt={in17Hours} now={now} lead='remaining' />
		);

		expect(
			screen.getByText('ещё 17 ч').closest('.MuiChip-root')
		).not.toBeNull();
		expect(container.textContent).toMatch(/^ещё 17 ч\s*до /);
	});

	it('по умолчанию первой ставит дату, срок меньше суток — тоже чипом', () => {
		const { container } = renderWithTheme(
			<ExpiryLabel expiresAt={in17Hours} now={now} />
		);

		expect(container.textContent).toMatch(/ещё 17 ч$/);
		expect(container.textContent).not.toMatch(/^ещё/);
		expect(
			screen.getByText('ещё 17 ч').closest('.MuiChip-root')
		).not.toBeNull();
	});

	it('у истёкшего срока показывает переданную строку под чипом', () => {
		renderWithTheme(
			<ExpiryLabel
				expiresAt='2026-10-12T08:00:00Z'
				now={now}
				expiredLabel='просрочена'
				expiredNote='нужно списать'
			/>
		);

		expect(screen.getByText('просрочена')).toBeInTheDocument();
		expect(screen.getByText('нужно списать')).toBeInTheDocument();
		expect(screen.queryByText(/^истёк/)).toBeNull();
	});
});
