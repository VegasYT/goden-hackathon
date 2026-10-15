import {
	Autocomplete,
	Box,
	Button,
	Stack,
	TextField,
	Typography,
} from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';

import { actorName } from '@entities/actor';
import {
	hotOfferSettingsQueryOptions,
	menuQueryOptions,
	type DishRef,
	type HotDish,
} from '@entities/dish';
import { ActionMenu, type ActionMenuItem } from '@shared/ui/action-menu';
import { FormDialog } from '@shared/ui/form-dialog';
import { formatEventTime, formatTime } from '@shared/lib/datetime';
import { Icon } from '@shared/ui/icon';

type Manual = NonNullable<HotDish['manual']>;

/** Действия управляющего с горячими предложениями; их даёт `useHotOffers`. */
export interface HotOfferActions {
	add: () => void;
	pin: (dish: DishRef) => void;
	edit: (dish: DishRef, manual: Manual) => void;
	hide: (dish: DishRef) => void;
	unpin: (dish: DishRef) => void;
	restore: (dish: DishRef) => void;
}

/** Комментарий управляющего к закреплённому блюду: что сказать гостю, кто и когда закрепил. */
export function ManualNote({ manual, now }: { manual: Manual; now: Date }) {
	return (
		<Box>
			{manual.comment && (
				<Typography sx={{ color: 'text.strong' }}>
					«{manual.comment}»
				</Typography>
			)}
			<Typography variant='caption' sx={{ color: 'text.secondary' }}>
				{actorName(manual.set_by)} · {formatEventTime(manual.set_at, now)}
			</Typography>
		</Box>
	);
}

/** Что открыть в диалоге: без блюда — выбор из меню, с `manual` — правка комментария закреплённого. */
export interface PinTarget {
	dish?: DishRef;
	manual?: Manual;
}

/** Диалог закрепления: блюдо из меню точки или выбранное в списке и комментарий для официанта. */
export function PinDialog({
	pointId,
	target,
	pending,
	error,
	onSubmit,
	onClose,
}: {
	pointId: string;
	target: PinTarget;
	pending: boolean;
	error?: string;
	onSubmit: (dish: DishRef, comment: string) => void;
	onClose: () => void;
}) {
	const menu = useQuery({
		...menuQueryOptions(pointId),
		enabled: !target.dish,
	});
	const settings = useQuery(hotOfferSettingsQueryOptions(pointId));
	const pinned = new Set(
		(settings.data ?? [])
			.filter(({ mode }) => mode === 'pinned')
			.map(({ dish }) => dish.id)
	);
	const options = (menu.data ?? []).filter(({ dish }) => !pinned.has(dish.id));
	const [chosen, setChosen] = useState<(typeof options)[number] | null>(null);
	const [comment, setComment] = useState(target.manual?.comment ?? '');
	const [submitted, setSubmitted] = useState(false);
	const dish = target.dish ?? chosen?.dish;

	const title = target.dish
		? target.manual
			? target.dish.name
			: `Закрепить «${target.dish.name}»`
		: 'Добавить в горячие предложения';

	return (
		<FormDialog
			title={title}
			submitLabel={target.manual ? 'Сохранить' : 'Закрепить'}
			pending={pending}
			error={error}
			onSubmit={() => {
				setSubmitted(true);

				if (dish) {
					onSubmit(dish, comment);
				}
			}}
			onClose={onClose}
		>
			{!target.dish && (
				<Autocomplete
					options={options}
					value={chosen}
					onChange={(_, next) => setChosen(next)}
					getOptionLabel={(option) => option.dish.name}
					isOptionEqualToValue={(option, value) =>
						option.dish.id === value.dish.id
					}
					loading={menu.isPending}
					noOptionsText='В меню точки нет блюд, которые можно закрепить'
					renderInput={(params) => (
						<TextField
							{...params}
							label='Блюдо'
							autoFocus
							error={submitted && !dish}
							helperText={
								submitted && !dish
									? 'Выберите блюдо'
									: chosen && !chosen.available
										? 'Сейчас блюдо в стоп-листе: его покажут, когда его можно будет приготовить'
										: 'Блюда меню точки; закреплённые не предлагаются'
							}
						/>
					)}
				/>
			)}
			<TextField
				label='Комментарий для официанта'
				multiline
				minRows={2}
				value={comment}
				onChange={(event) => setComment(event.target.value)}
				helperText='Необязательно, до 500 знаков. Блюдо будет первым в списке, пока его можно приготовить.'
				autoFocus={Boolean(target.dish)}
				slotProps={{ htmlInput: { maxLength: 500 } }}
			/>
		</FormDialog>
	);
}

