import { Button, Stack, Typography } from '@mui/material';
import { format, startOfDay } from 'date-fns';
import { useState } from 'react';

import { DateField } from '@shared/ui/date-field';
import { FormDialog } from '@shared/ui/form-dialog';
import { Icon } from '@shared/ui/icon';
import {
	SegmentedControl,
	type SegmentedOption,
} from '@shared/ui/segmented-control';

import {
	lastDayOf,
	periodLabel,
	type AnalyticsSearch,
	type Period,
	type PeriodChoice,
} from '../model/search';

const options: SegmentedOption<PeriodChoice>[] = [
	{ value: '7d', label: '7 дней' },
	{ value: '30d', label: '30 дней' },
	{ value: 'custom', label: 'Свой период' },
];

type PeriodPatch = Pick<AnalyticsSearch, 'period' | 'from' | 'to'>;

/** Формат дня в адресе. */
const dayFormat = 'yyyy-MM-dd';

/** Даты своего периода; оба дня входят в период. */
function CustomPeriodDialog({
	period,
	now,
	onApply,
	onClose,
}: {
	period: Period;
	now: Date;
	onApply: (patch: PeriodPatch) => void;
	onClose: () => void;
}) {
	const [from, setFrom] = useState<Date | null>(period.start);
	const [to, setTo] = useState<Date | null>(lastDayOf(period, now));
	const reversed =
		from !== null && to !== null && startOfDay(from) > startOfDay(to);

	return (
		<FormDialog
			title='Свой период'
			submitLabel='Показать'
			submitDisabled={from === null || to === null || reversed}
			onSubmit={() =>
				from &&
				to &&
				onApply({
					period: 'custom',
					from: format(from, dayFormat),
					to: format(to, dayFormat),
				})
			}
			onClose={onClose}
		>
			<DateField label='С' value={from} onChange={setFrom} fullWidth />
			<DateField
				label='По'
				value={to}
				onChange={setTo}
				fullWidth
				error={reversed}
				helperText={
					reversed
						? 'Конец периода раньше начала'
						: 'Оба дня входят в период целиком'
				}
			/>
		</FormDialog>
	);
}

interface PeriodControlProps {
	period: Period;
	now: Date;
	wide: boolean;
	onChange: (patch: PeriodPatch) => void;
}

/** Период потерь и дефицита: 7 дней, 30 дней или свой. */
export function PeriodControl({
	period,
	now,
	wide,
	onChange,
}: PeriodControlProps) {
	const [dialogOpen, setDialogOpen] = useState(false);

	return (
		<Stack
			direction={wide ? 'row' : 'column'}
			sx={{
				gap: 1.5,
				alignItems: wide ? 'flex-end' : 'stretch',
				justifyContent: 'space-between',
				mt: 3.5,
				mb: 1.5,
			}}
		>
			<div>
				<Typography variant='h2'>Потери и дефицит</Typography>
				<Stack direction='row' sx={{ gap: 1, alignItems: 'center', mt: 0.25 }}>
					<Typography sx={{ color: 'text.secondary' }}>
						{periodLabel(period, now)}
					</Typography>
					{period.choice === 'custom' && (
						<Button
							size='small'
							startIcon={<Icon name='edit' />}
							onClick={() => setDialogOpen(true)}
						>
							Изменить даты
						</Button>
					)}
				</Stack>
			</div>
			<SegmentedControl
				label='Период'
				options={options}
				value={period.choice}
				fullWidth={!wide}
				onChange={(choice) =>
					choice === 'custom'
						? setDialogOpen(true)
						: onChange({ period: choice, from: undefined, to: undefined })
				}
			/>
			{dialogOpen && (
				<CustomPeriodDialog
					period={period}
					now={now}
					onClose={() => setDialogOpen(false)}
					onApply={(patch) => {
						setDialogOpen(false);
						onChange(patch);
					}}
				/>
			)}
		</Stack>
	);
}
