/** Номер чата Telegram из поля ввода: целое число со знаком; иначе `null`. */
export function parseChatId(text: string): number | null {
	const value = text.trim().replace(/^[−–]/, '-');

	if (!/^-?\d+$/.test(value)) {
		return null;
	}

	const number = Number(value);

	return Number.isSafeInteger(number) ? number : null;
}
