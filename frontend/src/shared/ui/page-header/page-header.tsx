import { Box, Stack, Typography } from '@mui/material';
import type { ReactNode } from 'react';

export interface PageHeaderProps {
	/** Единственный заголовок первого уровня на странице. */
	title: string;
	/** Путь к разделу, например ссылка «Поставки» над заголовком. */
	breadcrumbs?: ReactNode;
	/** Статус рядом с заголовком, например чип «ожидает приёмки». */
	status?: ReactNode;
	/** Строка подробностей под заголовком. */
	meta?: ReactNode;
	actions?: ReactNode;
}

export function PageHeader({
	title,
	breadcrumbs,
	status,
	meta,
	actions,
}: PageHeaderProps) {
	return (
		<Box component='header' sx={{ mb: 2.5 }}>
			{breadcrumbs && (
				<Typography
					component='nav'
					aria-label='Путь к разделу'
					variant='body2'
					sx={{ color: 'text.secondary', mb: 1 }}
				>
					{breadcrumbs}
				</Typography>
			)}
			<Stack
				direction='row'
				sx={{
					alignItems: 'flex-end',
					justifyContent: 'space-between',
					flexWrap: 'wrap',
					gap: 2,
				}}
			>
				<Box sx={{ minWidth: 0 }}>
					<Stack
						direction='row'
						sx={{
							alignItems: 'center',
							flexWrap: 'wrap',
							columnGap: 1.5,
							rowGap: 0.5,
						}}
					>
						<Typography variant='h1'>{title}</Typography>
						{status}
					</Stack>
					{meta && (
						<Typography sx={{ color: 'text.secondary', mt: 0.75 }}>
							{meta}
						</Typography>
					)}
				</Box>
				{actions && (
					<Stack direction='row' sx={{ gap: 1, flexWrap: 'wrap' }}>
						{actions}
					</Stack>
				)}
			</Stack>
		</Box>
	);
}