/** Меню действий у блюда: у закреплённого — комментарий и снятие, у блюда из расчёта — закрепить и скрыть. */
export function OfferMenu({
	dish,
	actions,
}: {
	dish: HotDish;
	actions: HotOfferActions;
}) {
	const { manual } = dish;
	const items: ActionMenuItem[] = manual
		? [
				{
					icon: 'edit',
					label: 'Изменить комментарий',
					onClick: () => actions.edit(dish.dish, manual),
				},
				{
					icon: 'unpin',
					label: 'Снять закрепление',
					hint: 'блюдо вернётся в расчёт',
					onClick: () => actions.unpin(dish.dish),
				},
			]
		: [
				{
					icon: 'pin',
					label: 'Закрепить первым',
					hint: 'с комментарием для официанта',
					onClick: () => actions.pin(dish.dish),
				},
				{
					icon: 'hide',
					label: 'Скрыть до конца дня',
					hint: 'завтра блюдо снова выберет расчёт',
					onClick: () => actions.hide(dish.dish),
				},
			];

	return <ActionMenu label={`Действия: ${dish.dish.name}`} items={items} />;
}

/** «Скрыто до конца дня»: блюда, скрытые сегодня, с возвратом расчёту; без скрытых не показывается. */
export function HiddenOffers({
	pointId,
	actions,
}: {
	pointId: string;
	actions: HotOfferActions;
}) {
	const settings = useQuery(hotOfferSettingsQueryOptions(pointId));
	const hidden = (settings.data ?? []).filter(({ mode }) => mode === 'hidden');

	if (hidden.length === 0) {
		return null;
	}

	return (
		<Box
			component='section'
			aria-label='Скрыто до конца дня'
			sx={{ px: 2, py: 1.5, borderTop: 1, borderColor: 'divider' }}
		>
			<Typography variant='body2' sx={{ color: 'text.secondary', mb: 1 }}>
				Скрыто до конца дня · {hidden.length}
			</Typography>
			<Stack component='ul' sx={{ listStyle: 'none', m: 0, p: 0, gap: 0.5 }}>
				{hidden.map((setting) => (
					<Stack
						component='li'
						key={setting.dish.id}
						direction='row'
						sx={{ alignItems: 'center', gap: 1.5, flexWrap: 'wrap' }}
					>
						<Icon name='hide' size='sm' sx={{ color: 'text.tertiary' }} />
						<Typography sx={{ color: 'text.strong' }}>
							{setting.dish.name}
						</Typography>
						<Typography variant='body2' sx={{ color: 'text.secondary' }}>
							{[
								setting.hidden_until &&
									`до ${formatTime(setting.hidden_until)}`,
								actorName(setting.set_by),
							]
								.filter(Boolean)
								.join(' · ')}
						</Typography>
						<Box sx={{ flexGrow: 1 }} />
						<Button
							size='small'
							aria-label={`Вернуть в расчёт: ${setting.dish.name}`}
							onClick={() => actions.restore(setting.dish)}
						>
							Вернуть
						</Button>
					</Stack>
				))}
			</Stack>
		</Box>
	);
}
