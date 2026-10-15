import { Stack, Typography } from '@mui/material';

import { itemName, itemUnit, type DeliveryItem } from '@entities/delivery';
import { formatQuantity } from '@shared/lib/quantity';
import { Icon } from '@shared/ui/icon';

import type { Part } from '../model/decision';
import type { DistributionChange } from '../model/history';
import { formatPartValue } from './parts';

const changeLabels: Record<Part, string> = {
	accepted: 'принято',
	rejected: 'отклонено',
	quarantined: 'в карантине',
};

const stockTexts = {
	done: { in: 'новая партия', out: 'со склада снято' },
	planned: {
		in: 'На склад поступит новая партия',
		out: 'Со склада будет снято',
	},
};

interface ChangeLinesProps {
	item: DeliveryItem;
	change: DistributionChange;
	/** `done` — как в истории («со склада снято»), `planned` — как в подтверждении («будет снято»). */
	tense: 'done' | 'planned';
}

/** Изменение позиции при исправлении: прежние и новые количества и что меняется на складе. */
export function ChangeLines({ item, change, tense }: ChangeLinesProps) {
	const unit = itemUnit(item);
	const stock = formatQuantity(Math.abs(change.stock), unit);

	return (
		<Stack sx={{ gap: 0.25 }}>
			<Typography sx={{ fontWeight: 500, color: 'text.strong' }}>
				{itemName(item)}
			</Typography>
			<Typography variant='body2'>
				{change.parts
					.map(
						({ part, before, after }) =>
							`${changeLabels[part]} ${formatPartValue(before, unit)} → ${formatQuantity(after, unit)}`
					)
					.join(', ')}
			</Typography>
			{change.stock !== 0 && (
				<Typography
					variant='body2'
					sx={{
						display: 'flex',
						gap: 0.75,
						alignItems: 'center',
						color: 'text.secondary',
					}}
				>
					<Icon name={change.stock > 0 ? 'stockIn' : 'stockOut'} size='sm' />
					{stockTexts[tense][change.stock > 0 ? 'in' : 'out']} {stock}
				</Typography>
			)}
		</Stack>
	);
}
