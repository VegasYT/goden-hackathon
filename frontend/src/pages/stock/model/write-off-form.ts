import { useForm, useSelector } from '@tanstack/react-form';
import { useState } from 'react';

import type { Batch } from '../api/stock';
import { initialWriteOff, type WriteOffValues } from './actions';

/** Форма списания: начальные значения по открытой партии считаются один раз. */
export function useWriteOffForm(batch: Batch, now: Date) {
	const [defaultValues] = useState<WriteOffValues>(() =>
		initialWriteOff(batch, now)
	);
	const form = useForm({ defaultValues });
	const values = useSelector(form.store, (state) => state.values);

	return { form, values };
}
