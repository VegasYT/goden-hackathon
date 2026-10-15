import { Stack, TextField, Typography } from '@mui/material';
import { useState } from 'react';

import { ApiError } from '@shared/api';
import { FormDialog } from '@shared/ui/form-dialog';
import { InfoNote } from '@shared/ui/info-note';

import { useCreateDish } from '../api/catalog-writes';
import {
	quantityProblems,
	toIngredients,
	type RecipeRow,
} from '../model/recipe';
import { RecipeEditor } from './recipe-editor';

/** Новое блюдо сразу с рецептурой; без продуктов блюдо создаётся и попадает в стоп-лист. */
export function NewDishDialog({ onClose }: { onClose: () => void }) {
	const create = useCreateDish();
	const [name, setName] = useState('');
	const [rows, setRows] = useState<RecipeRow[]>([]);
	const [submitted, setSubmitted] = useState(false);
	const problems = submitted ? quantityProblems(rows) : {};
	const nameError =
		(submitted && !name.trim() ? 'Введите название' : undefined) ??
		(create.error instanceof ApiError
			? create.error.fieldErrors.name?.message
			: undefined);

	const submit = () => {
		setSubmitted(true);

		if (!name.trim() || Object.keys(quantityProblems(rows)).length > 0) {
			return;
		}

		create.mutate(
			{ name: name.trim(), ingredients: toIngredients(rows) },
			{ onSuccess: onClose }
		);
	};

	return (
		<FormDialog
			title='Новое блюдо'
			submitLabel='Создать блюдо'
			pending={create.isPending}
			error={
				create.error && !(create.error instanceof ApiError && nameError)
					? create.error.message
					: undefined
			}
			onSubmit={submit}
			onClose={onClose}
		>
			<TextField
				label='Название'
				value={name}
				onChange={(event) => {
					setName(event.target.value);
					create.reset();
				}}
				error={Boolean(nameError)}
				helperText={nameError}
				autoFocus
				slotProps={{ htmlInput: { maxLength: 200 } }}
			/>
			<Stack sx={{ gap: 0.5 }}>
				<Typography variant='h3' component='h3'>
					Рецептура на одну порцию
				</Typography>
				<RecipeEditor rows={rows} onChange={setRows} problems={problems} />
			</Stack>
			{rows.length === 0 && (
				<InfoNote>
					Без рецептуры блюдо сразу попадёт в стоп-лист всех точек. Её можно
					задать и позже.
				</InfoNote>
			)}
		</FormDialog>
	);
}
