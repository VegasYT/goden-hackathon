import { TextLink } from '@shared/ui/link';

import type { Product } from '../api/product-queries';

type ProductHistoryLinkProps = { product: Pick<Product, 'id' | 'name'> } & (
	| {
			pointId: string;
			/** Без права на склад точки название показывается текстом. */
			linked: boolean;
	  }
	/** На складе история открывается поверх текущей вкладки, а не на «Остатках». */
	| { inPlace: true }
);

/** Название продукта со ссылкой на его историю на складе точки: выглядит как текст, подчёркивается при наведении. */
export function ProductHistoryLink(props: ProductHistoryLinkProps) {
	const { product } = props;

	if ('linked' in props && !props.linked) {
		return <>{product.name}</>;
	}

	return 'inPlace' in props ? (
		<TextLink
			to='.'
			search={(prev: Record<string, unknown>) => ({
				...prev,
				history: product.id,
			})}
			color='inherit'
			underline='hover'
		>
			{product.name}
		</TextLink>
	) : (
		<TextLink
			to='/points/$pointId/stock'
			params={{ pointId: props.pointId }}
			search={{ history: product.id }}
			color='inherit'
			underline='hover'
		>
			{product.name}
		</TextLink>
	);
}
