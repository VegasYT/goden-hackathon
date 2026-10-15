import { EmptyState } from '@shared/ui/empty-state';
import { ButtonLink } from '@shared/ui/link';
import { PageContent } from '@shared/ui/page-content';

/** Поставки нет на выбранной точке: чужая, удалённая ссылка или ошибка в адресе. */
export function DeliveryNotFound({ pointId }: { pointId: string }) {
	return (
		<PageContent>
			<EmptyState
				icon='notFound'
				titleComponent='h1'
				title='Поставка не найдена'
				description='Возможно, ссылка устарела или поставка ещё не зарегистрирована'
				action={
					<ButtonLink
						to='/points/$pointId/deliveries'
						params={{ pointId }}
						variant='outlined'
					>
						К списку поставок
					</ButtonLink>
				}
			/>
		</PageContent>
	);
}
