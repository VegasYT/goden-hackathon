import { Button } from '@mui/material';

import type { DishRef, PointDish } from '@entities/dish';
import { ConfirmDialog } from '@shared/ui/confirm-dialog';

import { useSetDishOnPoint } from '../api/set-dish-on-point';

interface DisableDialogProps {
	pointId: string;
	dish: DishRef;
	onClose: () => void;
}

/** Подтверждение выключения блюда на точке. */
export function DisableDialog({ pointId, dish, onClose }: DisableDialogProps) {
	const mutation = useSetDishOnPoint(pointId, dish.id);

	return (
		<ConfirmDialog
			open
			title={`Выключить «${dish.name}»?`}
			description='Блюдо пропадёт из меню и стоп-листа точки: официанты не смогут его заказать. Включить его снова можно на этой странице, в группе «Выключены на точке».'
			confirmLabel='Выключить'
			confirmColor='error'
			pending={mutation.isPending}
			error={mutation.error?.message}
			onConfirm={() => mutation.mutate(false, { onSuccess: onClose })}
			onClose={onClose}
		/>
	);
}

export function DisableButton({
	dish,
	onDisable,
}: {
	dish: DishRef;
	onDisable: (dish: DishRef) => void;
}) {
	return (
		<Button
			size='small'
			aria-label={`Выключить на точке: ${dish.name}`}
			onClick={() => onDisable(dish)}
			sx={{ color: 'text.secondary' }}
		>
			Выключить
		</Button>
	);
}

interface EnableButtonProps {
	pointId: string;
	dish: PointDish;
	/** Текст ошибки для сообщения над меню; `null` убирает прежнее сообщение. */
	onError: (text: string | null) => void;
}

/** «Включить» без подтверждения. */
export function EnableButton({ pointId, dish, onError }: EnableButtonProps) {
	const mutation = useSetDishOnPoint(pointId, dish.dish_id);

	return (
		<Button
			variant='outlined'
			size='small'
			loading={mutation.isPending}
			aria-label={`Включить на точке: ${dish.name}`}
			onClick={() => {
				onError(null);
				mutation.mutate(true, {
					onError: (error) =>
						onError(`Не удалось включить «${dish.name}»: ${error.message}`),
				});
			}}
		>
			Включить
		</Button>
	);
}
