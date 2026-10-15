import type { ReactNode } from 'react';

import { LoadErrorState, type LoadQuery } from './load-error-state';

interface QueryStateProps<T> {
	query: LoadQuery & { data: T | undefined };
	/** Заготовка, пока ответа нет, обычно `SkeletonList`. */
	loading: ReactNode;
	/** Заголовок ошибки: «Не удалось загрузить меню». */
	errorTitle: string;
	children: (data: T) => ReactNode;
}

/** Загрузка, ошибка с «Повторить» или данные запроса. */
export function QueryState<T>({
	query,
	loading,
	errorTitle,
	children,
}: QueryStateProps<T>) {
	if (query.error) {
		return <LoadErrorState title={errorTitle} query={query} />;
	}

	if (query.data === undefined) {
		return loading;
	}

	return children(query.data);
}
