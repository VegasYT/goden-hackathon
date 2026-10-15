/** Равны ли значения на первом уровне: массивы — по элементам, объекты — по полям через `Object.is`. Сравнение для `compare` у `useSelector`. */
export function shallowEqual<T>(a: T, b: T): boolean {
	if (Object.is(a, b)) {
		return true;
	}

	if (
		typeof a !== 'object' ||
		typeof b !== 'object' ||
		a === null ||
		b === null ||
		Array.isArray(a) !== Array.isArray(b)
	) {
		return false;
	}

	const left = a as Record<string, unknown>;
	const right = b as Record<string, unknown>;
	const keys = Object.keys(left);

	return (
		keys.length === Object.keys(right).length &&
		keys.every(
			(key) => Object.hasOwn(right, key) && Object.is(left[key], right[key])
		)
	);
}
