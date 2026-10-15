import { Chip } from '@mui/material';

import { Icon } from '@shared/ui/icon';

/** Отметка горячего предложения, которое управляющий закрепил вручную. */
export function PinnedChip() {
	return (
		<Chip
			color='primary'
			icon={<Icon name='pin' size='sm' filled />}
			label='закреплено'
		/>
	);
}
