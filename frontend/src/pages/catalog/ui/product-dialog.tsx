import {
	Autocomplete,
	Box,
	Checkbox,
	FormControlLabel,
	FormHelperText,
	Stack,
	TextField,
	Typography,
} from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { useId, useState } from 'react';

import {
	productCategoriesQueryOptions,
	productQueryOptions,
	type Product,
} from '@entities/product';
import { ApiError } from '@shared/api';
import type { QuantityUnit } from '@shared/lib/quantity';
import { FormDialog } from '@shared/ui/form-dialog';
import { Icon } from '@shared/ui/icon';
import { InfoNote } from '@shared/ui/info-note';
import { TextLink } from '@shared/ui/link';
import { SegmentedControl } from '@shared/ui/segmented-control';

import { useSaveProduct } from '../api/catalog-writes';
import { useCatalogScope } from '../model/catalog-scope';
import {
	productDraft,
	productProblems,
	toNewProduct,
	toProductChange,
	type ProductDraft,
} from '../model/product-form';

const unitOptions: { value: QuantityUnit; label: string }[] = [
	{ value: 'g', label: 'граммы' },
	{ value: 'ml', label: 'миллилитры' },
	{ value: 'pcs', label: 'штуки' },
];

function CategoryField({
	value,
	error,
	onChange,
}: {
	value: string;
	error?: string;
	onChange: (category: string) => void;
}) {
	const { companyId } = useCatalogScope();
	const categories = useQuery(productCategoriesQueryOptions(companyId));
	const items = categories.data ?? [];
	const names = items.map(({ name }) => name);
	const typed = value.trim();

	return (
		<Autocomplete
			freeSolo
			options={names}
			inputValue={value}
			onInputChange={(_, next) => onChange(next)}
			filterOptions={(options) => {
				const matches = options.filter((name) =>
					name.toLowerCase().includes(typed.toLowerCase())
				);

				return typed && !names.includes(typed) ? [...matches, typed] : matches;
			}}
			renderOption={({ key, ...props }, option) => {
				const known = items.find(({ name }) => name === option);

				return (
					<Box
						component='li'
						key={key}
						{...props}
						sx={{
							'&.MuiAutocomplete-option': { justifyContent: 'space-between' },
						}}
					>
						{known ? (
							<>
								<span>{option}</span>
								<Typography variant='caption' sx={{ color: 'text.tertiary' }}>
									{known.products}
								</Typography>
							</>
						) : (
							<Typography sx={{ color: 'primary.main', fontWeight: 600 }}>
								+ Новая категория «{option}»
							</Typography>
						)}
					</Box>
				);
			}}
			renderInput={(params) => (
				<TextField
					{...params}
					label='Категория'
					placeholder='Например, Молочные продукты'
					error={Boolean(error)}
					helperText={error}
				/>
			)}
		/>
	);
}

/** Пояснение к занятому названию: у архивного продукта — ссылка на архив с этим названием. */
function NameTakenByArchived({
	name,
	onOpenArchive,
}: {
	name: string;
	onOpenArchive: () => void;
}) {
	const { pointId } = useCatalogScope();

	return (
		<>
			Это название у продукта в архиве. Восстановите его вместо нового:{' '}
			<TextLink
				to='/points/$pointId/catalog'
				params={{ pointId }}
				search={{ archived: true, q: name }}
				onClick={onOpenArchive}
			>
				открыть архив
			</TextLink>
		</>
	);
}

interface ProductDialogProps {
	/** Изменяемый продукт; без него диалог создаёт новый. */
	product?: Product;
	/** Название нового продукта, например текст поиска. */
	initialName?: string;
	onClose: () => void;
	onSaved: (product: Product) => void;
}

