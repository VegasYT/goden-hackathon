import { Chip } from '@mui/material';

import { Icon } from '@shared/ui/icon';

/** Пометка блюда, которое сейчас нельзя приготовить; `label` уточняет причину. */
export function StopListChip({ label = 'стоп-лист' }: { label?: string }) {
	return (
		<Chip
			color='error'
			icon={<Icon name='stopList' size='sm' />}
			label={label}
		/>
	);
}
