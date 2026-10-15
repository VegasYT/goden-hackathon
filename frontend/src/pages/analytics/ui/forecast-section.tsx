import { Box, Chip, Stack, Typography } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { useEffect, useRef, useState } from 'react';

import { portionsOf } from '@entities/dish';
import { ProductHistoryLink } from '@entities/product';
import {
	formatShortDay,
	hourWindowLabels,
	hourWindows,
	type HourWindow,
} from '@shared/lib/datetime';
import { countOf } from '@shared/lib/plural';
import { EmptyState } from '@shared/ui/empty-state';
import { TextLink } from '@shared/ui/link';
import {
	SegmentedControl,
	type SegmentedOption,
} from '@shared/ui/segmented-control';

import { forecastQueryOptions, type StopListForecast } from '../api/analytics';
import {
	forecastReason,
	isSoon,
	momentLabel,
	untilLabel,
} from '../model/labels';
import {
	labeledTicks,
	timelineShare,
	timelineTicks,
	type TimelineTick,
} from '../model/timeline';
import { Section } from './section';

const horizonOptions: SegmentedOption<`${HourWindow}`>[] = hourWindows.map(
	(hours) => ({ value: `${hours}`, label: hourWindowLabels[hours] })
);

// Колонки строки на ноутбуке: блюдо, причина, момент, шкала.
const wideColumns = '220px minmax(0, 1.3fr) 150px minmax(0, 1fr)';

interface ForecastSectionProps {
	pointId: string;
	horizon: HourWindow;
	onHorizonChange: (horizon: HourWindow) => void;
	now: Date;
	wide: boolean;
	/** Название продукта ведёт в его историю на складе. */
	mayViewStock: boolean;
}

/** Когда блюдо закончится: через сколько и в какой момент. */
function When({
	item,
	now,
	align,
}: {
	item: StopListForecast;
	now: Date;
	align: 'flex-start' | 'flex-end';
}) {
	const label = untilLabel(item.expected_at, now);

	return (
		<Stack sx={{ gap: 0.5, alignItems: align, flexShrink: 0 }}>
			{isSoon(item.expected_at, now) ? (
				<Chip color='warning' label={label} />
			) : (
				<Typography
					variant='body2'
					component='span'
					sx={{ whiteSpace: 'nowrap' }}
				>
					{label}
				</Typography>
			)}
			<Typography variant='caption' sx={{ color: 'text.secondary' }}>
				{momentLabel(item.expected_at, now)}
			</Typography>
		</Stack>
	);
}

/** Шкала горизонта: полоса от текущего момента до момента прогноза и деления по полуночам. */
function Timeline({
	share,
	soon,
	ticks,
}: {
	share: number;
	soon: boolean;
	ticks: TimelineTick[];
}) {
	return (
		<Box
			aria-hidden
			sx={{
				position: 'relative',
				height: 8,
				borderRadius: 1,
				bgcolor: 'track',
			}}
		>
			<Box
				sx={{
					position: 'absolute',
					inset: 0,
					right: 'auto',
					width: `${share}%`,
					borderRadius: 1,
					bgcolor: soon ? 'warning.main' : 'neutralFill',
				}}
			/>
			{ticks.map((tick) => (
				<Box
					key={tick.at.getTime()}
					sx={{
						position: 'absolute',
						top: -3,
						bottom: -3,
						left: `${tick.share}%`,
						width: '1px',
						bgcolor: 'lineStrong',
					}}
				/>
			))}
		</Box>
	);
}

/** Ширина элемента в пикселях; без `ResizeObserver` (jsdom) — `undefined`. */
function useWidth() {
	const ref = useRef<HTMLDivElement>(null);
	const [width, setWidth] = useState<number>();

	useEffect(() => {
		const element = ref.current;

		if (!element || typeof ResizeObserver === 'undefined') {
			return;
		}

		const observer = new ResizeObserver(([entry]) =>
			setWidth(entry.contentRect.width)
		);

		observer.observe(element);

		return () => observer.disconnect();
	}, []);

	return [ref, width] as const;
}

/** Подписи делений над шкалой: "сейчас" и даты полуночей, которые помещаются. */
function TimelineAxis({ ticks }: { ticks: TimelineTick[] }) {
	const [ref, width] = useWidth();
	const shown =
		width === undefined
			? ticks.filter((tick) => tick.share > 12 && tick.share < 94)
			: labeledTicks(ticks, width);

	return (
		<Box ref={ref} aria-hidden sx={{ position: 'relative', height: 18 }}>
			<Typography
				variant='caption'
				sx={{ position: 'absolute', left: 0, color: 'text.tertiary' }}
			>
				сейчас
			</Typography>
			{shown.map((tick) => (
				<Typography
					key={tick.at.getTime()}
					variant='caption'
					sx={{
						position: 'absolute',
						left: `${tick.share}%`,
						transform: 'translateX(-50%)',
						color: 'text.tertiary',
						whiteSpace: 'nowrap',
					}}
				>
					{formatShortDay(tick.at)}
				</Typography>
			))}
		</Box>
	);
}

