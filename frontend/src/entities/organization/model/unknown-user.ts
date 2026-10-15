import { ApiError } from '@shared/api';

/** Почта из запроса, если сервер ответил, что пользователя с ней нет; иначе `undefined`. */
export function unknownUserEmail(
	error: unknown,
	email: string | undefined
): string | undefined {
	return error instanceof ApiError &&
		error.status === 422 &&
		error.type === 'unknown-reference'
		? email
		: undefined;
}
