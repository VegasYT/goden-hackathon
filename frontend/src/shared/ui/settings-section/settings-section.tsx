import { Box, Paper, Stack, Typography } from '@mui/material';
import { useId, type ReactNode } from 'react';

interface SettingsSectionProps {
	title: string;
	description?: ReactNode;
	/** Действие справа от заголовка; в узкой карточке переносится под текст. */
	action?: ReactNode;
	/** Необратимое действие: рамка в цвете ошибки. */
	danger?: boolean;
	children?: ReactNode;
}

/** Карточка настроек с заголовком второго уровня, пояснением и содержимым. */
export function SettingsSection({
	title,
	description,
	action,
	danger = false,
	children,
}: SettingsSectionProps) {
	const titleId = useId();

	return (
		<Paper
			component='section'
			aria-labelledby={titleId}
			sx={{
				p: 2.5,
				...(danger && {
					borderColor: 'error.main',
					borderStyle: 'solid',
					borderWidth: 1,
				}),
			}}
		>
			<Stack
				direction='row'
				sx={{
					alignItems: 'flex-start',
					justifyContent: 'space-between',
					flexWrap: 'wrap',
					gap: 2,
				}}
			>
				<Box sx={{ flex: '1 1 320px', minWidth: 0 }}>
					<Typography id={titleId} variant='h2'>
						{title}
					</Typography>
					{description && (
						<Typography sx={{ color: 'text.secondary', mt: 0.5 }}>
							{description}
						</Typography>
					)}
				</Box>
				{action}
			</Stack>
			{children && <Stack sx={{ mt: 2, gap: 1.5 }}>{children}</Stack>}
		</Paper>
	);
}
