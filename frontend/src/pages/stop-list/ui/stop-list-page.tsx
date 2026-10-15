import { Paper, Stack } from '@mui/material';
import { useQuery } from '@tanstack/react-query';

import {
	EmptyMenuState,
	menuQueryOptions,
	stoppedDishes,
} from '@entities/dish';
import { can, mayWrite, usePoint } from '@entities/profile';
import { formatTime } from '@shared/lib/datetime';
import { countOf } from '@shared/lib/plural';
import { EmptyState, LoadErrorState } from '@shared/ui/empty-state';
import { InfoNote } from '@shared/ui/info-note';
import { Icon } from '@shared/ui/icon';
import { ButtonLink } from '@shared/ui/link';
import { PageContent } from '@shared/ui/page-content';
import { PageHeader } from '@shared/ui/page-header';
import { SkeletonList } from '@shared/ui/skeleton-list';
import { useWideLayout } from '@shared/ui/theme';

import { StopDishCard } from './stop-dish-card';

/** Блюда точки, которые сейчас нельзя приготовить, с причинами; читает то же меню, что «Кухня». */
export function StopListPage({ pointId }: { pointId: string }) {
	const wide = useWideLayout();
	const { profile, point } = usePoint(pointId);
	const mayViewStock = can(point, 'inventory.view');
	const mayManage = mayWrite(profile, point, 'menu.manage');
	const menu = useQuery(menuQueryOptions(pointId));
	const stopped = stoppedDishes(menu.data ?? []);

	// Ошибка и пустые состояния вместо списка; null, если показывать нужно список.
	const message = () => {
		if (menu.isError) {
			return (
				<LoadErrorState title='Не удалось загрузить стоп-лист' query={menu} />
			);
		}

		if (menu.data?.length === 0) {
			return <EmptyMenuState />;
		}

		if (menu.data && stopped.length === 0) {
			return (
				<EmptyState
					icon='done'
					title='Все блюда можно приготовить'
					description='Продуктов хватает хотя бы на одну порцию каждого блюда меню.'
					action={
						<ButtonLink
							to='/points/$pointId/kitchen'
							params={{ pointId }}
							variant='outlined'
						>
							Открыть кухню
						</ButtonLink>
					}
				/>
			);
		}

		return null;
	};

	const content = () => {
		if (menu.isPending) {
			return (
				<SkeletonList label='Загружаем стоп-лист' count={3} height={120} />
			);
		}

		const state = message();

		if (state) {
			return <Paper>{state}</Paper>;
		}

		return (
			<Stack sx={{ gap: 2 }}>
				<Stack
					component='ul'
					aria-label='Блюда в стоп-листе'
					sx={{ listStyle: 'none', m: 0, p: 0, gap: 1.5 }}
				>
					{stopped.map((dish) => (
						<li key={dish.dish.id}>
							<StopDishCard
								pointId={pointId}
								dish={dish}
								mayManage={mayManage}
								mayViewStock={mayViewStock}
								wide={wide}
							/>
						</li>
					))}
				</Stack>
				<InfoNote>
					Блюдо выходит из стоп-листа само, когда продуктов хватает хотя бы на
					одну порцию: после поставки, исправления остатка или изменения
					рецептуры.
				</InfoNote>
			</Stack>
		);
	};

	return (
		<PageContent>
			<PageHeader
				title='Стоп-лист'
				meta={
					stopped.length > 0 &&
					`${countOf(stopped.length, 'блюдо', 'блюда', 'блюд')} нельзя приготовить · на ${formatTime(new Date(menu.dataUpdatedAt))}`
				}
				actions={
					mayViewStock && (
						<ButtonLink
							to='/points/$pointId/stock'
							params={{ pointId }}
							variant='outlined'
							startIcon={<Icon name='stock' />}
						>
							Открыть склад
						</ButtonLink>
					)
				}
			/>
			{content()}
		</PageContent>
	);
}
