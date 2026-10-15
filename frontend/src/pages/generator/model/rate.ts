export const maxRate = 600;

/** Ошибка интенсивности генератора: целое число в минуту от 0 до 600. */
export function rateError(value: number | null): string | undefined {
	if (value === null) {
		return 'Укажите число';
	}

	if (!Number.isInteger(value)) {
		return 'Только целое число';
	}

	if (value < 0) {
		return 'Не меньше 0';
	}

	return value > maxRate ? `Не больше ${maxRate}` : undefined;
}
