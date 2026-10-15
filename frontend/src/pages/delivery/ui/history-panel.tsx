import { Button, Collapse, Stack, Typography } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { useId, useState } from 'react';

import { actorName } from '@entities/actor';
import {
	deliveryHistoryQueryOptions,
	itemName,
	itemUnit,
	positionsCount,
	reasonLabel,
	type Delivery,
} from '@entities/delivery';
import { formatDateTime } from '@shared/lib/datetime';
import { formatQuantity } from '@shared/lib/quantity';
import { QueryState } from '@shared/ui/empty-state';
import { DisclosureIcon, type IconName } from '@shared/ui/icon';
import { SidePanel } from '@shared/ui/side-panel';
import { SkeletonList } from '@shared/ui/skeleton-list';
import { radius } from '@shared/ui/theme';
import { TimelineItem } from '@shared/ui/timeline';

import { historyView, type HistoryView } from '../model/history';
import { ChangeLines } from './change-lines';

interface HistoryPanelProps {
	pointId: string;
	delivery: Delivery;
	open: boolean;
	now: Date;
	onClose: () => void;
}

const icons: Record<HistoryView['kind'], IconName> = {
	registered: 'registered',
	approved: 'approved',
	quarantine: 'quarantineResolved',
	corrected: 'corrected',
};

function title(view: HistoryView): string {
	switch (view.kind) {
		case 'registered':
			return 'Зарегистрирована';

		case 'approved':
			return 'Принята';

		case 'quarantine':
			return `Карантин решён: ${itemName(view.item)}`;

		case 'corrected':
			return 'Приёмка исправлена';
	}
}

function Details({ view }: { view: HistoryView }) {
	switch (view.kind) {
		case 'registered':
			return (
				<Typography variant='body2'>
					{positionsCount(view.delivery.items.length)}, заявлено по документам
					поставки
				</Typography>
			);

		case 'approved':
			return (
				<Typography variant='body2'>
					{view.outcomes.toStock} на склад · {view.outcomes.rejected} отклонено
					· {view.outcomes.quarantined} в карантин
				</Typography>
			);

		case 'quarantine': {
			const unit = itemUnit(view.item);
			const parts = [
				view.accepted > 0 &&
					`принято ${formatQuantity(view.accepted, unit)} — новая партия`,
				view.rejected > 0 && `отклонено ${formatQuantity(view.rejected, unit)}`,
			].filter(Boolean);
			const why = [
				view.rejectReason && reasonLabel(view.rejectReason),
				view.comment,
			].filter(Boolean);

			return (
				<>
					<Typography variant='body2'>{parts.join('; ')}</Typography>
					{why.length > 0 && (
						<Typography variant='body2' sx={{ color: 'text.secondary' }}>
							{why.join(' — ')}
						</Typography>
					)}
				</>
			);
		}

		case 'corrected':
			return (
				<Stack sx={{ gap: 1 }}>
					{view.reason && (
						<Typography variant='body2'>Причина: {view.reason}</Typography>
					)}
					<Stack
						aria-label='Изменения: было → стало'
						sx={(theme) => ({
							gap: 1.25,
							p: 1.5,
							borderRadius: radius(theme, 'md'),
							bgcolor: 'background.default',
						})}
					>
						{view.changes.map(({ item, change }) => (
							<ChangeLines
								key={item.id}
								item={item}
								change={change}
								tense='done'
							/>
						))}
					</Stack>
				</Stack>
			);
	}
}

/** Позиции, которые после события отклонены или в карантине. */
function StateAfter({ delivery }: { delivery: Delivery }) {
	const items = delivery.items.filter(
		(item) => item.rejected_quantity > 0 || item.quarantined_quantity > 0
	);

	if (items.length === 0) {
		return (
			<Typography variant='body2' sx={{ color: 'text.secondary' }}>
				Все позиции на складе
			</Typography>
		);
	}

	return (
		<Stack component='ul' sx={{ m: 0, pl: 2.5, gap: 0.5 }}>
			{items.map((item) => {
				const unit = itemUnit(item);
				const parts = [
					item.rejected_quantity > 0 &&
						`отклонено ${formatQuantity(item.rejected_quantity, unit)}`,
					item.quarantined_quantity > 0 &&
						`в карантине ${formatQuantity(item.quarantined_quantity, unit)}`,
				].filter(Boolean);
				const reasons = [item.reject_reason, item.quarantine_reason]
					.filter((reason): reason is string => Boolean(reason))
					.map(reasonLabel);

				return (
					<Typography key={item.id} component='li' variant='body2'>
						{itemName(item)} — {parts.join(', ')}
						{reasons.length > 0 && (
							<Typography
								component='span'
								variant='body2'
								sx={{ color: 'text.secondary' }}
							>
								{' · '}
								{reasons.join(', ')}
							</Typography>
						)}
					</Typography>
				);
			})}
		</Stack>
	);
}

function HistoryEvent({ view, now }: { view: HistoryView; now: Date }) {
	const [expanded, setExpanded] = useState(false);
	const stateId = useId();

	return (
		<TimelineItem icon={icons[view.kind]}>
			<Stack sx={{ gap: 0.5 }}>
				<Typography
					component='h3'
					sx={{ fontWeight: 600, color: 'text.strong' }}
				>
					{title(view)}
				</Typography>
				<Typography variant='caption' sx={{ color: 'text.secondary' }}>
					{formatDateTime(view.at, now)} · {actorName(view.actor)}
				</Typography>
				<Details view={view} />
				{view.kind !== 'registered' && (
					<>
						<Button
							variant='text'
							size='small'
							aria-expanded={expanded}
							aria-controls={stateId}
							endIcon={<DisclosureIcon open={expanded} />}
							onClick={() => setExpanded((value) => !value)}
							sx={{ alignSelf: 'flex-start', ml: -1 }}
						>
							Состояние после события
						</Button>
						<Collapse in={expanded} id={stateId}>
							<StateAfter delivery={view.delivery} />
						</Collapse>
					</>
				)}
			</Stack>
		</TimelineItem>
	);
}

/** Боковая панель с историей поставки: события в порядке появления. */
export function HistoryPanel({
	pointId,
	delivery,
	open,
	now,
	onClose,
}: HistoryPanelProps) {
	const history = useQuery({
		...deliveryHistoryQueryOptions(pointId, delivery.id),
		enabled: open,
	});

	return (
		<SidePanel
			open={open}
			title={`История поставки № ${delivery.number}`}
			closeLabel='Закрыть историю'
			onClose={onClose}
		>
			<Stack sx={{ gap: 2.5, p: { xs: 2, sm: 3 } }}>
				<Typography variant='body2' sx={{ color: 'text.secondary' }}>
					События в порядке появления. Исходная приёмка не меняется —
					исправления и решения по карантину добавляются новыми событиями.
				</Typography>
				<QueryState
					query={history}
					errorTitle='Не удалось загрузить историю'
					loading={
						<SkeletonList label='Загружаем историю' count={3} height={72} />
					}
				>
					{(data) => (
						<Stack
							component='ol'
							aria-label='События поставки'
							sx={{ m: 0, p: 0, listStyle: 'none', gap: 2.5 }}
						>
							{historyView(data.items).map((view, index) => (
								<HistoryEvent
									key={`${view.at}-${index}`}
									view={view}
									now={now}
								/>
							))}
						</Stack>
					)}
				</QueryState>
			</Stack>
		</SidePanel>
	);
}
