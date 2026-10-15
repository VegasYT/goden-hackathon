import userEvent from '@testing-library/user-event';

/** Заменяет текст поля и уходит с него: пустое `value` оставляет поле пустым. */
export async function retype(input: HTMLElement, value: string) {
	await userEvent.clear(input);

	if (value) {
		await userEvent.type(input, value);
	}

	await userEvent.tab();
}
