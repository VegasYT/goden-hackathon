import { Alert, Button, Stack } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';

import { migomQueryOptions } from '@entities/organization';
import type { WriteAccess } from '@entities/profile';
import { ConfirmDialog } from '@shared/ui/confirm-dialog';
import { InfoNote } from '@shared/ui/info-note';
import { ReadOnlyNote } from '@shared/ui/read-only-note';
import { SettingsSection } from '@shared/ui/settings-section';
import { SkeletonList } from '@shared/ui/skeleton-list';

import { useSetMigom } from '../api/point-writes';
import { parseOutletId } from '../model/outlet-id';
import { ConnectionStatus, NumberConnectForm } from './number-connection';

const description =
	'Мигом показывает покупателям меню точки и горячие предложения, оформляет заказы на кухню и отменяет их. Заказы Мигома расходуют склад точки так же, как заказы официантов.';

interface MigomSectionProps {
	pointId: string;
	pointName: string;
	/** Право владельца на точке: на отключённой точке Мигом можно только отключить. */
	access: Exclude<WriteAccess, 'forbidden'>;
}

/** Карточка «Доставка Мигом»: подключено ли заведение сервиса доставки к точке. */
export function MigomSection({
	pointId,
	pointName,
	access,
}: MigomSectionProps) {
	const migom = useQuery(migomQueryOptions(pointId));

	return (
		<SettingsSection title='Доставка Мигом' description={description}>
			{migom.isPending ? (
				<SkeletonList
					label='Загружаем подключение Мигома'
					count={1}
					height={36}
				/>
			) : migom.isError ? (
				<Alert
					severity='error'
					action={
						<Button color='inherit' onClick={() => void migom.refetch()}>
							Повторить
						</Button>
					}
				>
					Не удалось загрузить подключение Мигома: {migom.error.message}
				</Alert>
			) : (
				<MigomForm
					key={migom.data.outlet_id}
					pointId={pointId}
					pointName={pointName}
					access={access}
					outletId={migom.data.outlet_id}
				/>
			)}
		</SettingsSection>
	);
}

function MigomForm({
	pointId,
	pointName,
	access,
	outletId,
}: MigomSectionProps & { outletId: number | null }) {
	const connected = outletId !== null;
	const [confirming, setConfirming] = useState(false);
	const mutation = useSetMigom(pointId);
	const disconnecting = mutation.isPending && mutation.variables === null;

	const disconnectButton = connected && (
		<Button
			onClick={() => setConfirming(true)}
			sx={{ mt: access === 'allowed' ? 3 : 0, color: 'text.secondary' }}
		>
			Отключить Мигом
		</Button>
	);

	return (
		<>
			<ConnectionStatus
				connected={connected}
				on='подключён'
				off='не подключён'
				detail={`заведение № ${outletId}`}
			/>
			{access === 'allowed' ? (
				<>
					<NumberConnectForm
						label='Номер заведения в Мигоме'
						placeholder='например, 42'
						invalidText='Нужен номер заведения: целое число больше нуля'
						parse={parseOutletId}
						current={outletId}
						pending={mutation.isPending && !disconnecting}
						error={confirming ? undefined : mutation.error?.message}
						onSubmit={(next) => mutation.mutate(next)}
					>
						{disconnectButton}
					</NumberConnectForm>
					<InfoNote>
						Номер заведения — в кабинете владельца Мигома. Там же выберите эту
						точку: Мигом начнёт работать с ней, когда обе стороны укажут друг
						друга.
					</InfoNote>
				</>
			) : (
				<Stack
					direction='row'
					sx={{ alignItems: 'center', gap: 1.5, flexWrap: 'wrap' }}
				>
					<ReadOnlyNote>
						{connected
							? 'Точка отключена — Мигом с ней не работает. Подключение можно снять.'
							: 'Точка отключена — подключить к ней Мигом нельзя'}
					</ReadOnlyNote>
					{disconnectButton}
				</Stack>
			)}
			<ConfirmDialog
				open={confirming}
				title={`Отключить Мигом от «${pointName}»?`}
				description='Покупатели Мигома сразу перестанут видеть меню точки и оформлять на ней заказы. Принятые заказы останутся в истории. Подключить снова можно в любой момент.'
				confirmLabel='Отключить Мигом'
				confirmColor='error'
				pending={disconnecting}
				error={confirming ? mutation.error?.message : undefined}
				onConfirm={() => mutation.mutate(null)}
				onClose={() => {
					mutation.reset();
					setConfirming(false);
				}}
			/>
		</>
	);
}
