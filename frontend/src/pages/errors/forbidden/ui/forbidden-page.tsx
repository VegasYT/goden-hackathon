import { EmptyState } from '@shared/ui/empty-state';
import { ButtonLink } from '@shared/ui/link';
import { PageHeader } from '@shared/ui/page-header';
import { PageContent } from '@shared/ui/page-content';

/** Страница раздела, на который у пользователя нет права. */
export function ForbiddenPage({ section }: { section: string }) {
	return (
		<PageContent>
			<PageHeader title={section} />
			<EmptyState
				icon='lock'
				title='Нет доступа к разделу'
				description='У вашей учётной записи нет права на этот раздел. Если оно нужно для работы, обратитесь к администратору.'
				action={
					<ButtonLink to='/' variant='outlined'>
						Перейти к сводке
					</ButtonLink>
				}
			/>
		</PageContent>
	);
}
