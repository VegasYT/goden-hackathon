import { Alert, Button, Skeleton, Stack, Typography } from '@mui/material';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { dishCatalogKeys, dishQueryOptions, type Dish } from '@entities/dish';
import { ActionBar } from '@shared/ui/action-bar';
import { ConfirmDialog } from '@shared/ui/confirm-dialog';
import { QueryState } from '@shared/ui/empty-state';
import { Icon } from '@shared/ui/icon';
import { InfoNote } from '@shared/ui/info-note';
import { RenameForm } from '@shared/ui/rename-form';
import { SidePanel } from '@shared/ui/side-panel';

import { useChangeRecipe, useRenameDish } from '../api/catalog-writes';
import { useCatalogScope } from '../model/catalog-scope';
import {
	quantityProblems,
	recipeLine,
	recipeRows,
	sameRecipe,
	toIngredients,
	type RecipeRow,
} from '../model/recipe';
import type { ItemActions } from './catalog-list';
import { RecipeEditor } from './recipe-editor';

type DishActions = Pick<
	ItemActions<Dish>,
	'onArchive' | 'onRestore' | 'restoringId'
>;

interface DishEditorProps extends DishActions {
	dish: Dish;
	/** Черновик рецептуры; `null` — рецептура не менялась. */
	draft: RecipeRow[] | null;
	dirty: boolean;
	setDraft: (draft: RecipeRow[] | null) => void;
}

function DishEditor({
	dish,
	draft,
	dirty,
	setDraft,
	onArchive,
	onRestore,
	restoringId,
}: DishEditorProps) {
	const { readOnly } = useCatalogScope();
	const rename = useRenameDish(dish.id);
	const save = useChangeRecipe(dish.id);
	const [submitted, setSubmitted] = useState(false);
	const rows = draft ?? recipeRows(dish);
	const problems = submitted ? quantityProblems(rows) : {};
	const empty = rows.length === 0;

	const discard = () => {
		setDraft(null);
		setSubmitted(false);
		save.reset();
	};

	const submit = () => {
		setSubmitted(true);

		if (empty || Object.keys(quantityProblems(rows)).length > 0) {
			return;
		}

		save.mutate(toIngredients(rows), { onSuccess: discard });
	};

	if (readOnly) {
		return (
			<Stack sx={{ p: 3, gap: 1 }}>
				<Typography variant='h3' component='h3'>
					Рецептура на одну порцию
				</Typography>
				<Typography sx={{ color: 'text.secondary' }}>
					{dish.ingredients.length > 0 ? recipeLine(dish) : 'нет рецептуры'}
				</Typography>
			</Stack>
		);
	}

	return (
		<Stack sx={{ minHeight: '100%' }}>
			<Stack sx={{ p: 3, gap: 3 }}>
				<RenameForm
					label='Название блюда'
					value={dish.name}
					pending={rename.isPending}
					error={rename.error?.message}
					onSave={(name) => rename.mutate(name)}
				/>
				<Stack sx={{ gap: 1 }}>
					<Stack
						direction='row'
						sx={{ justifyContent: 'space-between', alignItems: 'baseline' }}
					>
						<Typography variant='h3' component='h3'>
							Рецептура на одну порцию
						</Typography>
						<Typography variant='caption' sx={{ color: 'text.secondary' }}>
							в единице продукта
						</Typography>
					</Stack>
					<RecipeEditor
						rows={rows}
						onChange={(next) => {
							setDraft(next);
							save.reset();
						}}
						problems={problems}
					/>
				</Stack>
				{empty ? (
					<Alert severity='warning'>
						Рецептура не может быть пустой. Если блюдо больше не нужно,
						отправьте его в архив.
					</Alert>
				) : (
					<InfoNote>
						Рецептура общая для всех точек компании. На точке, где продукта не
						хватает на порцию, блюдо попадёт в стоп-лист.
					</InfoNote>
				)}
				{save.error && <Alert severity='error'>{save.error.message}</Alert>}
			</Stack>
			<ActionBar
				label='Действия с блюдом'
				action={
					<>
						<Button
							variant='outlined'
							disabled={!dirty || save.isPending}
							onClick={discard}
						>
							Отменить
						</Button>
						<Button
							variant='contained'
							disabled={!dirty || empty}
							loading={save.isPending}
							onClick={submit}
						>
							Сохранить рецептуру
						</Button>
					</>
				}
			>
				{dish.archived ? (
					<Button
						startIcon={<Icon name='unarchive' />}
						loading={restoringId === dish.id}
						onClick={() => onRestore(dish)}
					>
						Восстановить
					</Button>
				) : (
					<Button
						startIcon={<Icon name='archive' />}
						onClick={() => onArchive(dish)}
						sx={{ color: 'text.secondary' }}
					>
						Отправить в архив
					</Button>
				)}
			</ActionBar>
		</Stack>
	);
}

interface DishPanelProps extends DishActions {
	dishId: string | undefined;
	onClose: () => void;
}

/** Боковая панель блюда: название и рецептура; несохранённые изменения подтверждаются при закрытии. */
export function DishPanel({ dishId, onClose, ...actions }: DishPanelProps) {
	const { companyId } = useCatalogScope();
	const queryClient = useQueryClient();
	const dish = useQuery({
		...dishQueryOptions(companyId, dishId ?? ''),
		enabled: dishId !== undefined,
		// Пока блюдо читается, панель показывает его из загруженного списка.
		placeholderData: () =>
			queryClient
				.getQueriesData<{ items: Dish[] }>({
					queryKey: dishCatalogKeys.all(companyId),
				})
				.flatMap(([, page]) => page?.items ?? [])
				.find(({ id }) => id === dishId),
	});
	const [draft, setDraft] = useState<{
		dishId: string;
		rows: RecipeRow[];
	} | null>(null);
	const [confirmClose, setConfirmClose] = useState(false);
	// Черновик принадлежит открытому блюду: у другого блюда он не показывается.
	const rows = draft && draft.dishId === dishId ? draft.rows : null;
	const dirty =
		dish.data !== undefined && rows !== null && !sameRecipe(rows, dish.data);

	const close = () => {
		setDraft(null);
		setConfirmClose(false);
		onClose();
	};

	return (
		<>
			<SidePanel
				open={dishId !== undefined}
				title={dish.data?.name ?? 'Блюдо'}
				closeLabel='Закрыть блюдо'
				width={540}
				onClose={() => (dirty ? setConfirmClose(true) : close())}
			>
				<QueryState
					query={dish}
					loading={
						<Stack
							sx={{ p: 3, gap: 2 }}
							role='status'
							aria-label='Загружаем блюдо'
						>
							<Skeleton variant='rounded' height={40} />
							<Skeleton variant='rounded' height={160} />
						</Stack>
					}
					errorTitle='Не удалось загрузить блюдо'
				>
					{(data) => (
						<DishEditor
							dish={data}
							draft={rows}
							dirty={dirty}
							setDraft={(next) =>
								setDraft(next && dishId ? { dishId, rows: next } : null)
							}
							{...actions}
						/>
					)}
				</QueryState>
			</SidePanel>
			<ConfirmDialog
				open={confirmClose}
				title='Отбросить изменения рецептуры?'
				description='Изменения рецептуры не сохранены.'
				confirmLabel='Отбросить'
				cancelLabel='Продолжить правку'
				confirmColor='error'
				onConfirm={close}
				onClose={() => setConfirmClose(false)}
			/>
		</>
	);
}
