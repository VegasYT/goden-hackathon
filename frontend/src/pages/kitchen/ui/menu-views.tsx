import {
	Box,
	ButtonBase,
	Card,
	Stack,
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableRow,
	Typography,
} from '@mui/material';
import type { ReactNode } from 'react';

import {
	StopListChip,
	type DishAvailability,
	type DishRef,
	type PointDish,
} from '@entities/dish';
import { plural } from '@shared/lib/plural';
import { CardList, CardListItem } from '@shared/ui/card-list';
import { EmptyValue } from '@shared/ui/empty-value';
import { DisclosureIcon } from '@shared/ui/icon';
import { coarsePointer, radius } from '@shared/ui/theme';

import { DisableButton, EnableButton } from './dish-actions';
import { Limiting } from './limiting';
import { Recipe } from './recipe';

interface MenuViewProps {
	pointId: string;
	menu: DishAvailability[];
	disabled: PointDish[];
	expanded: ReadonlySet<string>;
	onToggle: (dishId: string) => void;
	mayManage: boolean;
	onDisable: (dish: DishRef) => void;
	onEnableError: (text: string | null) => void;
}

const recipeId = (dish: DishAvailability) => `recipe-${dish.dish.id}`;

const ingredientNames = (dish: DishAvailability) =>
	dish.ingredients.map(({ product }) => product.name).join(', ');

/** Название блюда с составом; блюдо с рецептурой раскрывается в неё нажатием. */
function DishToggle({
	dish,
	open,
	onToggle,
	caption,
	trailing,
}: {
	dish: DishAvailability;
	open: boolean;
	onToggle: () => void;
	/** Строка обрезается по ширине, разметка (причины стоп-листа) переносится. */
	caption: ReactNode;
	trailing?: ReactNode;
}) {
	const hasRecipe = dish.ingredients.length > 0;
	const content = (
		<>
			{hasRecipe ? (
				<DisclosureIcon open={open} sx={{ mt: 0.25 }} />
			) : (
				<Box sx={{ width: 20, flexShrink: 0 }} />
			)}
			<Box sx={{ flex: 1, minWidth: 0 }}>
				<Typography sx={{ fontWeight: 500, color: 'text.strong' }}>
					{dish.dish.name}
				</Typography>
				<Typography
					variant='caption'
					component='div'
					noWrap={typeof caption === 'string'}
					sx={{ color: 'text.secondary' }}
				>
					{hasRecipe ? caption : 'рецептура не задана'}
				</Typography>
			</Box>
			{trailing}
		</>
	);
	const layout = {
		display: 'flex',
		alignItems: 'flex-start',
		gap: 0.75,
		flex: 1,
		minWidth: 0,
		textAlign: 'left',
	} as const;

	if (!hasRecipe) {
		return <Box sx={layout}>{content}</Box>;
	}

	return (
		<ButtonBase
			aria-expanded={open}
			aria-controls={open ? recipeId(dish) : undefined}
			aria-label={`Рецептура: ${dish.dish.name}`}
			onClick={onToggle}
			sx={(theme) => ({
				...layout,
				borderRadius: radius(theme, 'sm'),
				[coarsePointer]: { minHeight: 44 },
			})}
		>
			{content}
		</ButtonBase>
	);
}

/** Число порций или пометка «стоп-лист»; `withUnit` добавляет слово «порций» под числом. */
function Portions({
	dish,
	withUnit = false,
}: {
	dish: DishAvailability;
	withUnit?: boolean;
}) {
	if (!dish.available) {
		return <StopListChip />;
	}

	return (
		<Box sx={{ textAlign: withUnit ? 'right' : undefined }}>
			<Typography
				sx={{
					fontWeight: 600,
					color: 'text.strong',
					...(withUnit && { fontSize: 20, lineHeight: 1.2 }),
				}}
			>
				{dish.portions}
			</Typography>
			{withUnit && (
				<Typography variant='caption' sx={{ color: 'text.secondary' }}>
					{plural(dish.portions, 'порция', 'порции', 'порций')}
				</Typography>
			)}
		</Box>
	);
}

function DisabledDishInfo({ dish }: { dish: PointDish }) {
	return (
		<>
			<Typography sx={{ fontWeight: 500, color: 'text.secondary' }}>
				{dish.name}
			</Typography>
			<Typography variant='caption' sx={{ color: 'text.secondary' }}>
				не продаётся на точке, официанты его не видят
			</Typography>
		</>
	);
}

const disabledTitle = (count: number) => `Выключены на точке · ${count}`;

