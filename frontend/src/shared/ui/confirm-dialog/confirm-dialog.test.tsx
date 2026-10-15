import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { renderWithTheme } from '@shared/lib/test-utils';

import { ConfirmDialog, type ConfirmDialogProps } from './confirm-dialog';

const notice = {
	id: 'test-approval',
	text: 'После подтверждения приёмку нельзя изменить.',
};

function renderDialog(props: Partial<ConfirmDialogProps> = {}) {
	const onConfirm = vi.fn();
	const onClose = vi.fn();
	const view = renderWithTheme(
		<ConfirmDialog
			open
			title='Подтвердить приёмку поставки № 118?'
			confirmLabel='Подтвердить приёмку'
			onConfirm={onConfirm}
			onClose={onClose}
			{...props}
		/>
	);

	return { onConfirm, onClose, ...view };
}

describe('ConfirmDialog', () => {
	it('«Отмена» закрывает диалог без действия', async () => {
		const { onConfirm, onClose } = renderDialog();

		await userEvent.click(screen.getByRole('button', { name: 'Отмена' }));

		expect(onClose).toHaveBeenCalled();
		expect(onConfirm).not.toHaveBeenCalled();
	});

	it('диалог доступен как alertdialog с заголовком', () => {
		renderDialog();
		expect(
			screen.getByRole('alertdialog', {
				name: 'Подтвердить приёмку поставки № 118?',
			})
		).toBeInTheDocument();
	});

	it('скрытое предупреждение не показывается повторно, а подтверждение показывается', async () => {
		const first = renderDialog({ dismissibleNotice: notice });

		await userEvent.click(
			screen.getByRole('checkbox', {
				name: 'Больше не показывать это предупреждение',
			})
		);
		await userEvent.click(
			screen.getByRole('button', { name: 'Подтвердить приёмку' })
		);
		expect(first.onConfirm).toHaveBeenCalled();
		first.unmount();

		renderDialog({ dismissibleNotice: notice });

		expect(screen.getByRole('alertdialog')).toBeInTheDocument();
		expect(screen.queryByText(notice.text)).toBeNull();
	});

	it('без отметки предупреждение показывается снова', async () => {
		const first = renderDialog({ dismissibleNotice: notice });

		await userEvent.click(
			screen.getByRole('button', { name: 'Подтвердить приёмку' })
		);
		first.unmount();

		renderDialog({ dismissibleNotice: notice });

		expect(screen.getByText(notice.text)).toBeInTheDocument();
	});

	it('отмеченное предупреждение остаётся, пока подтверждение ждёт ответа и после ошибки', async () => {
		const { rerender, onConfirm, onClose } = renderDialog({
			dismissibleNotice: notice,
		});
		// Родитель собирает предупреждение заново при каждой перерисовке.
		const dialog = (props: Partial<ConfirmDialogProps>) => (
			<ConfirmDialog
				open
				title='Подтвердить приёмку поставки № 118?'
				confirmLabel='Подтвердить приёмку'
				onConfirm={onConfirm}
				onClose={onClose}
				dismissibleNotice={{ ...notice }}
				{...props}
			/>
		);

		await userEvent.click(
			screen.getByRole('checkbox', {
				name: 'Больше не показывать это предупреждение',
			})
		);
		await userEvent.click(
			screen.getByRole('button', { name: 'Подтвердить приёмку' })
		);
		rerender(dialog({ pending: true }));

		expect(onConfirm).toHaveBeenCalled();
		expect(screen.getByText(notice.text)).toBeInTheDocument();

		rerender(dialog({ error: 'Не удалось подтвердить приёмку' }));

		expect(screen.getByText(notice.text)).toBeInTheDocument();
		expect(
			screen.getByRole('checkbox', {
				name: 'Больше не показывать это предупреждение',
			})
		).toBeChecked();
	});

	it('во время отправки повторное нажатие не подтверждает действие', async () => {
		const { onConfirm, onClose } = renderDialog({ pending: true });
		const user = userEvent.setup({ pointerEventsCheck: 0 });

		await user.click(
			screen.getByRole('button', { name: /Подтвердить приёмку/ })
		);
		await user.keyboard('{Escape}');

		expect(onConfirm).not.toHaveBeenCalled();
		expect(onClose).not.toHaveBeenCalled();
	});
});
