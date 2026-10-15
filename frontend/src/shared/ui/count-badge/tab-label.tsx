import { CountBadge } from './count-badge';

/** Подпись вкладки с числом; число не показывается, пока его нет или оно ноль. */
export function TabLabel({
	text,
	count,
	tone,
}: {
	text: string;
	count: number | undefined;
	tone?: 'warning';
}) {
	return (
		<span>
			{text}
			{!!count && (
				<>
					{' '}
					<CountBadge count={count} tone={tone} sx={{ ml: 0.5 }} />
				</>
			)}
		</span>
	);
}
