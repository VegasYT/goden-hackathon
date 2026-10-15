import {
	Paper,
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableRow,
} from '@mui/material';

import { itemUnit, type Delivery } from '@entities/delivery';
import { formatQuantity } from '@shared/lib/quantity';
import { ExpiryLabel } from '@shared/ui/expiry-label';

import { ItemTitle } from './item-blocks';

/** Позиции ожидающей поставки без полей ввода: у пользователя нет права приёмки или точка отключена. */
export function PendingItems({
	delivery,
	now,
}: {
	delivery: Delivery;
	now: Date;
}) {
	return (
		<Paper sx={{ overflow: 'hidden' }}>
			<Table aria-label='Позиции поставки'>
				<TableHead>
					<TableRow>
						<TableCell>Продукт</TableCell>
						<TableCell align='right'>Заявлено</TableCell>
						<TableCell>Годен до</TableCell>
					</TableRow>
				</TableHead>
				<TableBody>
					{delivery.items.map((item) => (
						<TableRow key={item.id}>
							<TableCell>
								<ItemTitle item={item} />
							</TableCell>
							<TableCell align='right'>
								{formatQuantity(item.declared_quantity, itemUnit(item))}
							</TableCell>
							<TableCell>
								<ExpiryLabel expiresAt={item.expires_at} now={now} />
							</TableCell>
						</TableRow>
					))}
				</TableBody>
			</Table>
		</Paper>
	);
}
