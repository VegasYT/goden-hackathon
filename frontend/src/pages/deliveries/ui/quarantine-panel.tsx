import { Alert, Stack, Typography } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';

import { AccessNote, usePoint, writeAccess } from '@entities/profile';
import { EmptyState, QueryState } from '@shared/ui/empty-state';
import { InfoNote } from '@shared/ui/info-note';
import { SidePanel } from '@shared/ui/side-panel';
import { SkeletonList } from '@shared/ui/skeleton-list';

import { openQuarantineItems } from '../model/quarantine';
import { openQuarantineQueryOptions } from '../api/open-quarantine';
import { QuarantineCard, type ClosedMessage } from './quarantine-card';

interface QuarantinePanelProps {
	pointId: string;
	open: boolean;
	now: Date;
	onClose: () => void;
}

/** Панель открытого карантина точки: позиции всех поставок, по которым нет решения. */
export function QuarantinePanel({
	pointId,
	open,
	now,
	onClose,
}: QuarantinePanelProps) {
	const { profile, point } = usePoint(pointId);
	const access = writeAccess(profile, point, 'deliveries.approve');
	const mayResolve = access === 'allowed';
	const list = useQuery({
		...openQuarantineQueryOptions(pointId),
		enabled: open,
	});
	const [message, setMessage] = useState<ClosedMessage>();
	const entries = list.data ? openQuarantineItems(list.data.items) : [];

	return (
		<SidePanel
			open={open}
			title='Открытый карантин'
			count={list.data ? entries.length : undefined}
			closeLabel='Закрыть карантин'
			onClose={onClose}
			width={440}
		>
			<Stack sx={{ gap: 2, p: { xs: 2, sm: 3 } }}>
				<Typography sx={{ color: 'text.secondary' }}>
					Позиции из прошлых поставок ждут решения. Пока решение не принято, они
					не входят в доступный остаток.
				</Typography>
				<AccessNote
					access={access}
					disabled='Точка отключена — решить карантин нельзя'
					forbidden='Решить карантин может сотрудник с правом приёмки поставок'
				/>
				{message && (
					<Alert
						severity={message.severity}
						role='status'
						onClose={() => setMessage(undefined)}
						// Сообщение остаётся видно, даже если решение приняли внизу длинного списка.
						sx={{ position: 'sticky', top: 0, zIndex: 1 }}
					>
						{message.text}
					</Alert>
				)}
				<QueryState
					query={list}
					errorTitle='Не удалось загрузить карантин'
					loading={
						<SkeletonList label='Загружаем карантин' count={2} height={180} />
					}
				>
					{() =>
						entries.length === 0 ? (
							<EmptyState
								icon='quarantine'
								title='Открытого карантина нет'
								description='Позиции появятся здесь, если при приёмке их отправят в карантин'
							/>
						) : (
							<Stack
								component='ul'
								aria-label='Позиции в карантине'
								sx={{ gap: 2, m: 0, p: 0, listStyle: 'none' }}
							>
								{entries.map((entry) => (
									<QuarantineCard
										key={`${entry.delivery.id}/${entry.item.id}`}
										pointId={pointId}
										entry={entry}
										now={now}
										mayResolve={mayResolve}
										onClosed={setMessage}
									/>
								))}
							</Stack>
						)
					}
				</QueryState>
				{list.data && list.data.total > list.data.items.length && (
					<Typography variant='caption' sx={{ color: 'text.secondary' }}>
						Показаны позиции из первых {list.data.items.length} поставок с
						открытым карантином
					</Typography>
				)}
				{mayResolve && entries.length > 0 && (
					<InfoNote>
						Решение по карантину окончательное и попадает в журнал аудита.
						Принятое количество станет новой партией на складе.
					</InfoNote>
				)}
			</Stack>
		</SidePanel>
	);
}
