import { EmptyState } from '@shared/ui/empty-state';
import { ButtonLink } from '@shared/ui/link';
import { StandaloneScreen } from '@shared/ui/standalone-screen';

export function NotFoundPage() {
	return (
		<StandaloneScreen>
			<EmptyState
				icon='notFound'
				titleComponent='h1'
				title='Страница не найдена'
				description='Такого адреса в приложении нет. Проверьте ссылку или вернитесь к сводке.'
				action={
					<ButtonLink to='/' variant='outlined'>
						Перейти к сводке
					</ButtonLink>
				}
			/>
		</StandaloneScreen>
	);
}
