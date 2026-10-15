import { Box, Button, Paper, Skeleton, Stack, Typography } from '@mui/material';
import type { ReactNode } from 'react';

import { Icon, type IconName } from '@shared/ui/icon';
import { radius } from '@shared/ui/theme';

import type { BlockQuery } from '../model/block-query';
import type { Tile, TileTone } from '../model/tiles';

interface StatTileProps {
	icon: IconName;
	label: string;
	query: BlockQuery;
	/** Счётчик по ответу; `undefined`, пока ответа нет. */
	tile: Tile | undefined;
	/** Ссылка в раздел вокруг содержимого счётчика. */
	link: (content: ReactNode) => ReactNode;
}

const toneColors: Record<TileTone, { tint: string; ink: string }> = {
	ok: { tint: 'success.tint', ink: 'success.ink' },
	neutral: { tint: 'track', ink: 'text.strong' },
	warning: { tint: 'warning.tint', ink: 'warning.ink' },
	error: { tint: 'error.tint', ink: 'error.ink' },
};

const contentSx = { display: 'grid', alignContent: 'start', gap: 0.75, p: 2 };

function Label({
	icon,
	label,
	tone,
}: {
	icon: IconName;
	label: string;
	tone: TileTone;
}) {
	return (
		<Stack direction='row' sx={{ alignItems: 'center', gap: 1, minWidth: 0 }}>
			<Box
				sx={(theme) => ({
					display: 'grid',
					placeItems: 'center',
					flexShrink: 0,
					width: 28,
					height: 28,
					borderRadius: radius(theme, 'sm'),
					bgcolor: toneColors[tone].tint,
					color: toneColors[tone].ink,
				})}
			>
				<Icon name={icon} size='sm' />
			</Box>
			<Typography
				variant='body2'
				component='span'
				noWrap
				sx={{ color: 'text.secondary' }}
			>
				{label}
			</Typography>
		</Stack>
	);
}

/** Счётчик сводки: число со словом и подписью, ведёт в свой раздел; пока данных нет — заготовка, при ошибке — «Повторить». */
export function StatTile({ icon, label, query, tile, link }: StatTileProps) {
	const content = () => {
		if (query.error) {
			return (
				<Box sx={contentSx} role='group' aria-label={label}>
					<Label icon={icon} label={label} tone='neutral' />
					<Stack direction='row' sx={{ alignItems: 'center', gap: 1 }}>
						<Icon name='cloudOff' sx={{ color: 'text.secondary' }} />
						<Typography sx={{ color: 'text.strong', fontWeight: 500 }}>
							Не загрузилось
						</Typography>
					</Stack>
					<Button
						variant='outlined'
						size='small'
						startIcon={<Icon name='refresh' />}
						loading={query.isFetching}
						onClick={query.refetch}
						sx={{ justifySelf: 'start' }}
					>
						Повторить
					</Button>
				</Box>
			);
		}

		if (!tile) {
			return (
				<Box sx={contentSx} aria-busy='true' aria-label={label}>
					<Label icon={icon} label={label} tone='neutral' />
					<Skeleton variant='rounded' width='40%' height={33} />
					<Skeleton variant='text' width='80%' />
				</Box>
			);
		}

		return link(
			<Box sx={contentSx}>
				<Label icon={icon} label={label} tone={tile.tone} />
				<Stack
					direction='row'
					sx={{ alignItems: 'baseline', gap: 0.75, minWidth: 0 }}
				>
					<Typography
						component='span'
						sx={{
							fontSize: 30,
							lineHeight: 1.1,
							fontWeight: 600,
							letterSpacing: '-0.02em',
							color: toneColors[tile.tone].ink,
						}}
					>
						{tile.value}
					</Typography>
					<Typography component='span' sx={{ color: 'text.secondary' }}>
						{tile.unit}
					</Typography>
				</Stack>
				<Typography variant='body2' sx={{ color: 'text.secondary' }}>
					{tile.note}
				</Typography>
			</Box>
		);
	};

	// Сетка растягивает ссылку на всю высоту ряда счётчиков.
	return (
		<Paper sx={{ display: 'grid', overflow: 'hidden', minWidth: 0 }}>
			{content()}
		</Paper>
	);
}