const rowSx = {
	px: 2,
	py: 1.5,
	borderBottom: '1px solid',
	borderColor: 'divider',
	'&:last-of-type': { borderBottom: 0 },
} as const;

/** Прогноз стоп-листа: какие блюда закончатся в горизонте, из-за какого продукта и когда. */
export function ForecastSection({
	pointId,
	horizon,
	onHorizonChange,
	now,
	wide,
	mayViewStock,
}: ForecastSectionProps) {
	const forecast = useQuery(forecastQueryOptions(pointId, horizon));
	const ticks = timelineTicks(now, horizon);

	const row = (item: StopListForecast) => {
		const soon = isSoon(item.expected_at, now);
		const dish = (
			<Box sx={{ minWidth: 0 }}>
				<Typography sx={{ fontWeight: 500, color: 'text.strong' }}>
					{item.dish.name}
				</Typography>
				<Typography variant='body2' sx={{ color: 'text.secondary' }}>
					хватит примерно на{' '}
					{countOf(portionsOf(item), 'порцию', 'порции', 'порций')}
				</Typography>
			</Box>
		);
		const reason = (
			<Typography variant='body2' sx={{ color: 'text.secondary' }}>
				<ProductHistoryLink
					pointId={pointId}
					product={item.limiting_product}
					linked={mayViewStock}
				/>
				: {forecastReason(item)}
			</Typography>
		);
		const timeline = (
			<Timeline
				share={timelineShare(item.expected_at, now, horizon)}
				soon={soon}
				ticks={ticks}
			/>
		);

		return (
			<Box
				component='li'
				key={item.dish.id}
				aria-label={item.dish.name}
				sx={rowSx}
			>
				{wide ? (
					<Box
						sx={{
							display: 'grid',
							gridTemplateColumns: wideColumns,
							gap: 2,
							alignItems: 'center',
						}}
					>
						{dish}
						{reason}
						<When item={item} now={now} align='flex-start' />
						{timeline}
					</Box>
				) : (
					<Stack sx={{ gap: 1 }}>
						<Stack
							direction='row'
							sx={{
								gap: 2,
								alignItems: 'flex-start',
								justifyContent: 'space-between',
							}}
						>
							{dish}
							<When item={item} now={now} align='flex-end' />
						</Stack>
						{reason}
						{timeline}
					</Stack>
				)}
			</Box>
		);
	};

	return (
		<Section
			title='Прогноз стоп-листа'
			description='Когда блюдо закончится, если расход останется как за последние сутки'
			subject='прогноз'
			query={forecast}
			action={
				<SegmentedControl
					label='Горизонт прогноза'
					options={horizonOptions}
					value={`${horizon}`}
					onChange={(value) => onHorizonChange(Number(value) as HourWindow)}
					fullWidth={!wide}
				/>
			}
			empty={
				<EmptyState
					icon='expiryWarning'
					title={`За ${hourWindowLabels[horizon]} блюда не закончатся`}
					description='Прогноз строится по расходу продуктов под заказы за последние сутки и по срокам годности партий.'
				/>
			}
			footer={
				<>
					Блюда, которые уже в стоп-листе, в прогноз не входят.{' '}
					<TextLink to='/points/$pointId/stop-list' params={{ pointId }}>
						Стоп-лист
					</TextLink>
				</>
			}
		>
			{(items) => (
				<>
					<Box
						sx={{
							px: 2,
							pt: 1.5,
							pb: 0.5,
							...(wide
								? { display: 'grid', gridTemplateColumns: wideColumns, gap: 2 }
								: {}),
						}}
					>
						{wide && (
							<>
								<Typography variant='caption' sx={{ color: 'text.tertiary' }}>
									Блюдо
								</Typography>
								<Typography variant='caption' sx={{ color: 'text.tertiary' }}>
									Закончится из-за
								</Typography>
								<Typography variant='caption' sx={{ color: 'text.tertiary' }}>
									Когда
								</Typography>
							</>
						)}
						<TimelineAxis ticks={ticks} />
					</Box>
					<Box
						component='ul'
						aria-label='Прогноз стоп-листа'
						sx={{ m: 0, p: 0, listStyle: 'none' }}
					>
						{items.map(row)}
					</Box>
				</>
			)}
		</Section>
	);
}
