import type { NewDelivery } from '@entities/delivery';
import { shelfLifeDays, type Product } from '@entities/product';
import type { FieldError } from '@shared/api';
import { dayMs } from '@shared/lib/datetime';
import { isExpired } from '@shared/lib/expiry';
import { fromDisplay } from '@shared/lib/quantity';

/** Позиция формы; количество в единице показа (кг, л, шт), как в поле ввода. */
export interface ItemValues {
	key: string;
	productId: string | null;
	quantity: number | null;
	expiresAt: Date | null;
	manufacturedAt: Date | null;
}

interface RegistrationValues {
	number: string;
	deliveredAt: Date | null;
	items: ItemValues[];
}

type ItemField = 'product' | 'quantity' | 'expiresAt' | 'manufacturedAt';
type ItemErrors = Partial<Record<ItemField, string>>;

export interface RegistrationErrors {
	number?: string;
	deliveredAt?: string;
	items: ItemErrors[];
}

/** Наибольшее количество в единице продукта, которое принимает API. */
const maxQuantity = 1e12;

export function emptyItem(): ItemValues {
	return {
		key: crypto.randomUUID(),
		productId: null,
		quantity: null,
		expiresAt: null,
		manufacturedAt: null,
	};
}

export function initialValues(now: Date): RegistrationValues {
	return { number: '', deliveredAt: now, items: [emptyItem()] };
}

const invalid = (date: Date) => Number.isNaN(date.getTime());

function quantityError(
	quantity: number | null,
	product: Product
): string | undefined {
	if (quantity === null) {
		return 'Укажите количество';
	}

	const value = fromDisplay(quantity, product.unit);

	if (value === null) {
		return 'Только целые штуки';
	}

	if (value <= 0) {
		return 'Больше нуля';
	}

	return value > maxQuantity ? 'Слишком большое количество' : undefined;
}

/** Ошибки позиции до отправки; продукт берётся из каталога по `productId`. */
export function itemProblems(item: ItemValues, product?: Product): ItemErrors {
	const errors: ItemErrors = {};
	const quantity = product && quantityError(item.quantity, product);

	if (!product) {
		errors.product = 'Выберите продукт';
	} else if (quantity) {
		errors.quantity = quantity;
	}

	if (!item.expiresAt) {
		errors.expiresAt = 'Укажите срок годности';
	} else if (invalid(item.expiresAt)) {
		errors.expiresAt = 'Неверная дата';
	}

	if (item.manufacturedAt && invalid(item.manufacturedAt)) {
		errors.manufacturedAt = 'Неверная дата';
	} else if (item.manufacturedAt && item.expiresAt && !errors.expiresAt) {
		const life = item.expiresAt.getTime() - item.manufacturedAt.getTime();
		const shelfLife = product?.shelf_life_days;

		if (life < 0) {
			errors.manufacturedAt = 'Позже срока годности';
		} else if (shelfLife !== undefined && life > shelfLife * dayMs) {
			errors.expiresAt = `Дольше срока хранения продукта: до ${shelfLifeDays(shelfLife)}`;
		}
	}

	return errors;
}

/** Ошибки номера и времени поступления. */
export function headerProblems({
	number,
	deliveredAt,
}: Pick<RegistrationValues, 'number' | 'deliveredAt'>): Omit<
	RegistrationErrors,
	'items'
> {
	const deliveredAtError = !deliveredAt
		? 'Укажите время поступления'
		: invalid(deliveredAt)
			? 'Неверная дата'
			: undefined;

	return {
		number: number.trim() ? undefined : 'Укажите номер поставки',
		deliveredAt: deliveredAtError,
	};
}

/** Ошибки всей формы до отправки. */
export function formProblems(
	values: RegistrationValues,
	products: ReadonlyMap<string, Product>
): RegistrationErrors {
	return {
		...headerProblems(values),
		items: values.items.map((item) =>
			itemProblems(
				item,
				item.productId ? products.get(item.productId) : undefined
			)
		),
	};
}