/** Диалог нового продукта или изменения продукта каталога. */
export function ProductDialog({
	product,
	initialName = '',
	onClose,
	onSaved,
}: ProductDialogProps) {
	const { companyId } = useCatalogScope();
	const unitHintId = useId();
	const mutation = useSaveProduct(product?.id);
	// Блюда с продуктом есть только в ответе на чтение продукта по идентификатору.
	const detail = useQuery({
		...productQueryOptions(companyId, product?.id ?? ''),
		enabled: product !== undefined && (product.dishes ?? 0) > 0,
	});
	const [draft, setDraft] = useState<ProductDraft>(() =>
		productDraft(product, initialName)
	);
	const [submitted, setSubmitted] = useState(false);
	const problems = submitted ? productProblems(draft) : {};
	const serverErrors =
		mutation.error instanceof ApiError ? mutation.error.fieldErrors : {};
	const unitLocked = product?.unit_locked ?? false;
	const usedIn = detail.data?.used_in ?? [];

	const set = (patch: Partial<ProductDraft>) => {
		setDraft((current) => ({ ...current, ...patch }));
		mutation.reset();
	};

	const submit = () => {
		setSubmitted(true);

		if (Object.keys(productProblems(draft)).length > 0) {
			return;
		}

		mutation.mutate(
			product ? toProductChange(product, draft) : toNewProduct(draft),
			{ onSuccess: onSaved }
		);
	};

	const nameError =
		serverErrors.name?.code === 'taken_by_archived' ? (
			<NameTakenByArchived name={draft.name.trim()} onOpenArchive={onClose} />
		) : (
			(problems.name ?? serverErrors.name?.message)
		);
	const fieldError = Object.keys(serverErrors).length > 0;

	return (
		<FormDialog
			title={product ? 'Изменить продукт' : 'Новый продукт'}
			submitLabel={product ? 'Сохранить' : 'Создать продукт'}
			pending={mutation.isPending}
			error={mutation.error && !fieldError ? mutation.error.message : undefined}
			onSubmit={submit}
			onClose={onClose}
		>
			<TextField
				label='Название'
				value={draft.name}
				onChange={(event) => set({ name: event.target.value })}
				error={Boolean(nameError)}
				helperText={nameError}
				autoFocus={!initialName}
				slotProps={{ htmlInput: { maxLength: 200 } }}
			/>
			<Stack sx={{ gap: 0.75, alignItems: 'flex-start' }}>
				<Typography variant='body2' component='span' sx={{ fontWeight: 500 }}>
					Единица
				</Typography>
				<SegmentedControl
					label='Единица'
					options={unitOptions}
					value={draft.unit}
					onChange={(unit) => set({ unit })}
					disabled={unitLocked}
				/>
				<FormHelperText
					id={unitHintId}
					error={Boolean(serverErrors.unit)}
					sx={{ m: 0, display: 'flex', gap: 0.5, alignItems: 'center' }}
				>
					{unitLocked && <Icon name='lock' size='sm' />}
					{serverErrors.unit?.message ??
						(unitLocked
							? 'Единицу нельзя сменить: продукт уже есть в поставках и рецептурах'
							: 'Все количества продукта будут в этой единице. Продукт, который расходуется частями (пол-лимона), учитывайте в граммах.')}
				</FormHelperText>
			</Stack>
			<CategoryField
				value={draft.category}
				error={serverErrors.category?.message}
				onChange={(category) => set({ category })}
			/>
			<Stack direction='row' sx={{ gap: 2, alignItems: 'flex-start' }}>
				<TextField
					label='Срок хранения, дней'
					value={draft.noShelfLife ? '' : draft.shelfLifeDays}
					onChange={(event) =>
						set({ shelfLifeDays: event.target.value.replace(/\D/g, '') })
					}
					disabled={draft.noShelfLife}
					error={Boolean(
						problems.shelfLifeDays ?? serverErrors.shelf_life_days
					)}
					helperText={
						problems.shelfLifeDays ?? serverErrors.shelf_life_days?.message
					}
					slotProps={{ htmlInput: { inputMode: 'numeric', maxLength: 4 } }}
					sx={{ width: 180 }}
				/>
				<FormControlLabel
					label='Без срока'
					sx={{ mt: 3 }}
					control={
						<Checkbox
							checked={draft.noShelfLife}
							onChange={(event) => set({ noShelfLife: event.target.checked })}
						/>
					}
				/>
			</Stack>
			{usedIn.length > 0 && (
				<InfoNote>
					Есть в рецептурах: {usedIn.map(({ name }) => name).join(', ')}. Новое
					название сразу появится на складе, в поставках и меню.
				</InfoNote>
			)}
		</FormDialog>
	);
}
