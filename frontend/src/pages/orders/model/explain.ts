import { portionsText, type HotDish, type HotReason } from '@entities/dish';
import { plural } from '@shared/lib/plural';
import { formatAmount } from '@shared/lib/quantity';

/** Сколько продукта под риском уйдёт с рекомендованными порциями блюда. */
function consumption(dish: HotDish, reason: HotReason) {
	const unit = reason.product.unit;
	const perPortion = formatAmount(reason.required_quantity, unit);

	// Сервер снижает вклад продукта, которого под риском меньше порции: об этом и пишем.
	if (reason.significance < 1) {
		return `Меньше порции: на порцию нужно ${perPortion}, поэтому блюдо ниже в списке`;
	}

	const sold = dish.suggested_portions;
	const used = Math.min(
		sold * reason.required_quantity,
		reason.at_risk_quantity
	);
	const spent =
		used >= reason.at_risk_quantity
			? 'всё'
			: `${formatAmount(used, unit)} из ${formatAmount(reason.at_risk_quantity, unit)}`;

	return `На порцию ${perPortion}: ${portionsText(sold)} израсходуют ${spent}`;
}

function otherDishes(reason: HotReason) {
	const others = reason.dishes_using - 1;

	return others < 1
		? 'используется только в этом блюде'
		: `используется ещё в ${others} ${plural(others, 'блюде', 'блюдах', 'блюдах')}`;
}

/** Почему продукт делает блюдо горячим предложением: расход порциями и другие блюда с ним. */
export function explainReason(dish: HotDish, reason: HotReason): string {
	return `${consumption(dish, reason)} · ${otherDishes(reason)}`;
}
