import { useNavigate } from '@tanstack/react-router';

/** Открывает и закрывает историю продукта поверх текущей вкладки склада: `undefined` закрывает. */
export function useHistoryPanel() {
	const navigate = useNavigate();

	return (productId: string | undefined) =>
		void navigate({
			to: '.',
			search: (prev: Record<string, unknown>) => ({
				...prev,
				history: productId,
			}),
		});
}
