import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { renderWithTheme } from '@shared/lib/test-utils';

import { Icon } from './icon';

describe('Icon', () => {
	it('без label иконка декоративная и скрыта от программ чтения', () => {
		const { container } = renderWithTheme(<Icon name='check' />);
		const svg = container.querySelector('svg');

		expect(svg).toHaveAttribute('aria-hidden', 'true');
		expect(screen.queryByRole('img')).toBeNull();
	});

	it('с label иконка озвучивается как изображение', () => {
		renderWithTheme(<Icon name='warning' label='Нужно решение' />);

		expect(
			screen.getByRole('img', { name: 'Нужно решение' })
		).toBeInTheDocument();
	});

	it('filled выбирает заливную версию', () => {
		const { container: outline } = renderWithTheme(<Icon name='deliveries' />);
		const { container: filled } = renderWithTheme(
			<Icon name='deliveries' filled />
		);

		const path = (c: HTMLElement) => c.querySelector('path')?.getAttribute('d');

		expect(path(filled)).not.toBe(path(outline));
	});
});
