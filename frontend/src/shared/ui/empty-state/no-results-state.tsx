import { Button } from '@mui/material';
import type { ReactNode } from 'react';

import { Icon } from '@shared/ui/icon';

import { EmptyState } from './empty-state';

interface NoResultsStateProps {
	title?: string;
	description?: ReactNode;
	/** Подпись кнопки сброса: «Сбросить поиск» или «Сбросить фильтры». */
	resetLabel?: string;
	onReset: () => void;
}

/** Поиск или фильтры ничего не нашли: что искали и кнопка сброса. */
export function NoResultsState({
	title = 'Ничего не нашлось',
	description,
	resetLabel = 'Сбросить поиск',
	onReset,
}: NoResultsStateProps) {
	return (
		<EmptyState
			icon='notFound'
			title={title}
			description={description}
			action={
				<Button
					variant='outlined'
					startIcon={<Icon name='close' />}
					onClick={onReset}
				>
					{resetLabel}
				</Button>
			}
		/>
	);
}
