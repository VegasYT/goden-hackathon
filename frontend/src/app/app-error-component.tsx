import {
	ErrorComponent,
	type ErrorComponentProps,
} from '@tanstack/react-router';

import { ForbiddenPage } from '@pages/errors/forbidden';

import { ForbiddenError } from './point-access';

/** Компонент ошибки роутера приложения. */
export function AppErrorComponent({ error }: ErrorComponentProps) {
	if (error instanceof ForbiddenError) {
		return <ForbiddenPage section={error.section} />;
	}

	return <ErrorComponent error={error} />;
}
