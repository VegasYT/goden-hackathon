import { Button, Link, Paper, Stack, Typography } from '@mui/material';
import type { ComponentProps, ReactNode } from 'react';

import { ActionMenu } from '@shared/ui/action-menu';
import { QueryState, type LoadQuery } from '@shared/ui/empty-state';
import { Icon } from '@shared/ui/icon';
import { ListPagination } from '@shared/ui/list-pagination';
import { PanelHeader } from '@shared/ui/panel-header';
import { SkeletonList } from '@shared/ui/skeleton-list';
import { useWideLayout } from '@shared/ui/theme';

/** Название в списке: открывает изменение, а без `onOpen` — просто текст. */
export function ItemName({
	name,
	onOpen,
}: {
	name: string;
	onOpen?: () => void;
}) {
	const sx = { fontWeight: 500, color: 'text.strong', textAlign: 'left' };

	if (!onOpen) {
		return <Typography sx={sx}>{name}</Typography>;
	}

	return (
		<Link
			component='button'
			type='button'
			underline='hover'
			onClick={onOpen}
			sx={sx}
		>
			{name}
		</Link>
	);
}

interface CatalogItem {
	id: string;
	name: string;
	archived: boolean;
}

/** Действия со строками списка; без права записи действий нет. */
export interface ItemActions<T> {
	readOnly: boolean;
	/** Строка, которая сейчас восстанавливается. */
	restoringId: string | undefined;
	onEdit: (item: T) => void;
	onArchive: (item: T) => void;
	onRestore: (item: T) => void;
}

/** Действия строки: у действующего — меню «Изменить» и «Отправить в архив», у архивного — «Восстановить». */
export function RowActions<T extends CatalogItem>({
	item,
	readOnly,
	restoringId,
	onEdit,
	onArchive,
	onRestore,
}: ItemActions<T> & { item: T }) {
	if (readOnly) {
		return null;
	}

	if (item.archived) {
		return (
			<Button
				variant='outlined'
				startIcon={<Icon name='unarchive' />}
				loading={restoringId === item.id}
				onClick={() => onRestore(item)}
				aria-label={`Восстановить: ${item.name}`}
			>
				Восстановить
			</Button>
		);
	}

	return (
		<ActionMenu
			label={`Действия: ${item.name}`}
			items={[
				{ icon: 'edit', label: 'Изменить', onClick: () => onEdit(item) },
				{
					icon: 'archive',
					label: 'Отправить в архив',
					onClick: () => onArchive(item),
				},
			]}
		/>
	);
}

interface CatalogListBodyProps<T> {
	query: LoadQuery & { data: { items: T[]; total: number } | undefined };
	/** Подпись заготовки: «Загружаем продукты». */
	loadingLabel: string;
	errorTitle: string;
	/** Что показать, когда на странице нет строк. */
	empty: ReactNode;
	table: (items: T[]) => ReactNode;
	cards: (items: T[]) => ReactNode;
	pagination: Omit<ComponentProps<typeof ListPagination>, 'total'>;
}

/** Содержимое списка вкладки: заготовка, ошибка, пустой список или таблица (на планшете карточки) с пагинацией. */
export function CatalogListBody<T>({
	query,
	loadingLabel,
	errorTitle,
	empty,
	table,
	cards,
	pagination,
}: CatalogListBodyProps<T>) {
	const wide = useWideLayout();

	return (
		<QueryState
			query={query}
			loading={
				<SkeletonList
					label={loadingLabel}
					count={6}
					height={40}
					sx={{ p: 2 }}
				/>
			}
			errorTitle={errorTitle}
		>
			{(data) =>
				data.items.length === 0 ? (
					empty
				) : (
					<>
						{wide ? table(data.items) : cards(data.items)}
						<ListPagination total={data.total} {...pagination} />
					</>
				)
			}
		</QueryState>
	);
}

/** Список вкладки: фильтры в шапке и содержимое под ними. */
export function CatalogListPanel({
	filters,
	children,
}: {
	filters: ReactNode;
	children: ReactNode;
}) {
	return (
		<Paper sx={{ overflow: 'hidden' }}>
			<PanelHeader>
				<Stack
					direction='row'
					sx={{
						gap: 1.5,
						alignItems: 'center',
						flexWrap: 'wrap',
						width: '100%',
					}}
				>
					{filters}
				</Stack>
			</PanelHeader>
			{children}
		</Paper>
	);
}
