import { Alert, Button, Chip, Stack } from '@mui/material';
import { useSuspenseQuery } from '@tanstack/react-query';
import { useRouterState } from '@tanstack/react-router';
import { useEffect, useRef, useState } from 'react';

import { actorName } from '@entities/actor';
import {
	deliveryQueryOptions,
	DeliveryStatusChip,
	positionsCount,
	registeredMessage,
	type Delivery,
} from '@entities/delivery';
import { AccessNote, usePoint, writeAccess } from '@entities/profile';
import { formatLongDateTime, formatTime } from '@shared/lib/datetime';
import { useNow } from '@shared/lib/expiry';
import { Icon } from '@shared/ui/icon';
import { ButtonLink } from '@shared/ui/link';
import { useNotice } from '@shared/ui/notice';
import { PageContent } from '@shared/ui/page-content';
import { PageHeader } from '@shared/ui/page-header';

import type { DeliveryPanel } from '../model/search';
import { ApproveDeliveryForm } from './approval-form';
import { DeliveryBreadcrumbs } from './delivery-breadcrumbs';
import { DeliverySnapshot } from './delivery-snapshot';
import { HistoryPanel } from './history-panel';
import { PendingItems } from './pending-items';

interface DeliveryPageProps {
	pointId: string;
	deliveryId: string;
	panel: DeliveryPanel;
	/** Открывает или закрывает боковую панель карточки через адрес. */
	onPanelChange: (panel: DeliveryPanel) => void;
}

function meta(delivery: Delivery, now: Date): string {
	const parts = [`Поступила ${formatLongDateTime(delivery.delivered_at)}`];

	if (delivery.approved_by && delivery.approved_at) {
		parts.push(
			`приёмка: ${actorName(delivery.approved_by)}, ${formatLongDateTime(delivery.approved_at)}`
		);
	}

	parts.push(positionsCount(delivery.items.length));

	if (delivery.status === 'registered') {
		parts.push(`сроки проверены на ${formatTime(now)}`);
	}

	return parts.join(' · ');
}

function alreadyApprovedText(delivery: Delivery): string {
	const by = [
		delivery.approved_by ? actorName(delivery.approved_by) : 'другой сотрудник',
		delivery.approved_at && formatTime(delivery.approved_at),
	]
		.filter(Boolean)
		.join(', ');

	return `Поставка № ${delivery.number} уже принята: ${by}. Ваши решения не сохранены.`;
}

/** Карточка поставки: форма приёмки у ожидающей, снимок приёмки у принятой. */
export function DeliveryPage({
	pointId,
	deliveryId,
	panel,
	onPanelChange,
}: DeliveryPageProps) {
	const { data: delivery } = useSuspenseQuery(
		deliveryQueryOptions(pointId, deliveryId)
	);
	const { profile, point } = usePoint(pointId);
	const now = useNow();
	const [alreadyApproved, setAlreadyApproved] = useState(false);
	const registered = useRouterState({
		select: (state) => state.location.state.registeredDelivery,
	});
	// Внизу карточки стоит панель итога приёмки, поэтому сообщение показывается сверху.
	const notice = useNotice({ top: true });
	const registeredShown = useRef(false);

	useEffect(() => {
		if (registered && !registeredShown.current) {
			registeredShown.current = true;
			notice.show(registeredMessage(registered.number, registered.positions));
		}
	}, [registered, notice]);

	const pendingApproval = delivery.status === 'registered';
	const access = writeAccess(profile, point, 'deliveries.approve');
	const mayApprove = access === 'allowed';

	const header = (
		<PageHeader
			breadcrumbs={
				<DeliveryBreadcrumbs pointId={pointId} delivery={delivery} />
			}
			title={
				pendingApproval
					? `Приёмка поставки № ${delivery.number}`
					: `Поставка № ${delivery.number}`
			}
			status={
				<Stack direction='row' sx={{ gap: 1 }}>
					<DeliveryStatusChip status={delivery.status} />
					{delivery.corrected_at && (
						<Chip icon={<Icon name='edit' size='sm' />} label='исправлена' />
					)}
				</Stack>
			}
			meta={meta(delivery, now)}
			actions={
				!pendingApproval && (
					<Stack direction='row' sx={{ gap: 1, flexWrap: 'wrap' }}>
						<Button
							variant='outlined'
							startIcon={<Icon name='history' />}
							onClick={() => onPanelChange('history')}
						>
							История событий
						</Button>
						{mayApprove && (
							<ButtonLink
								to='/points/$pointId/deliveries/$deliveryId/correction'
								params={{ pointId, deliveryId }}
								variant='outlined'
								startIcon={<Icon name='edit' />}
							>
								Исправить приёмку
							</ButtonLink>
						)}
					</Stack>
				)
			}
		/>
	);

	if (pendingApproval && mayApprove) {
		return (
			<>
				{notice.element}
				<ApproveDeliveryForm
					pointId={pointId}
					delivery={delivery}
					now={now}
					header={header}
					onAlreadyApproved={() => setAlreadyApproved(true)}
				/>
			</>
		);
	}

	return (
		<PageContent>
			{notice.element}
			{header}
			<Stack sx={{ gap: 2 }}>
				{alreadyApproved && !pendingApproval && (
					<Alert severity='warning'>{alreadyApprovedText(delivery)}</Alert>
				)}
				{pendingApproval && (
					<AccessNote
						access={access}
						disabled='Точка отключена — принять поставку нельзя'
						forbidden='Принять поставку может сотрудник с правом приёмки поставок'
					/>
				)}
				{pendingApproval ? (
					<PendingItems delivery={delivery} now={now} />
				) : (
					<DeliverySnapshot delivery={delivery} now={now} />
				)}
			</Stack>
			{!pendingApproval && (
				<HistoryPanel
					pointId={pointId}
					delivery={delivery}
					open={panel === 'history'}
					now={now}
					onClose={() => onPanelChange(undefined)}
				/>
			)}
		</PageContent>
	);
}
