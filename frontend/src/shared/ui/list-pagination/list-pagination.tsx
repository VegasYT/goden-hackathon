import { TablePagination } from '@mui/material';

import { pageSizes, type PageSize } from '@shared/lib/search-params';
import { useWideLayout } from '@shared/ui/theme';

import { lastPage } from './last-page';

interface ListPaginationProps {
	total: number;
	/** Страница с единицы, как в адресе. */
	page: number;
	size: number;
	onPageChange: (page: number) => void;
	/** Новое число строк; список при этом открывается с первой страницы. */
	onSizeChange: (size: PageSize) => void;
}

/** Пагинация списка: «1–10 из 48», выбор от 5 до 50 строк, первая и последняя страница. */
export function ListPagination({
	total,
	page,
	size,
	onPageChange,
	onSizeChange,
}: ListPaginationProps) {
	// На планшете над пагинацией карточки, поэтому ей нужна своя линия; у таблицы линия уже есть под строкой.
	const divided = !useWideLayout();

	return (
		<TablePagination
			component='div'
			count={total}
			page={Math.min(page, lastPage(total, size)) - 1}
			rowsPerPage={size}
			rowsPerPageOptions={pageSizes}
			onPageChange={(_, next) => onPageChange(next + 1)}
			onRowsPerPageChange={(event) =>
				onSizeChange(Number(event.target.value) as PageSize)
			}
			showFirstButton
			showLastButton
			sx={
				divided ? { borderTop: '1px solid', borderColor: 'divider' } : undefined
			}
		/>
	);
}
