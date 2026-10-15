import { Chip, Stack, Typography } from '@mui/material';

import { formatDateTime } from '@shared/lib/datetime';
import { expiryStatus, remainingLabel } from '@shared/lib/expiry';

interface ExpiryLabelProps {
	expiresAt: string;
	now: Date;
	/** Что идёт первым: дата «годен до» (её сверяют с накладной) или оставшийся срок. */
	lead?: 'date' | 'remaining';
	/** `false` — без даты, когда она стоит в соседней колонке. */
	withDate?: boolean;
	/** Части в одну строку, для карточек и пунктов выбора. */
	inline?: boolean;
	/** Подпись просрочки на чипе; по умолчанию «просрочен». */
	expiredLabel?: string;
	/** Строка под чипом просрочки; по умолчанию «истёк <дата>». */
	expiredNote?: string;
}

/** Срок годности: дата и оставшееся время, не больше суток — жёлтым чипом, просрочка — красным. */
export function ExpiryLabel({
	expiresAt,
	now,
	lead = 'date',
	withDate = true,
	inline = false,
	expiredLabel = 'просрочен',
	expiredNote,
}: ExpiryLabelProps) {
	const status = expiryStatus(expiresAt, now);
	const date = formatDateTime(expiresAt, now);
	const layout = {
		direction: inline ? 'row' : 'column',
		sx: {
			gap: inline ? 0.75 : 0.25,
			alignItems: inline ? 'center' : 'flex-start',
		},
	} as const;

	if (status === 'expired') {
		return (
			<Stack {...layout}>
				<Chip color='error' label={expiredLabel} />
				<Typography variant='caption' sx={{ color: 'error.ink' }}>
					{expiredNote ?? `истёк ${date}`}
				</Typography>
			</Stack>
		);
	}

	const remaining = remainingLabel(expiresAt, now);
	const remainingPart =
		status === 'soon' ? (
			<Chip key='remaining' color='warning' label={remaining} />
		) : (
			<Typography
				key='remaining'
				variant={lead === 'remaining' ? 'body2' : 'caption'}
				sx={{ color: lead === 'remaining' ? undefined : 'text.secondary' }}
			>
				{remaining}
			</Typography>
		);

	if (!withDate) {
		return <Stack {...layout}>{remainingPart}</Stack>;
	}

	const datePart =
		lead === 'date' ? (
			<Typography key='date' variant='body2'>
				{date}
			</Typography>
		) : (
			<Typography
				key='date'
				variant='caption'
				sx={{ color: 'text.secondary', whiteSpace: 'nowrap' }}
			>
				до {date}
			</Typography>
		);

	return (
		<Stack {...layout}>
			{lead === 'date' ? [datePart, remainingPart] : [remainingPart, datePart]}
		</Stack>
	);
}
