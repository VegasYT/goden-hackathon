/** Номер заведения Мигома из поля ввода: целое число больше нуля; иначе `null`. */
export function parseOutletId(text: string): number | null {
	const value = text.trim();

	if (!/^\d+$/.test(value)) {
		return null;
	}

	const number = Number(value);

	return number > 0 && Number.isSafeInteger(number) ? number : null;
}
