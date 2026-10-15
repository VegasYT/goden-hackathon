import {
	LinearProgress,
	Stack,
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableRow,
} from '@mui/material';
import { useQuery } from '@tanstack/react-query';

import { ProductHistoryLink } from '@entities/product';
import { EmptyState } from '@shared/ui/empty-state';

import { shortagesQueryOptions, type PeriodParams } from '../api/analytics';
import { formatCount, formatDuration } from '../model/labels';
import { Section } from './section';

interface ShortagesSectionProps {
	pointId: string;
	period: PeriodParams;
	mayViewStock: boolean;
}

/** Продукты, из-за которых блюда стояли в стоп-листе и отклонялись заказы за период. */
export function ShortagesSection({
	pointId,
	period,
	mayViewStock,
}: ShortagesSectionProps) {
	const shortages = useQuery(shortagesQueryOptions(pointId, period));

	return (
		<Section
			title='Дефицитные ингредиенты'
			description='Сначала те, из-за которых блюда дольше стояли в стоп-листе'
			subject='дефицитные ингредиенты'
			headingLevel='h3'
			query={shortages}
			empty={
				<EmptyState
					icon='done'
					title='За период блюда не попадали в стоп-лист'
					description='Ни один заказ не отклонён из-за нехватки продуктов.'
				/>
			}
			footer='Отказ - заказ, который не прошёл из-за нехватки продукта.'
		>
			{(items) => {
				const longest = Math.max(...items.map((item) => item.stop_seconds), 1);

				return (
					<Table aria-label='Дефицитные ингредиенты'>
						<TableHead>
							<TableRow>
								<TableCell sx={{ pl: 2 }}>Продукт</TableCell>
								<TableCell>Блюда в стоп-листе</TableCell>
								<TableCell align='right'>Попаданий</TableCell>
								<TableCell align='right' sx={{ pr: 2 }}>
									Отказов
								</TableCell>
							</TableRow>
						</TableHead>
						<TableBody>
							{items.map((item) => (
								<TableRow key={item.product.id}>
									<TableCell sx={{ pl: 2 }}>
										<ProductHistoryLink
											pointId={pointId}
											product={item.product}
											linked={mayViewStock}
										/>
									</TableCell>
									<TableCell>
										<Stack
											direction='row'
											sx={{ gap: 1.25, alignItems: 'center' }}
										>
											<LinearProgress
												variant='determinate'
												value={(item.stop_seconds / longest) * 100}
												aria-hidden
												sx={{
													width: 96,
													flexShrink: 0,
													'& .MuiLinearProgress-bar': {
														bgcolor: 'neutralFill',
													},
												}}
											/>
											<span>
												{item.stop_seconds > 0
													? formatDuration(item.stop_seconds * 1000)
													: 'не было'}
											</span>
										</Stack>
									</TableCell>
									<TableCell align='right'>
										{formatCount(item.stop_entries)}
									</TableCell>
									<TableCell align='right' sx={{ pr: 2 }}>
										{formatCount(item.rejected_orders)}
									</TableCell>
								</TableRow>
							))}
						</TableBody>
					</Table>
				);
			}}
		</Section>
	);
}
