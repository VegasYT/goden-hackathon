import { Box, IconButton, Stack, Typography } from '@mui/material';
import { useState } from 'react';

import { ProductPicker } from '@entities/product';
import { baseUnitLabel } from '@shared/lib/quantity';
import { Icon } from '@shared/ui/icon';
import { QuantityField } from '@shared/ui/quantity-field';
import { coarsePointer } from '@shared/ui/theme';

import { useCatalogScope } from '../model/catalog-scope';
import { rowOf, type RecipeRow } from '../model/recipe';
import { ProductDialog } from './product-dialog';

interface RecipeEditorProps {
	rows: RecipeRow[];
	onChange: (rows: RecipeRow[]) => void;
	/** Ошибки количеств по продукту; показываются после попытки сохранить. */
	problems: Record<string, string>;
}

/** Состав блюда на порцию: продукты с количеством в их единице и поиск нового продукта. */
export function RecipeEditor({ rows, onChange, problems }: RecipeEditorProps) {
	const { companyId } = useCatalogScope();
	// Название из поиска для диалога нового продукта; `null` — диалог закрыт.
	const [creating, setCreating] = useState<string | null>(null);
	const inRecipe = new Set(rows.map((row) => row.productId));
	const update = (productId: string, patch: Partial<RecipeRow>) =>
		onChange(
			rows.map((row) =>
				row.productId === productId ? { ...row, ...patch } : row
			)
		);

	return (
		<Stack sx={{ gap: 1 }}>
			{rows.length > 0 && (
				<Stack
					component='ul'
					aria-label='Продукты рецептуры'
					sx={{ listStyle: 'none', m: 0, p: 0 }}
				>
					{rows.map((row) => (
						<Box
							component='li'
							key={row.productId}
							sx={{
								display: 'flex',
								alignItems: 'center',
								gap: 1,
								py: 1,
								borderBottom: '1px solid',
								borderColor: 'divider',
							}}
						>
							<Box sx={{ flex: 1, minWidth: 0 }}>
								<Typography sx={{ fontWeight: 500, color: 'text.strong' }}>
									{row.name}
								</Typography>
								{row.archived && (
									<Typography variant='caption' sx={{ color: 'warning.ink' }}>
										в архиве
									</Typography>
								)}
							</Box>
							<Stack
								direction='row'
								sx={{
									width: 160,
									flexShrink: 0,
									gap: 1,
									alignItems: 'flex-start',
								}}
							>
								<QuantityField
									label='Количество'
									accessibleLabel={`Количество: ${row.name}`}
									hideLabel
									fullWidth
									unit={baseUnitLabel(row.unit)}
									value={row.quantity}
									onValueChange={(quantity) =>
										update(row.productId, { quantity })
									}
									min={1}
									step={1}
									format={{ maximumFractionDigits: 0 }}
									error={Boolean(problems[row.productId])}
									helperText={problems[row.productId]}
								/>
								<Typography
									aria-hidden
									// Высота строки как у поля: единица стоит вровень с числом и при ошибке под полем.
									sx={{
										width: 24,
										lineHeight: '32px',
										color: 'text.secondary',
										[coarsePointer]: { lineHeight: '44px' },
									}}
								>
									{baseUnitLabel(row.unit)}
								</Typography>
							</Stack>
							<IconButton
								aria-label={`Убрать из рецептуры: ${row.name}`}
								onClick={() =>
									onChange(
										rows.filter(({ productId }) => productId !== row.productId)
									)
								}
							>
								<Icon name='close' size='sm' />
							</IconButton>
						</Box>
					))}
				</Stack>
			)}
			<ProductPicker
				companyId={companyId}
				value={null}
				clearOnSelect
				onChange={(product) => product && onChange([...rows, rowOf(product)])}
				disabledReason={(product) =>
					inRecipe.has(product.id) ? 'уже в рецептуре' : undefined
				}
				onCreate={setCreating}
				ariaLabel='Добавить продукт'
				placeholder='Добавить продукт'
				unitLabel={baseUnitLabel}
			/>
			{creating !== null && (
				<ProductDialog
					initialName={creating}
					onClose={() => setCreating(null)}
					onSaved={(product) => {
						setCreating(null);
						onChange([...rows, rowOf(product)]);
					}}
				/>
			)}
		</Stack>
	);
}
