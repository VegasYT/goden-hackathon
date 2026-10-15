const rules = new Intl.PluralRules('ru-RU');

/** Слово в нужной форме для числа: `plural(3, 'позиция', 'позиции', 'позиций')` — «позиции». */
export function plural(count: number, one: string, few: string, many: string) {
	const form = rules.select(count);

	if (form === 'one') {
		return one;
	}

	return form === 'few' ? few : many;
}

/** Число со словом: «3 позиции». */
export function countOf(count: number, one: string, few: string, many: string) {
	return `${count} ${plural(count, one, few, many)}`;
}
