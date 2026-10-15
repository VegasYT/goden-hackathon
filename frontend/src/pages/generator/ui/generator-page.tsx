import { Box } from '@mui/material';

import { isPointDisabled, usePoint } from '@entities/profile';
import { PageContent } from '@shared/ui/page-content';
import { PageHeader } from '@shared/ui/page-header';

import { GeneratorCard } from './generator-card';
import { TestDeliveryCard } from './test-delivery-card';

/** Раздел «Генератор» владельца: генератор событий точки и тестовая поставка для ручной приёмки. */
export function GeneratorPage({ pointId }: { pointId: string }) {
	const { profile, point } = usePoint(pointId);
	const disabled = isPointDisabled(profile, point);

	return (
		<PageContent>
			<PageHeader
				title='Генератор'
				meta={`Эмулирует работу точки «${point.name}»: привозит поставки и оформляет заказы. Для демонстрации и проверки.`}
			/>
			<Box
				sx={{
					display: 'grid',
					gridTemplateColumns: { xs: '1fr', lg: 'repeat(2, minmax(0, 1fr))' },
					gap: 2.5,
					alignItems: 'start',
				}}
			>
				<GeneratorCard pointId={pointId} disabled={disabled} />
				<TestDeliveryCard
					pointId={pointId}
					companyId={point.company_id}
					disabled={disabled}
				/>
			</Box>
		</PageContent>
	);
}
