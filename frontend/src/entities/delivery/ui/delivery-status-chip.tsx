import { Chip } from '@mui/material';

import type { Delivery } from '../api/delivery-queries';
import { statusLabel } from '../model/delivery';

/** Статус поставки: «ожидает приёмки» или «принята». */
export function DeliveryStatusChip({ status }: { status: Delivery['status'] }) {
	return (
		<Chip
			color={status === 'approved' ? 'success' : 'default'}
			label={statusLabel(status)}
		/>
	);
}
