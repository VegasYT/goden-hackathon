import { Box } from '@mui/material';

import { can, usePoint } from '@entities/profile';
import { useNow } from '@shared/lib/expiry';
import { PageContent } from '@shared/ui/page-content';
import { PageHeader } from '@shared/ui/page-header';
import { useWideLayout } from '@shared/ui/theme';

import {
	periodOf,
	toPeriodParams,
	type AnalyticsSearch,
} from '../model/search';
import { ForecastSection } from './forecast-section';
import { PeriodControl } from './period-control';
import { ShortagesSection } from './shortages-section';
import { WriteOffDaysSection } from './write-off-days-section';
import { WriteOffsSection } from './write-offs-section';

interface AnalyticsPageProps {
	pointId: string;
	search: AnalyticsSearch;
	/** Меняет состояние аналитики в адресе. */
	onSearchChange: (patch: Partial<AnalyticsSearch>) => void;
}

/** Аналитика точки: прогноз стоп-листа, дефицитные ингредиенты и списания за период. */
export function AnalyticsPage({
	pointId,
	search,
	onSearchChange,
}: AnalyticsPageProps) {
	const wide = useWideLayout();
	const now = useNow();
	const { point } = usePoint(pointId);
	const mayViewStock = can(point, 'inventory.view');
	const period = periodOf(search, now);
	const params = toPeriodParams(period);

	return (
		<PageContent>
			<PageHeader title='Аналитика' meta={point.name} />
			<ForecastSection
				pointId={pointId}
				horizon={search.horizon}
				onHorizonChange={(horizon) => onSearchChange({ horizon })}
				now={now}
				wide={wide}
				mayViewStock={mayViewStock}
			/>
			<PeriodControl
				period={period}
				now={now}
				wide={wide}
				onChange={onSearchChange}
			/>
			<Box
				sx={{
					display: 'grid',
					gridTemplateColumns: wide
						? 'repeat(2, minmax(0, 1fr))'
						: 'minmax(0, 1fr)',
					gap: 2,
					alignItems: 'start',
				}}
			>
				<ShortagesSection
					pointId={pointId}
					period={params}
					mayViewStock={mayViewStock}
				/>
				<WriteOffsSection
					pointId={pointId}
					period={params}
					view={search.writeOffs}
					onViewChange={(writeOffs) => onSearchChange({ writeOffs })}
					wide={wide}
					mayViewStock={mayViewStock}
				/>
			</Box>
			<Box sx={{ mt: 2 }}>
				<WriteOffDaysSection
					pointId={pointId}
					period={period}
					params={params}
					now={now}
					product={search.dayProduct}
					onProductChange={(dayProduct) => onSearchChange({ dayProduct })}
				/>
			</Box>
		</PageContent>
	);
}