/** Меню таблицей на широком экране, выключенные блюда — группой в конце. */
export function MenuTable({
	pointId,
	menu,
	disabled,
	expanded,
	onToggle,
	mayManage,
	onDisable,
	onEnableError,
}: MenuViewProps) {
	const columns = mayManage ? 4 : 3;

	return (
		<Table aria-label='Меню точки' sx={{ tableLayout: 'fixed' }}>
			<TableHead>
				<TableRow>
					<TableCell>Блюдо</TableCell>
					<TableCell sx={{ width: 128 }}>Порций</TableCell>
					<TableCell sx={{ width: 400 }}>Что ограничивает</TableCell>
					{mayManage && <TableCell sx={{ width: 132 }} />}
				</TableRow>
			</TableHead>
			<TableBody>
				{menu.map((dish) => {
					const open = expanded.has(dish.dish.id);

					return [
						<TableRow
							key={dish.dish.id}
							sx={{
								verticalAlign: 'top',
								bgcolor: open ? 'field' : undefined,
							}}
						>
							<TableCell>
								<DishToggle
									dish={dish}
									open={open}
									onToggle={() => onToggle(dish.dish.id)}
									caption={ingredientNames(dish)}
								/>
							</TableCell>
							<TableCell>
								<Portions dish={dish} />
							</TableCell>
							<TableCell>
								<Limiting dish={dish} />
							</TableCell>
							{mayManage && (
								<TableCell align='right'>
									<DisableButton dish={dish.dish} onDisable={onDisable} />
								</TableCell>
							)}
						</TableRow>,
						open && (
							<TableRow key={recipeId(dish)} sx={{ bgcolor: 'field' }}>
								<TableCell colSpan={columns} sx={{ pt: 0, pl: 5.5 }}>
									<Recipe dish={dish} id={recipeId(dish)} />
								</TableCell>
							</TableRow>
						),
					];
				})}
				{disabled.length > 0 && (
					<TableRow>
						<TableCell
							component='th'
							scope='rowgroup'
							colSpan={columns}
							sx={{ pt: 2, fontWeight: 500, color: 'text.secondary' }}
						>
							{disabledTitle(disabled.length)}
						</TableCell>
					</TableRow>
				)}
				{disabled.map((dish) => (
					<TableRow key={dish.dish_id}>
						<TableCell sx={{ pl: 5.5 }}>
							<DisabledDishInfo dish={dish} />
						</TableCell>
						<TableCell>
							<EmptyValue />
						</TableCell>
						<TableCell>
							<EmptyValue />
						</TableCell>
						{mayManage && (
							<TableCell align='right'>
								<EnableButton
									pointId={pointId}
									dish={dish}
									onError={onEnableError}
								/>
							</TableCell>
						)}
					</TableRow>
				))}
			</TableBody>
		</Table>
	);
}

/** Меню карточками на планшете; у доступного блюда под названием состав, у остальных — причина. */
export function MenuCards({
	pointId,
	menu,
	disabled,
	expanded,
	onToggle,
	mayManage,
	onDisable,
	onEnableError,
}: MenuViewProps) {
	return (
		<Stack sx={{ gap: 1.5, p: 2 }}>
			<CardList label='Меню точки' sx={{ p: 0 }}>
				{menu.map((dish) => {
					const open = expanded.has(dish.dish.id);

					return (
						<CardListItem
							key={dish.dish.id}
							label={dish.dish.name}
							sx={{
								pl: 1.5,
								display: 'grid',
								gridTemplateColumns: 'minmax(0, 1fr)',
							}}
						>
							<Stack direction='row' sx={{ gap: 1, alignItems: 'center' }}>
								<DishToggle
									dish={dish}
									open={open}
									onToggle={() => onToggle(dish.dish.id)}
									caption={
										dish.available ? (
											ingredientNames(dish)
										) : (
											<Limiting dish={dish} />
										)
									}
									trailing={<Portions dish={dish} withUnit />}
								/>
								{mayManage && (
									<DisableButton dish={dish.dish} onDisable={onDisable} />
								)}
							</Stack>
							{open && (
								<Box sx={{ pl: 3.5 }}>
									<Recipe dish={dish} id={recipeId(dish)} />
								</Box>
							)}
						</CardListItem>
					);
				})}
			</CardList>
			{disabled.length > 0 && (
				<Stack
					component='section'
					aria-label='Выключены на точке'
					sx={{ gap: 1 }}
				>
					<Typography
						component='h2'
						sx={{ fontWeight: 500, color: 'text.secondary' }}
					>
						{disabledTitle(disabled.length)}
					</Typography>
					{disabled.map((dish) => (
						<Card
							key={dish.dish_id}
							sx={{
								p: 1.5,
								display: 'flex',
								alignItems: 'center',
								gap: 1.5,
								bgcolor: 'field',
							}}
						>
							<Box sx={{ flex: 1, minWidth: 0 }}>
								<DisabledDishInfo dish={dish} />
							</Box>
							{mayManage && (
								<EnableButton
									pointId={pointId}
									dish={dish}
									onError={onEnableError}
								/>
							)}
						</Card>
					))}
				</Stack>
			)}
		</Stack>
	);
}
