/** Колбэки `mutate` восстановления из архива: сообщение об успехе или ошибке. */
export const restoreNotice = (
	name: string,
	notify: (text: string) => void
) => ({
	onSuccess: () => notify(`«${name}» снова в каталоге`),
	onError: (error: Error) =>
		notify(`Не удалось восстановить: ${error.message}`),
});
