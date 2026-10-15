import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableRow,
} from '@mui/material';

import type { Shortage } from '../api/orders';
import { shortageAmounts, shortageName } from '../model/order';

/** Недостающие продукты отклонённого заказа: сколько нужно, сколько было, сколько не хватило. */
export function ShortageTable({ shortages }: { shortages: Shortage[] }) {
	return (
		<Table
			size='small'
			aria-label='Недостающие продукты'
			sx={{ maxWidth: 560, '& td, & th': { px: 0 } }}
		>
			<TableHead>
				<TableRow>
					<TableCell>Продукт</TableCell>
					<TableCell align='right'>Нужно</TableCell>
					<TableCell align='right'>Было</TableCell>
					<TableCell align='right'>Не хватило</TableCell>
				</TableRow>
			</TableHead>
			<TableBody>
				{shortages.map((shortage) => {
					const amounts = shortageAmounts(shortage);

					return (
						<TableRow key={shortage.product_id}>
							<TableCell sx={{ fontWeight: 500, color: 'text.strong' }}>
								{shortageName(shortage)}
							</TableCell>
							<TableCell align='right'>{amounts.required}</TableCell>
							<TableCell align='right'>{amounts.available}</TableCell>
							<TableCell
								align='right'
								sx={{ fontWeight: 600, color: 'error.ink' }}
							>
								{amounts.missing}
							</TableCell>
						</TableRow>
					);
				})}
			</TableBody>
		</Table>
	);
}
