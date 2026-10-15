import * as z from 'zod/mini';

/** Состояние карточки поставки в адресе: открытая боковая панель. */
export const deliverySearchSchema = z.object({
	panel: z.catch(z.optional(z.enum(['history'])), undefined),
});

export type DeliveryPanel = z.output<typeof deliverySearchSchema>['panel'];

/** Значения по умолчанию: их нет в адресе. */
export const deliverySearchDefaults = deliverySearchSchema.parse({});
