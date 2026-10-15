import { ReadOnlyNote } from '@shared/ui/read-only-note';

import type { WriteAccess } from '../model/profile';

interface AccessNoteProps {
	access: WriteAccess;
	/** Пояснение на отключённой точке. */
	disabled: string;
	/** Пояснение, когда у пользователя нет права. */
	forbidden: string;
}

/** Почему записывать нельзя: на отключённой точке один текст, без права другой; при доступе ничего. */
export function AccessNote({ access, disabled, forbidden }: AccessNoteProps) {
	if (access === 'allowed') {
		return null;
	}

	return (
		<ReadOnlyNote>{access === 'disabled' ? disabled : forbidden}</ReadOnlyNote>
	);
}
