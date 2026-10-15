import {
	Box,
	Chip,
	Paper,
	Stack,
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableRow,
	Typography,
} from '@mui/material';

import {
	ingredientQuantity,
	portionsOf,
	type DishAvailability,
} from '@entities/dish';
import { countOf } from '@shared/lib/plural';

import { ExpiredNote } from './limiting';

const portionsAccusative = (count: number) =>
	countOf(count, 'порцию', 'порции', 'порций');

function orderNote(dish: DishAvailability): string {
	const base = 'Доступно — остаток без просроченных партий.';

	return dish.available
		? `${base} Заказ на ${portionsAccusative(dish.portions)} будет принят, на ${dish.portions + 1} — нет.`
		: `${base} Пока продуктов не хватает на порцию, блюдо в стоп-листе.`;
}

/** Рецептура блюда на одну порцию: по каждому продукту остаток и на сколько порций его хватит. */
export function Recipe({ dish, id }: { dish: DishAvailability; id: string }) {
	const limiting = new Set(dish.limiting.map(({ product }) => product.id));

	return (
		<Stack id={id} sx={{ gap: 1 }}>
			<Paper sx={{ overflow: 'hidden' }}>
				<Table
					size='small'
					aria-label={`Рецептура блюда ${dish.dish.name} на одну порцию`}
					sx={{ tableLayout: 'fixed' }}
				>
					<TableHead>
						<TableRow>
							<TableCell>Продукт</TableCell>
							<TableCell align='right' sx={{ width: '18%' }}>
								На порцию
							</TableCell>
							<TableCell align='right' sx={{ width: '26%' }}>
								Доступно
							</TableCell>
							<TableCell align='right' sx={{ width: '18%' }}>
								Хватит на
							</TableCell>
						</TableRow>
					</TableHead>
					<TableBody>
						{dish.ingredients.map((ingredient) => {
							const limits = limiting.has(ingredient.product.id);

							return (
								<TableRow key={ingredient.product.id}>
									<TableCell>
										<Stack
											direction='row'
											sx={{ gap: 1, alignItems: 'center', flexWrap: 'wrap' }}
										>
											<Box
												component='span'
												sx={{
													color: 'text.strong',
													fontWeight: limits ? 500 : 400,
												}}
											>
												{ingredient.product.name}
											</Box>
											{limits && <Chip label='ограничивает' />}
										</Stack>
									</TableCell>
									<TableCell align='right'>
										{ingredientQuantity(
											ingredient,
											ingredient.required_quantity
										)}
									</TableCell>
									<TableCell align='right'>
										{ingredientQuantity(
											ingredient,
											ingredient.available_quantity
										)}
										<ExpiredNote ingredient={ingredient} />
									</TableCell>
									<TableCell
										align='right'
										sx={
											limits
												? { fontWeight: 600, color: 'text.strong' }
												: { color: 'text.secondary' }
										}
									>
										{portionsAccusative(portionsOf(ingredient))}
									</TableCell>
								</TableRow>
							);
						})}
					</TableBody>
				</Table>
			</Paper>
			<Typography variant='body2' sx={{ color: 'text.secondary' }}>
				{orderNote(dish)}
			</Typography>
		</Stack>
	);
}
