import { Box, Divider, Stack, Typography } from '@mui/material';
import { Fragment } from 'react';

import {
	itemName,
	itemUnit,
	positionsCount,
	reasonLabel,
	type DeliveryItem,
} from '@entities/delivery';
import { formatQuantity } from '@shared/lib/quantity';
import { Icon } from '@shared/ui/icon';

import type { Decision, Part } from '../model/decision';
import { useDecisions, type ApprovalForm } from '../model/form';
import { outcomeParts } from './parts';

const reasonOf: Partial<Record<Part, 'rejectReason' | 'quarantineReason'>> = {
	rejected: 'rejectReason',
	quarantined: 'quarantineReason',
};

function note(decision: Decision, part: Part): string | undefined {
	const field = reasonOf[part];

	return field
		? `${reasonLabel(decision[field])} — «${decision.comment.trim()}»`
		: undefined;
}

/** Итог приёмки по исходам для диалога подтверждения; читает решения, только пока открыт. */
export function ApprovalSummary({
	form,
	items,
}: {
	form: ApprovalForm;
	items: DeliveryItem[];
}) {
	const decisions = useDecisions(form);
	const groups = outcomeParts
		.map(({ part, label, icon }) => ({
			part,
			label,
			icon,
			lines: items.flatMap((item, index) => {
				const quantity = decisions[index][part] ?? 0;

				return quantity > 0
					? [{ item, quantity, note: note(decisions[index], part) }]
					: [];
			}),
		}))
		.filter(({ lines }) => lines.length > 0);

	return (
		<Stack divider={<Divider />} sx={{ gap: 1.5 }}>
			{groups.map(({ part, label, icon, lines }) => (
				<Box key={part} component='section' aria-label={label}>
					<Typography
						variant='body2'
						sx={{
							display: 'flex',
							gap: 1,
							alignItems: 'center',
							fontWeight: 600,
							mb: 0.75,
						}}
					>
						<Icon name={icon} sx={{ color: 'text.secondary' }} />
						{label} · {positionsCount(lines.length)}
					</Typography>
					<Box
						component='dl'
						sx={{
							display: 'grid',
							gridTemplateColumns: '1fr auto',
							columnGap: 2,
							rowGap: 0.5,
							m: 0,
							pl: 3.5,
						}}
					>
						{lines.map(({ item, quantity, note: text }) => (
							<Fragment key={item.id}>
								<Box component='dt'>
									<Typography variant='body2'>{itemName(item)}</Typography>
									{text && (
										<Typography
											variant='caption'
											component='div'
											sx={{ color: 'text.secondary' }}
										>
											{text}
										</Typography>
									)}
								</Box>
								<Box component='dd' sx={{ m: 0, textAlign: 'right' }}>
									<Typography variant='body2' sx={{ color: 'text.strong' }}>
										{formatQuantity(quantity, itemUnit(item))}
									</Typography>
								</Box>
							</Fragment>
						))}
					</Box>
				</Box>
			))}
		</Stack>
	);
}
