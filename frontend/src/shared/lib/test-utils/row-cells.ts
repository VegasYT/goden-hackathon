import { within } from '@testing-library/react';

/** Тексты строки таблицы по колонкам (заголовок строки, затем ячейки); пробелы, и неразрывные тоже, обычные. */
export function rowCells(row: HTMLElement) {
	const scope = within(row);

	return [
		...scope.queryAllByRole('rowheader'),
		...scope.queryAllByRole('cell'),
	].map((cell) => cell.textContent.replace(/\s+/g, ' ').trim());
}
