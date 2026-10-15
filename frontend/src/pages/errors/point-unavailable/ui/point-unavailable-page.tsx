import { Typography } from '@mui/material';

import type { ProfilePoint } from '@entities/profile';
import { EmptyState } from '@shared/ui/empty-state';
import { ButtonLink } from '@shared/ui/link';
import { PageContent } from '@shared/ui/page-content';

/** Страница точки, которой нет в профиле пользователя; `target` — куда ведёт кнопка. */
export function PointUnavailablePage({ target }: { target?: ProfilePoint }) {
	return (
		<PageContent>
			<EmptyState
				icon='pointUnavailable'
				titleComponent='h1'
				title='Точка недоступна'
				description='Вас могли снять с этой точки, или ссылка неверная. Выберите точку в переключателе.'
				action={
					<>
						<ButtonLink to='/' variant='contained'>
							Открыть мою точку
						</ButtonLink>
						{target && (
							<Typography
								variant='caption'
								component='p'
								sx={{ mt: 1, color: 'text.tertiary' }}
							>
								{target.name} · {target.company_name}
							</Typography>
						)}
					</>
				}
			/>
		</PageContent>
	);
}
