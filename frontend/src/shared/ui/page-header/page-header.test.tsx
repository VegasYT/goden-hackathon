import { Chip } from '@mui/material';
import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { renderWithTheme } from '@shared/lib/test-utils';

import { PageHeader } from './page-header';

describe('PageHeader', () => {
	it('даёт ровно один заголовок первого уровня с названием страницы', () => {
		renderWithTheme(
			<PageHeader
				title='Приёмка поставки № 118'
				breadcrumbs='Поставки'
				status={<Chip label='ожидает приёмки' />}
				meta='Поступила 10 октября в 09:40'
			/>
		);

		const headings = screen.getAllByRole('heading', { level: 1 });

		expect(headings).toHaveLength(1);
		expect(headings[0]).toHaveTextContent('Приёмка поставки № 118');
	});
});
