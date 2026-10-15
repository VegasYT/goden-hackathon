import { Button } from '@mui/material';

import { Icon } from '@shared/ui/icon';

import { EmptyState } from './empty-state';

/** Запрос для состояний загрузки и ошибки: подходит результат `useQuery`. */
export interface LoadQuery {
	error: Error | null;
	isFetching: boolean;
	refetch: () => unknown;
}

/** Ошибка загрузки списка или панели: текст ошибки запроса и «Повторить». */
export function LoadErrorState({
	title,
	query,
}: {
	title: string;
	query: LoadQuery;
}) {
	return (
		<EmptyState
			icon='cloudOff'
			title={title}
			description={query.error?.message}
			action={
				<Button
					variant='outlined'
					startIcon={<Icon name='refresh' />}
					loading={query.isFetching}
					onClick={() => void query.refetch()}
				>
					Повторить
				</Button>
			}
		/>
	);
}
