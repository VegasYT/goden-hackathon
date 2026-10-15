import { Button } from '@mui/material';
import { useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import {
	hotOfferSettingsQueryOptions,
	type DishRef,
	type HotOfferChange,
} from '@entities/dish';
import { useNotice } from '@shared/ui/notice';

import { useSetHotOffer } from '../api/hot-offer-writes';
import {
	autoChange,
	hideChange,
	pinChange,
	revertChange,
} from '../model/hot-offer';
import { PinDialog, type HotOfferActions, type PinTarget } from './hot-offers';

/** Действия управляющего с горячими предложениями, а также их диалог и сообщение с отменой, которые страница ставит у себя. */
export function useHotOffers(pointId: string) {
	const queryClient = useQueryClient();
	const write = useSetHotOffer(pointId);
	const [target, setTarget] = useState<PinTarget | null>(null);
	const notice = useNotice();

	/** Записывает режим; отмена в сообщении вернёт настройку блюда, которая была до записи. */
	const apply = (
		dish: DishRef,
		change: HotOfferChange,
		text: string,
		undoLabel?: string
	) => {
		const previous = revertChange(
			queryClient.getQueryData(hotOfferSettingsQueryOptions(pointId).queryKey),
			dish.id
		);
		// Ошибку в открытом диалоге показывает сам диалог.
		const inDialog = target !== null;

		write.mutate(
			{ dishId: dish.id, change },
			{
				onSuccess: () => {
					setTarget(null);
					notice.show(
						text,
						undoLabel && (
							<Button
								onClick={() =>
									write.mutate({ dishId: dish.id, change: previous })
								}
							>
								{undoLabel}
							</Button>
						)
					);
				},
				onError: (error) => {
					if (!inDialog) {
						notice.show(`Не удалось сохранить: ${error.message}`);
					}
				},
			}
		);
	};

	const actions: HotOfferActions = {
		add: () => setTarget({}),
		pin: (dish) => setTarget({ dish }),
		edit: (dish, manual) => setTarget({ dish, manual }),
		hide: (dish) =>
			apply(
				dish,
				hideChange(),
				`«${dish.name}» скрыто до конца дня`,
				'Вернуть'
			),
		unpin: (dish) =>
			apply(
				dish,
				autoChange,
				`«${dish.name}» больше не закреплено`,
				'Отменить'
			),
		restore: (dish) =>
			apply(dish, autoChange, `«${dish.name}» снова в расчёте`),
	};

	const elements = (
		<>
			{target && (
				<PinDialog
					pointId={pointId}
					target={target}
					pending={write.isPending}
					error={write.error?.message}
					onSubmit={(dish, comment) =>
						target.manual
							? apply(dish, pinChange(comment), 'Комментарий сохранён')
							: apply(
									dish,
									pinChange(comment),
									`«${dish.name}» закреплено первым`,
									'Отменить'
								)
					}
					onClose={() => {
						write.reset();
						setTarget(null);
					}}
				/>
			)}
			{notice.element}
		</>
	);

	return { actions, elements };
}
