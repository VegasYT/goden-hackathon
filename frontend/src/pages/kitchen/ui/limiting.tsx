import { Box, Stack, Typography } from '@mui/material';

import {
	ingredientQuantity,
	type DishAvailability,
	type MenuIngredient,
} from '@entities/dish';

/** Просроченный остаток продукта под его количеством; ничего, если просрочки нет. */
export function ExpiredNote({ ingredient }: { ingredient: MenuIngredient }) {
	if (ingredient.expired_quantity === 0) {
		return null;
	}

	return (
		<Typography variant='caption' component='div' sx={{ color: 'error.ink' }}>
			ещё {ingredientQuantity(ingredient, ingredient.expired_quantity)}{' '}
			просрочено
		</Typography>
	);
}

/** Что ограничивает порции блюда: продукт, остаток и количество на порцию, у стоп-листа — нехватка. */
export function Limiting({ dish }: { dish: DishAvailability }) {
	if (dish.reason === 'no_recipe') {
		return (
			<Typography variant='body2' sx={{ color: 'text.secondary' }}>
				Нет рецептуры: блюдо нельзя приготовить
			</Typography>
		);
	}

	return (
		<Stack sx={{ gap: 0.25 }}>
			{dish.limiting.map((ingredient, index) => (
				<Box key={ingredient.product.id}>
					<Typography variant='body2' sx={{ color: 'text.secondary' }}>
						{!dish.available && index === 0 && 'Не хватает: '}
						<Box component='span' sx={{ color: 'text.strong' }}>
							{ingredient.product.name}
						</Box>{' '}
						· есть{' '}
						{ingredientQuantity(ingredient, ingredient.available_quantity)}, на
						порцию{' '}
						{ingredientQuantity(ingredient, ingredient.required_quantity)}
					</Typography>
					{!dish.available && <ExpiredNote ingredient={ingredient} />}
				</Box>
			))}
		</Stack>
	);
}
