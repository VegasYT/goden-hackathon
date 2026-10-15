import { Box } from '@mui/material';

import { usePoint } from '@entities/profile';
import { useNow } from '@shared/lib/expiry';
import { PageContent } from '@shared/ui/page-content';
import { PageHeader } from '@shared/ui/page-header';

import { dashboardBlocks } from '../model/blocks';
import { ExpiringList, HotList, StopList } from './lists';
import {
	ExpiringTile,
	HotTile,
	PendingTile,
	QuarantineTile,
	StopListTile,
} from './tiles';

/** Состояние точки на одном экране: счётчики-ссылки и короткие списки по правам роли. */
export function DashboardPage({ pointId }: { pointId: string }) {
	const now = useNow();
	const { point } = usePoint(pointId);
	const blocks = dashboardBlocks(point);
	const side = blocks.stopList || blocks.hot;

	return (
		<PageContent>
			<PageHeader
				title='Сводка'
				meta={`${point.name} · что требует внимания сейчас`}
			/>
			<Box
				sx={{
					display: 'grid',
					// Сетка на все счётчики, чтобы у роли с частью блоков они были того же размера.
					gridTemplateColumns: {
						xs: 'repeat(2, minmax(0, 1fr))',
						sm: 'repeat(3, minmax(0, 1fr))',
						lg: 'repeat(5, minmax(0, 1fr))',
					},
					gap: 1.5,
					mb: 2,
				}}
			>
				{blocks.expiring && <ExpiringTile pointId={pointId} now={now} />}
				{blocks.stopList && <StopListTile pointId={pointId} />}
				{blocks.hot && <HotTile pointId={pointId} />}
				{blocks.deliveries && <PendingTile pointId={pointId} now={now} />}
				{blocks.deliveries && <QuarantineTile pointId={pointId} />}
			</Box>
			<Box
				sx={{
					display: 'grid',
					gridTemplateColumns: {
						lg:
							blocks.expiring && side
								? 'minmax(0, 1.2fr) minmax(0, 1fr)'
								: undefined,
					},
					gap: 2,
					alignItems: 'start',
				}}
			>
				{blocks.expiring && <ExpiringList pointId={pointId} now={now} />}
				{side && (
					<Box
						sx={{
							display: 'grid',
							gap: 2,
							minWidth: 0,
							alignItems: 'start',
							gridTemplateColumns: {
								lg:
									!blocks.expiring && blocks.stopList && blocks.hot
										? 'repeat(2, minmax(0, 1fr))'
										: undefined,
							},
						}}
					>
						{blocks.stopList && <StopList pointId={pointId} />}
						{blocks.hot && <HotList pointId={pointId} now={now} />}
					</Box>
				)}
			</Box>
		</PageContent>
	);
}
