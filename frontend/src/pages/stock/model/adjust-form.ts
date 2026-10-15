import { useForm, useSelector } from '@tanstack/react-form';

import type { AdjustValues } from './actions';

const defaultValues: AdjustValues = { actual: null, reason: '', comment: '' };

/** Форма исправления остатка: фактическое количество, причина и комментарий. */
export function useAdjustForm() {
	const form = useForm({ defaultValues });
	const values = useSelector(form.store, (state) => state.values);

	return { form, values };
}
