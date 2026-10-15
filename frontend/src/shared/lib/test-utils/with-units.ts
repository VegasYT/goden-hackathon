/** Имя с количеством для `getByRole(…, { name })`: пробел перед единицей неразрывный, а имя сравнивается без нормализации. */
export function withUnits(text: string): RegExp {
	return new RegExp(`^${text.replace(/ /g, '\\s')}$`);
}
