import { Paper, Stack, Typography } from '@mui/material';
import type { ReactNode } from 'react';

import { LoadErrorState } from '@shared/ui/empty-state';
import { Icon } from '@shared/ui/icon';
import { PanelHeader } from '@shared/ui/panel-header';
import { SkeletonList } from '@shared/ui/skeleton-list';

import type { BlockQuery } from '../model/block-query';

interface PanelProps {
	title: string;
	/** Ссылка в раздел справа от заголовка. */
	action: ReactNode;
	query: BlockQuery;
	/** Ответа ещё нет: вместо списка заготовка строк. */
	loading: boolean;
	/** Что загружается, для программ чтения с экрана. */
	loadingLabel: string;
	errorTitle: string;
	children: ReactNode;
}

/** Список сводки: заголовок со ссылкой в раздел, заготовка строк, ошибка с «Повторить». */
export function Panel({
	title,
	action,
	query,
	loading,
	loadingLabel,
	errorTitle,
	children,
}: PanelProps) {
	const content = () => {
		if (query.error) {
			return <LoadErrorState title={errorTitle} query={query} />;
		}

		if (loading) {
			return (
				<SkeletonList
					label={loadingLabel}
					count={3}
					height={40}
					sx={{ p: 2 }}
				/>
			);
		}

		return children;
	};

	return (
		<Paper
			component='section'
			aria-label={title}
			sx={{ overflow: 'hidden', minWidth: 0 }}
		>
			<PanelHeader>
				<Typography variant='h3' component='h2'>
					{title}
				</Typography>
				{action}
			</PanelHeader>
			{content()}
		</Paper>
	);
}

/** Строка «всё в порядке» вместо пустого списка. */
export function CalmRow({ children }: { children: ReactNode }) {
	return (
		<Stack
			direction='row'
			sx={{
				alignItems: 'center',
				gap: 1,
				px: 2,
				py: 1.75,
				color: 'text.secondary',
			}}
		>
			<Icon name='check' sx={{ color: 'success.ink' }} />
			<Typography>{children}</Typography>
		</Stack>
	);
}

/** Строки списка с разделителями. */
export function Rows({
	label,
	children,
}: {
	label: string;
	children: ReactNode;
}) {
	return (
		<Stack
			component='ul'
			aria-label={label}
			sx={{
				listStyle: 'none',
				m: 0,
				p: 0,
				'& > li + li': { borderTop: '1px solid', borderColor: 'divider' },
			}}
		>
			{children}
		</Stack>
	);
}