const hasItemErrors = (errors: ItemErrors) => Object.keys(errors).length > 0;

export function hasProblems(errors: RegistrationErrors): boolean {
	return (
		Boolean(errors.number || errors.deliveredAt) ||
		errors.items.some(hasItemErrors)
	);
}

/** Ошибки позиции на экране: свои проверки поверх ошибок сервера. */
export function visibleItemErrors(
	own: ItemErrors | undefined,
	server: ItemErrors | undefined
): ItemErrors {
	return { ...server, ...own };
}

/** Срок позиции уже истёк: её можно зарегистрировать, но при приёмке отклонят. */
export function isItemExpired(item: ItemValues, now: Date): boolean {
	return item.expiresAt !== null && isExpired(item.expiresAt, now);
}

interface RegistrationSummary {
	count: number;
	expired: number;
	/** Позиций, которые нужно исправить. */
	toFix: number;
	headerInvalid: boolean;
}

/** Числа для панели итога: сколько позиций, сколько истекло и сколько исправить. */
export function registrationSummary(
	values: RegistrationValues,
	products: ReadonlyMap<string, Product>,
	now: Date,
	submitted: boolean,
	server?: RegistrationErrors
): RegistrationSummary {
	const own = submitted ? formProblems(values, products) : undefined;

	return {
		count: values.items.length,
		expired: values.items.filter((item) => isItemExpired(item, now)).length,
		toFix: values.items.filter((_, index) =>
			hasItemErrors(visibleItemErrors(own?.items[index], server?.items[index]))
		).length,
		headerInvalid: Boolean(
			own?.number || own?.deliveredAt || server?.number || server?.deliveredAt
		),
	};
}

const serverHeaderFields: Record<string, 'number' | 'deliveredAt'> = {
	number: 'number',
	delivered_at: 'deliveredAt',
};

const serverItemFields: Record<string, ItemField> = {
	product_id: 'product',
	declared_quantity: 'quantity',
	expires_at: 'expiresAt',
	manufactured_at: 'manufacturedAt',
};

/** Ошибки полей из ответа сервера, разложенные по полям формы; `null`, если ни одна не относится к форме. */
export function serverProblems(
	fieldErrors: Readonly<Record<string, FieldError>>,
	itemsCount: number
): RegistrationErrors | null {
	const errors: RegistrationErrors = {
		items: Array.from({ length: itemsCount }, () => ({})),
	};
	let found = false;

	for (const [name, { message }] of Object.entries(fieldErrors)) {
		const header = serverHeaderFields[name];
		const match = /^items\[(\d+)\]\.(\w+)$/.exec(name);
		const itemErrors = match ? errors.items[Number(match[1])] : undefined;
		const field = match ? serverItemFields[match[2]] : undefined;

		if (header) {
			errors[header] = message;
			found = true;
		} else if (itemErrors && field) {
			itemErrors[field] = message;
			found = true;
		}
	}

	return found ? errors : null;
}

/** Тело запроса регистрации; `null`, если в форме не хватает значений. */
export function toNewDelivery(
	values: RegistrationValues,
	products: ReadonlyMap<string, Product>
): NewDelivery | null {
	const items: NewDelivery['items'] = [];

	for (const item of values.items) {
		const product = item.productId ? products.get(item.productId) : undefined;
		const quantity =
			product && item.quantity !== null
				? fromDisplay(item.quantity, product.unit)
				: null;

		if (!product || quantity === null || !item.expiresAt) {
			return null;
		}

		items.push({
			product_id: product.id,
			declared_quantity: quantity,
			expires_at: item.expiresAt.toISOString(),
			...(item.manufacturedAt && {
				manufactured_at: item.manufacturedAt.toISOString(),
			}),
		});
	}

	return values.deliveredAt
		? {
				number: values.number.trim(),
				delivered_at: values.deliveredAt.toISOString(),
				items,
			}
		: null;
}
