import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';

import { renderApp, testPoint, testProfile } from '../../test-router';

const twoPoints = testProfile([
	testPoint(),
	testPoint({
		id: 'center',
		company_id: 'bridge',
		company_name: 'Кафе у моста',
		name: 'Центр',
		owner: false,
		role: 'cook',
		permissions: ['point.view', 'kitchen.view', 'catalog.view'],
	}),
]);

async function choosePoint(name: string) {
	await userEvent.click(
		await screen.findByRole('button', { name: /^Точка: .*Сменить точку$/ })
	);
	await userEvent.click(
		screen.getByRole('menuitemradio', { name: new RegExp(name) })
	);
}

describe('PointSwitcher', () => {
	it('при смене точки открывает тот же раздел, если на нём есть право', async () => {
		const { router } = await renderApp('/points/lenina/kitchen', {
			profile: twoPoints,
		});

		await choosePoint('Центр');

		expect(router.state.location.pathname).toBe('/points/center/kitchen');
	});

	it('без права на раздел открывает «Сводку» новой точки и сообщает об этом', async () => {
		const { router } = await renderApp('/points/lenina/stock', {
			profile: twoPoints,
		});

		await choosePoint('Центр');

		expect(router.state.location.pathname).toBe('/points/center');
		expect(
			await screen.findByText(
				'На точке «Центр» нет раздела «Склад» — открыта «Сводка»'
			)
		).toBeInTheDocument();
	});

	it('у сотрудника одной точки показывает её название без меню', async () => {
		await renderApp('/points/lenina', {
			profile: testProfile(
				[
					testPoint({
						owner: false,
						role: 'cook',
						permissions: ['point.view'],
					}),
				],
				[]
			),
		});

		expect(await screen.findByText('Точка на Ленина')).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: /Сменить точку/ })).toBeNull();
	});

	it('у владельца одной точки меню есть и предлагает новую компанию', async () => {
		await renderApp('/points/lenina');

		await userEvent.click(
			await screen.findByRole('button', { name: /Сменить точку/ })
		);

		expect(
			screen.getByRole('menuitem', { name: 'Новая компания' })
		).toBeInTheDocument();
	});
});
