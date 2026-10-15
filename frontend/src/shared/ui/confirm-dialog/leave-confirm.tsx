import { useBlocker } from '@tanstack/react-router';

import { ConfirmDialog } from './confirm-dialog';

interface LeaveConfirmOptions {
	/** Есть ли что терять: форма изменена и ещё не сохранена. */
	shouldBlock: () => boolean;
	title: string;
	description: string;
}

/** Вопрос при уходе со страницы с несохранённой формой, а также при закрытии вкладки; возвращает диалог, который страница ставит у себя. */
export function useLeaveConfirm({
	shouldBlock,
	title,
	description,
}: LeaveConfirmOptions) {
	const blocker = useBlocker({
		shouldBlockFn: shouldBlock,
		enableBeforeUnload: shouldBlock,
		withResolver: true,
	});

	return (
		<ConfirmDialog
			open={blocker.status === 'blocked'}
			title={title}
			description={description}
			confirmLabel='Уйти'
			cancelLabel='Остаться'
			confirmColor='error'
			onConfirm={() => blocker.proceed?.()}
			onClose={() => blocker.reset?.()}
		/>
	);
}
