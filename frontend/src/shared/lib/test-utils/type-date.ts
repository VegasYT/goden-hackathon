import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

/** Вводит дату в поле пикера по его подписи: `typeDate('Годен до', '17.10.2026 23:59')`. */
export async function typeDate(label: string, value: string) {
	const field = screen.getByRole('group', { name: label });

	await userEvent.click(within(field).getAllByRole('spinbutton')[0]);
	await userEvent.paste(value);
}
