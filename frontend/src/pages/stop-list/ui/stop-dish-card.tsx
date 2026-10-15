import {
	Box,
	Paper,
	Stack,
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableRow,
	Typography,
} from '@mui/material';
import { useId, type ReactNode } from 'react';

import {
	ingredientQuantity,
	shortageOf,
	StopListChip,
	stopReasonLabel,
	type DishAvailability,
	type MenuIngredient,
} from '@entities/dish';
import { ProductHistoryLink } from '@entities/product';
import { Icon } from '@shared/ui/icon';
import { TextLink } from '@shared/ui/link';

interface Column {
	label: string;
	align: 'left' | 'right';
	width?: number;
	value: (ingredient: MenuIngredient) => ReactNode;
}

const strongError = { fontWeight: 600, color: 'error.ink' } as const;

// Одни и те же значения показывают таблица на ноутбуке и сетка на планшете.
const columns: Column[] = [
	{
		label: 'На порцию',
		align: 'right',
		width: 150,
		value: (ingredient) =>
			ingredientQuantity(ingredient, ingredient.required_quantity),
	},
	{
		label: 'Доступно',
		align: 'right',
		width: 150,
		value: (ingredient) =>
			ingredientQuantity(ingredient, ingredient.available_quantity),
	},
	{
		label: 'Не хватает',
		align: 'right',
		width: 150,
		value: (ingredient) => (
			<Box component='span' sx={strongError}>
				{ingredientQuantity(ingredient, shortageOf(ingredient))}
			</Box>
		),
	},
	{
		label: 'Просрочено',
		align: 'left',
		value: (ingredient) =>
			ingredient.expired_quantity > 0 ? (
				<>
					<Box component='span' sx={strongError}>
						{ingredientQuantity(ingredient, ingredient.expired_quantity)}
					</Box>
					<Typography
						variant='caption'
						component='div'
						sx={{ color: 'text.secondary' }}
					>
						в просроченных партиях, готовить из них нельзя
					</Typography>
				</>
			) : (
				<Box component='span' sx={{ color: 'text.secondary' }}>
					—
				</Box>
			),
	},
];

interface ShortageProps {
	dish: DishAvailability;
	productName: (ingredient: MenuIngredient) => ReactNode;
}

function ShortageTable({ dish, productName }: ShortageProps) {
	return (
		<Table
			size='small'
			aria-label={`Чего не хватает на порцию: ${dish.dish.name}`}
			sx={{ tableLayout: 'fixed' }}
		>
			<TableHead>
				<TableRow>
					<TableCell sx={{ pl: 2.5 }}>Продукт</TableCell>
					{columns.map((column) => (
						<TableCell
							key={column.label}
							align={column.align}
							sx={{
								width: column.width ?? 260,
								pl: column.align === 'left' ? 3 : 2,
							}}
						>
							{column.label}
						</TableCell>
					))}
				</TableRow>
			</TableHead>
			<TableBody>
				{dish.limiting.map((ingredient) => (
					<TableRow key={ingredient.product.id}>
						<TableCell sx={{ pl: 2.5, fontWeight: 500, color: 'text.strong' }}>
							{productName(ingredient)}
						</TableCell>
						{columns.map((column) => (
							<TableCell
								key={column.label}
								align={column.align}
								sx={{ pl: column.align === 'left' ? 3 : 2 }}
							>
								{column.value(ingredient)}
							</TableCell>
						))}
					</TableRow>
				))}
			</TableBody>
		</Table>
	);
}

function ShortageList({ dish, productName }: ShortageProps) {
	return (
		<Stack sx={{ px: 2.5, pb: 2, gap: 1.5 }}>
			{dish.limiting.map((ingredient) => (
				<Box
					key={ingredient.product.id}
					sx={{ borderTop: '1px solid', borderColor: 'divider', pt: 1.5 }}
				>
					<Typography sx={{ fontWeight: 500, color: 'text.strong' }}>
						{productName(ingredient)}
					</Typography>
					<Box
						component='dl'
						sx={{
							m: 0,
							mt: 1,
							display: 'grid',
							gridTemplateColumns: 'repeat(4, minmax(0, 1fr))',
							gap: 1.5,
						}}
					>
						{columns.map((column) => (
							<Box key={column.label}>
								<Typography
									component='dt'
									variant='body2'
									sx={{ color: 'text.secondary' }}
								>
									{column.label}
								</Typography>
								<Box component='dd' sx={{ m: 0, mt: 0.25 }}>
									{column.value(ingredient)}
								</Box>
							</Box>
						))}
					</Box>
				</Box>
			))}
		</Stack>
	);
}

interface StopDishCardProps {
	pointId: string;
	dish: DishAvailability;
	/** Можно предложить выключить блюдо без рецептуры на «Кухне». */
	mayManage: boolean;
	/** Название продукта ведёт в его историю на складе. */
	mayViewStock: boolean;
	/** Таблица на ноутбуке, список на планшете. */
	wide: boolean;
}

/** Блюдо стоп-листа: причина и по каждому недостающему продукту нужно, доступно, не хватает, просрочено. */
export function StopDishCard({
	pointId,
	dish,
	mayManage,
	mayViewStock,
	wide,
}: StopDishCardProps) {
	const titleId = useId();
	const productName = ({ product }: MenuIngredient) => (
		<ProductHistoryLink
			pointId={pointId}
			product={product}
			linked={mayViewStock}
		/>
	);

	return (
		<Paper
			component='article'
			aria-labelledby={titleId}
			sx={{ overflow: 'hidden' }}
		>
			<Stack
				direction='row'
				sx={{
					alignItems: 'center',
					gap: 1.5,
					flexWrap: 'wrap',
					px: 2.5,
					pt: 1.75,
					pb: 1.5,
				}}
			>
				<Typography id={titleId} variant='h3'>
					{dish.dish.name}
				</Typography>
				<StopListChip label={stopReasonLabel(dish)} />
			</Stack>
			{dish.reason === 'no_recipe' ? (
				<Stack
					direction='row'
					sx={{
						gap: 1.25,
						px: 2.5,
						pt: 1.5,
						pb: 2,
						borderTop: '1px solid',
						borderColor: 'divider',
					}}
				>
					<Icon name='recipe' sx={{ color: 'text.secondary' }} />
					<Typography>
						У блюда не задана рецептура, поэтому его нельзя приготовить.
						Рецептуры ведёт владелец компании в каталоге.
						{mayManage && (
							<>
								{' '}
								Если блюдо не продаётся, его можно выключить на странице{' '}
								<TextLink to='/points/$pointId/kitchen' params={{ pointId }}>
									«Кухня»
								</TextLink>
								.
							</>
						)}
					</Typography>
				</Stack>
			) : wide ? (
				<ShortageTable dish={dish} productName={productName} />
			) : (
				<ShortageList dish={dish} productName={productName} />
			)}
		</Paper>
	);
}
