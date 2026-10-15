import { Button } from '@mui/material';

import type { Point } from '@entities/organization';
import { InfoNote } from '@shared/ui/info-note';
import { SettingsSection } from '@shared/ui/settings-section';

import { useSetTelegramChat } from '../api/point-writes';
import { parseChatId } from '../model/chat-id';
import { ConnectionStatus, NumberConnectForm } from './number-connection';

interface TelegramSectionProps {
	point: Point;
	/** Можно ли менять чат: есть право и точка работает. */
	editable: boolean;
}

/** Чат Telegram, в который уходят уведомления точки. */
export function TelegramSection({ point, editable }: TelegramSectionProps) {
	const chatId = point.telegram_chat_id;
	const mutation = useSetTelegramChat(point.id);
	const disconnecting = mutation.isPending && mutation.variables === undefined;

	return (
		<SettingsSection
			title='Уведомления в Telegram'
			description='В чат точки приходят предупреждения об истекающих партиях и вопросы, что делать с просроченными.'
		>
			<ConnectionStatus
				connected={chatId !== undefined}
				on='подключены'
				off='не подключены'
				detail={`в чат ${chatId}`}
			/>
			{editable && (
				<>
					<NumberConnectForm
						label='Номер чата'
						placeholder='-100…'
						invalidText='Нужен номер чата: целое число, например −1001846203517'
						parse={parseChatId}
						current={chatId ?? null}
						pending={mutation.isPending && !disconnecting}
						error={mutation.error?.message}
						onSubmit={(next) => mutation.mutate(next)}
					>
						{chatId !== undefined && (
							<Button
								onClick={() => mutation.mutate(undefined)}
								loading={disconnecting}
								sx={{ mt: 3, color: 'text.secondary' }}
							>
								Отключить уведомления
							</Button>
						)}
					</NumberConnectForm>
					<InfoNote>
						Добавьте бота уведомлений Goden в группу точки и вставьте сюда номер
						группы. Без чата точка уведомлений не получает.
					</InfoNote>
				</>
			)}
		</SettingsSection>
	);
}
