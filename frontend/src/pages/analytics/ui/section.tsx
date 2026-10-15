import { Box, Paper, Typography } from '@mui/material';
import type { UseQueryResult } from '@tanstack/react-query';
import { useId, type ReactNode } from 'react';

import { QueryState } from '@shared/ui/empty-state';
import { PanelHeader } from '@shared/ui/panel-header';
import { SkeletonList } from '@shared/ui/skeleton-list';

interface SectionProps<T> {
	title: string;
	description: string;
	/** Переключатель в шапке блока. */
	action?: ReactNode;
	query: UseQueryResult<T[]>;
	/** Что сейчас загружается и что не загрузилось: "прогноз". */
	subject: string;
	/** Показывается вместо пустого списка. */
	empty: ReactNode;
	children: (items: T[]) => ReactNode;
	footer?: ReactNode;
	/** Уровень заголовка блока в структуре страницы. */
	headingLevel?: 'h2' | 'h3';
}

/** Блок аналитики: шапка, загрузка, ошибка с повтором, пусто или содержимое. */
export function Section<T>({
	title,
	description,
	action,
	query,
	subject,
	empty,
	children,
	footer,
	headingLevel = 'h2',
}: SectionProps<T>) {
	const titleId = useId();

	return (
		<Paper
			component='section'
			aria-labelledby={titleId}
			sx={{ overflow: 'hidden', minWidth: 0 }}
		>
			<PanelHeader>
				<Box sx={{ minWidth: 0 }}>
					<Typography id={titleId} variant='h3' component={headingLevel}>
						{title}
					</Typography>
					<Typography variant='caption' sx={{ color: 'text.secondary' }}>
						{description}
					</Typography>
				</Box>
				{action}
			</PanelHeader>
			<QueryState
				query={query}
				loading={
					<SkeletonList
						label={`Загружаем ${subject}`}
						count={4}
						height={36}
						sx={{ p: 2 }}
					/>
				}
				errorTitle={`Не удалось загрузить ${subject}`}
			>
				{(items) => (items.length === 0 ? empty : children(items))}
			</QueryState>
			{footer && query.isSuccess && query.data.length > 0 && (
				<Typography
					variant='caption'
					component='p'
					sx={{
						m: 0,
						px: 2,
						py: 1.25,
						color: 'text.secondary',
						borderTop: '1px solid',
						borderColor: 'divider',
					}}
				>
					{footer}
				</Typography>
			)}
		</Paper>
	);
}
