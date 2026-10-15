import { Skeleton, Stack } from '@mui/material';

import { PageContent } from '@shared/ui/page-content';

/** Заготовка карточки поставки, пока она загружается. */
export function DeliveryLoading() {
	return (
		<PageContent>
			<Stack role='status' aria-label='Загружаем поставку' sx={{ gap: 1.5 }}>
				<Skeleton variant='text' width={120} />
				<Skeleton variant='text' width={360} height={40} />
				<Skeleton variant='text' width={480} />
				{Array.from({ length: 5 }, (_, index) => (
					<Skeleton key={index} variant='rounded' height={48} />
				))}
			</Stack>
		</PageContent>
	);
}
