import { EmptyState } from '@shared/ui/empty-state';

/** Пустое меню точки; `hint` дописывает, что появится на экране вместе с блюдами. */
export function EmptyMenuState({ hint }: { hint?: string }) {
	return (
		<EmptyState
			icon='kitchen'
			title='В меню точки пока нет блюд'
			description={[
				'Блюда и их рецептуры добавляет владелец компании в каталоге.',
				hint,
			]
				.filter(Boolean)
				.join(' ')}
		/>
	);
}
