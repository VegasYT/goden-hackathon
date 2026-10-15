import { Box, Snackbar } from '@mui/material';
import { useState, type ReactNode } from 'react';

interface Notice {
	/** Новый номер у каждого сообщения, чтобы срок показа шёл заново. */
	id: number;
	open: boolean;
	text: string;
	action?: ReactNode;
}

/** Сообщение после записи: `show(text, action?)` показывает его, `element` страница ставит у себя; `top` — сверху окна, когда внизу панель итога. */
export function useNotice({ top = false }: { top?: boolean } = {}) {
	const [notice, setNotice] = useState<Notice>({
		id: 0,
		open: false,
		text: '',
	});
	const close = () => setNotice((current) => ({ ...current, open: false }));

	const element = (
		<Snackbar
			key={notice.id}
			open={notice.open}
			autoHideDuration={notice.action ? 8000 : 6000}
			anchorOrigin={top ? { vertical: 'top', horizontal: 'center' } : undefined}
			// Щелчок мимо не закрывает сообщение с действием, чтобы его успели нажать.
			onClose={(_, reason) => {
				if (reason !== 'clickaway' || !notice.action) {
					close();
				}
			}}
			message={notice.text}
			action={
				notice.action && (
					<Box component='span' sx={{ display: 'contents' }} onClick={close}>
						{notice.action}
					</Box>
				)
			}
		/>
	);

	return {
		show: (text: string, action?: ReactNode) =>
			setNotice((current) => ({
				id: current.id + 1,
				open: true,
				text,
				action,
			})),
		close,
		element,
	};
}
