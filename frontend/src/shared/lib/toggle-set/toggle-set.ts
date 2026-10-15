import { useState } from 'react';

/** Набор раскрытых строк списка: `toggle(id)` раскрывает строку или сворачивает раскрытую. */
export function useToggleSet() {
	const [items, setItems] = useState<ReadonlySet<string>>(new Set());

	const toggle = (id: string) =>
		setItems((previous) => {
			const next = new Set(previous);

			if (!next.delete(id)) {
				next.add(id);
			}

			return next;
		});

	return [items, toggle] as const;
}
