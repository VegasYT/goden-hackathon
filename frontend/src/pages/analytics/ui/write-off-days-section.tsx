import { MenuItem, TextField, useTheme } from '@mui/material';
import { BarChart } from '@mui/x-charts/BarChart';
import { useQuery } from '@tanstack/react-query';

import { formatShortDay } from '@shared/lib/datetime';
import { formatAmount } from '@shared/lib/quantity';
import { EmptyState } from '@shared/ui/empty-state';
import { themeVars } from '@shared/ui/theme';

import { writeOffDaysQueryOptions, type PeriodParams } from '../api/analytics';
import { formatCount } from '../model/labels';
import type { Period } from '../model/search';
import { dayBars, writtenOffProducts } from '../model/write-off-days';
import { Section } from './section';

interface WriteOffDaysSectionProps {
	pointId: string;
	period: Period;
	params: PeriodParams;
	now: Date;
	/** Продукт, количество которого показывает график; без него - число операций. */
	product?: string;
	onProductChange: (product: string | undefined) => void;
}

/** Списания по дням периода: истечение срока и остальные причины. */
export function WriteOffDaysSection({
	pointId,
	period,
	params,
	now,
	product,
	onProductChange,
}: WriteOffDaysSectionProps) {
	const theme = useTheme();
	const days = useQuery(writeOffDaysQueryOptions(pointId, params));
	const products = writtenOffProducts(days.data ?? []);
	const selected = products.find((item) => item.id === product);

	return (
		<Section
			title='Списания по дням'
			description={
				selected
					? `Сколько списано за день: ${selected.name}`
					: 'Число операций списания за день; выберите продукт, чтобы увидеть количество'
			}
			subject='списания по дням'
			query={days}
			action={
				products.length > 0 && (
					<TextField
						select
						value={selected?.id ?? ''}
						onChange={(event) =>
							onProductChange(event.target.value || undefined)
						}
						sx={{ width: 220 }}
						slotProps={{
							select: { displayEmpty: true },
							htmlInput: { 'aria-label': 'Продукт' },
						}}
					>
						<MenuItem value=''>Все продукты</MenuItem>
						{products.map((item) => (
							<MenuItem key={item.id} value={item.id}>
								{item.name}
							</MenuItem>
						))}
					</TextField>
				)
			}
			empty={
				<EmptyState icon='writtenOff' title='За период ничего не списано' />
			}
		>
			{(items) => {
				const bars = dayBars(items, period, now, selected?.id);
				const format = (value: number | null) =>
					selected
						? formatAmount(value ?? 0, selected.unit)
						: formatCount(value ?? 0);

				return (
					<BarChart
						height={300}
						borderRadius={4}
						xAxis={[
							{
								scaleType: 'band',
								data: bars.days.map((day) => formatShortDay(day)),
							},
						]}
						yAxis={[{ valueFormatter: format, width: 64 }]}
						series={[
							{
								data: bars.expired,
								label: 'Истёк срок годности',
								stack: 'day',
								color: themeVars(theme).palette.warning.main,
								valueFormatter: format,
							},
							{
								data: bars.other,
								label: 'Порча и другие причины',
								stack: 'day',
								color: themeVars(theme).palette.neutralFill,
								valueFormatter: format,
							},
						]}
					/>
				);
			}}
		</Section>
	);
}
