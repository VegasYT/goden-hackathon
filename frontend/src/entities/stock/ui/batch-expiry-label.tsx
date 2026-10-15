import { ExpiryLabel } from '@shared/ui/expiry-label';

interface BatchExpiryLabelProps {
	expiresAt: string;
	now: Date;
	/** Дата «годен до» рядом с оставшимся сроком и части в одну строку, для карточек. */
	inline?: boolean;
}

/** Срок истекающей партии: сколько осталось, у просроченной — «просрочена» и что её нужно списать. */
export function BatchExpiryLabel({
	expiresAt,
	now,
	inline = false,
}: BatchExpiryLabelProps) {
	return (
		<ExpiryLabel
			expiresAt={expiresAt}
			now={now}
			lead='remaining'
			withDate={inline}
			inline={inline}
			expiredLabel='просрочена'
			expiredNote='нужно списать'
		/>
	);
}
