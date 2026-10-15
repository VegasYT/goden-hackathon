import { Box, Paper, Typography } from '@mui/material';
import type { ReactNode } from 'react';

export function Section({
	title,
	description,
	children,
}: {
	title: string;
	description?: string;
	children: ReactNode;
}) {
	return (
		<Paper component='section' sx={{ p: 2.5 }}>
			<Typography variant='h2' sx={{ mb: description ? 0.5 : 2 }}>
				{title}
			</Typography>
			{description && (
				<Typography variant='body2' sx={{ color: 'text.secondary', mb: 2 }}>
					{description}
				</Typography>
			)}
			<Box>{children}</Box>
		</Paper>
	);
}
