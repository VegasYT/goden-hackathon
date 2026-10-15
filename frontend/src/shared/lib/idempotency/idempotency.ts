import { useRef } from 'react';

/**
 * Ключ идемпотентности для повторов записи: `keyFor(body)` возвращает прежний ключ,
 * пока тело запроса не изменилось, и новый — для изменённого тела.
 */
export function useIdempotencyKey() {
	const attempt = useRef<{ key: string; body: string } | null>(null);

	// Повтор с тем же телом идёт под тем же ключом, чтобы сервер не выполнил запись дважды.
	return (body: unknown): string => {
		const serialized = JSON.stringify(body);

		if (attempt.current?.body !== serialized) {
			attempt.current = { key: crypto.randomUUID(), body: serialized };
		}

		return attempt.current.key;
	};
}
