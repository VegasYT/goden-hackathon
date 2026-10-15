import { Alert, Button, Stack } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';

import { pointQueryOptions } from '@entities/organization';
import { AccessNote, usePoint, writeAccess } from '@entities/profile';
import { ConfirmDialog } from '@shared/ui/confirm-dialog';
import { PageContent } from '@shared/ui/page-content';
import { PageHeader } from '@shared/ui/page-header';
import { RenameForm } from '@shared/ui/rename-form';
import { SettingsSection } from '@shared/ui/settings-section';

import { useDisablePoint, useRenamePoint } from '../api/point-writes';
import { MigomSection } from './migom-section';
import { TelegramSection } from './telegram-section';

const disableWarning =
	'Сотрудники сразу потеряют доступ к точке. Поставки, заказы и списания на ней станут невозможны, данные останутся для чтения. Включить точку обратно нельзя.';

/** «Настройки точки»: название, Мигом и отключение — у владельца, чат Telegram — у управляющего и владельца. */
export function PointSettingsPage({ pointId }: { pointId: string }) {
	const { profile, point: profilePoint } = usePoint(pointId);
	const point = useQuery(pointQueryOptions(pointId));
	const ownerAccess = writeAccess(profile, profilePoint, 'company.manage');
	const chatAccess = writeAccess(profile, profilePoint, 'notifications.manage');
	const rename = useRenamePoint(pointId);
	const disable = useDisablePoint(pointId);
	const [confirming, setConfirming] = useState(false);
	const name = profilePoint.name;

	return (
		<PageContent>
			<PageHeader
				title='Настройки точки'
				meta={`${name} · компания «${profilePoint.company_name}»`}
			/>
			<Stack sx={{ gap: 2, maxWidth: 760 }}>
				{point.isError && (
					<Alert
						severity='error'
						action={
							<Button color='inherit' onClick={() => void point.refetch()}>
								Повторить
							</Button>
						}
					>
						Не удалось загрузить точку: {point.error.message}
					</Alert>
				)}
				<SettingsSection title='Название точки'>
					<RenameForm
						label='Название'
						value={name}
						hint='Видят все сотрудники точки. В компании название не повторяется.'
						pending={rename.isPending}
						error={rename.error?.message}
						onSave={(next) => rename.mutate(next)}
						readOnlyNote={
							ownerAccess !== 'allowed' && (
								<AccessNote
									access={ownerAccess}
									disabled='Точка отключена — менять её нельзя'
									forbidden='Переименовать и отключить точку может владелец'
								/>
							)
						}
					/>
				</SettingsSection>
				{point.data && (
					<TelegramSection
						key={point.data.telegram_chat_id}
						point={point.data}
						editable={chatAccess === 'allowed'}
					/>
				)}
				{ownerAccess !== 'forbidden' && (
					<MigomSection
						pointId={pointId}
						pointName={name}
						access={ownerAccess}
					/>
				)}
				{ownerAccess === 'allowed' && (
					<SettingsSection
						danger
						title='Отключить точку'
						description={disableWarning}
						action={
							<Button
								variant='outlined'
								color='error'
								onClick={() => setConfirming(true)}
								sx={{ flexShrink: 0 }}
							>
								Отключить точку
							</Button>
						}
					/>
				)}
			</Stack>
			<ConfirmDialog
				open={confirming}
				title={`Отключить «${name}»?`}
				description={disableWarning}
				confirmLabel='Отключить точку'
				confirmColor='error'
				pending={disable.isPending}
				error={disable.error?.message}
				onConfirm={() =>
					disable.mutate(undefined, { onSuccess: () => setConfirming(false) })
				}
				onClose={() => {
					disable.reset();
					setConfirming(false);
				}}
			/>
		</PageContent>
	);
}
