// Все иконки приложения подключаются здесь, чтобы набор оставался единым.
import archive from '@material-symbols/svg-400/rounded/archive.svg?react';
import unarchive from '@material-symbols/svg-400/rounded/unarchive.svg?react';
import add from '@material-symbols/svg-400/rounded/add.svg?react';
import addCircle from '@material-symbols/svg-400/rounded/add_circle.svg?react';
import arrowBack from '@material-symbols/svg-400/rounded/arrow_back.svg?react';
import arrowForward from '@material-symbols/svg-400/rounded/arrow_forward.svg?react';
import badge from '@material-symbols/svg-400/rounded/badge.svg?react';
import cancel from '@material-symbols/svg-400/rounded/cancel.svg?react';
import block from '@material-symbols/svg-400/rounded/block.svg?react';
import check from '@material-symbols/svg-400/rounded/check.svg?react';
import checkCircleFill from '@material-symbols/svg-400/rounded/check_circle-fill.svg?react';
import chevronRight from '@material-symbols/svg-400/rounded/chevron_right.svg?react';
import close from '@material-symbols/svg-400/rounded/close.svg?react';
import contentCopy from '@material-symbols/svg-400/rounded/content_copy.svg?react';
import cloudOff from '@material-symbols/svg-400/rounded/cloud_off.svg?react';
import darkMode from '@material-symbols/svg-400/rounded/dark_mode.svg?react';
import deleteIcon from '@material-symbols/svg-400/rounded/delete.svg?react';
import doNotDisturbOn from '@material-symbols/svg-400/rounded/do_not_disturb_on.svg?react';
import eventBusy from '@material-symbols/svg-400/rounded/event_busy.svg?react';
import key from '@material-symbols/svg-400/rounded/key.svg?react';
import keep from '@material-symbols/svg-400/rounded/keep.svg?react';
import keepFill from '@material-symbols/svg-400/rounded/keep-fill.svg?react';
import keepOff from '@material-symbols/svg-400/rounded/keep_off.svg?react';
import edit from '@material-symbols/svg-400/rounded/edit.svg?react';
import editNote from '@material-symbols/svg-400/rounded/edit_note.svg?react';
import error from '@material-symbols/svg-400/rounded/error.svg?react';
import history from '@material-symbols/svg-400/rounded/history.svg?react';
import hourglassBottom from '@material-symbols/svg-400/rounded/hourglass_bottom.svg?react';
import hourglassTop from '@material-symbols/svg-400/rounded/hourglass_top.svg?react';
import inbox from '@material-symbols/svg-400/rounded/inbox.svg?react';
import info from '@material-symbols/svg-400/rounded/info.svg?react';
import inventory2 from '@material-symbols/svg-400/rounded/inventory_2.svg?react';
import keyboardArrowDown from '@material-symbols/svg-400/rounded/keyboard_arrow_down.svg?react';
import lightMode from '@material-symbols/svg-400/rounded/light_mode.svg?react';
import localFireDepartment from '@material-symbols/svg-400/rounded/local_fire_department.svg?react';
import locationOff from '@material-symbols/svg-400/rounded/location_off.svg?react';
import lock from '@material-symbols/svg-400/rounded/lock.svg?react';
import logout from '@material-symbols/svg-400/rounded/logout.svg?react';
import moped from '@material-symbols/svg-400/rounded/moped.svg?react';
import mail from '@material-symbols/svg-400/rounded/mail.svg?react';
import menuBook from '@material-symbols/svg-400/rounded/menu_book.svg?react';
import moreVert from '@material-symbols/svg-400/rounded/more_vert.svg?react';
import moveToInbox from '@material-symbols/svg-400/rounded/move_to_inbox.svg?react';
import personAdd from '@material-symbols/svg-400/rounded/person_add.svg?react';
import personRemove from '@material-symbols/svg-400/rounded/person_remove.svg?react';
import playArrowFill from '@material-symbols/svg-400/rounded/play_arrow-fill.svg?react';
import refresh from '@material-symbols/svg-400/rounded/refresh.svg?react';
import remove from '@material-symbols/svg-400/rounded/remove.svg?react';
import restaurant from '@material-symbols/svg-400/rounded/restaurant.svg?react';
import schedule from '@material-symbols/svg-400/rounded/schedule.svg?react';
import search from '@material-symbols/svg-400/rounded/search.svg?react';
import searchOff from '@material-symbols/svg-400/rounded/search_off.svg?react';
import sort from '@material-symbols/svg-400/rounded/sort.svg?react';
import stopCircle from '@material-symbols/svg-400/rounded/stop_circle.svg?react';
import storefront from '@material-symbols/svg-400/rounded/storefront.svg?react';
import swapHoriz from '@material-symbols/svg-400/rounded/swap_horiz.svg?react';
import taskAlt from '@material-symbols/svg-400/rounded/task_alt.svg?react';
import tune from '@material-symbols/svg-400/rounded/tune.svg?react';
import unfoldMore from '@material-symbols/svg-400/rounded/unfold_more.svg?react';
import visibility from '@material-symbols/svg-400/rounded/visibility.svg?react';
import visibilityOff from '@material-symbols/svg-400/rounded/visibility_off.svg?react';
import warning from '@material-symbols/svg-400/rounded/warning.svg?react';
import type { ComponentType, SVGProps } from 'react';

import {
	NavDashboardOutline,
	NavDashboardFilled,
	NavStockOutline,
	NavStockFilled,
	NavDeliveriesOutline,
	NavDeliveriesFilled,
	NavOrdersOutline,
	NavOrdersFilled,
	NavKitchenOutline,
	NavKitchenFilled,
	NavStopListOutline,
	NavStopListFilled,
	NavAnalyticsOutline,
	NavAnalyticsFilled,
	NavGeneratorOutline,
	NavGeneratorFilled,
	NavStaffOutline,
	NavStaffFilled,
	NavPointSettingsOutline,
	NavPointSettingsFilled,
	NavCompanyOutline,
	NavCompanyFilled,
	NavCatalogOutline,
	NavCatalogFilled,
} from './nav-icons.gen';

type SvgComponent = ComponentType<SVGProps<SVGSVGElement>>;

interface IconEntry {
	outline: SvgComponent;
	filled?: SvgComponent;
}

export const icons = {
	// Навигация: у активного пункта заливная версия, части рисунка двигаются при наведении (navIconMotion).
	dashboard: { outline: NavDashboardOutline, filled: NavDashboardFilled },
	stock: { outline: NavStockOutline, filled: NavStockFilled },
	deliveries: { outline: NavDeliveriesOutline, filled: NavDeliveriesFilled },
	orders: { outline: NavOrdersOutline, filled: NavOrdersFilled },
	kitchen: { outline: NavKitchenOutline, filled: NavKitchenFilled },
	stopList: { outline: NavStopListOutline, filled: NavStopListFilled },
	analytics: { outline: NavAnalyticsOutline, filled: NavAnalyticsFilled },
	generator: { outline: NavGeneratorOutline, filled: NavGeneratorFilled },
	staff: { outline: NavStaffOutline, filled: NavStaffFilled },
	pointSettings: {
		outline: NavPointSettingsOutline,
		filled: NavPointSettingsFilled,
	},
	company: { outline: NavCompanyOutline, filled: NavCompanyFilled },
	catalog: { outline: NavCatalogOutline, filled: NavCatalogFilled },
	// Действия и состояния.
	add: { outline: add },
	remove: { outline: remove },
	check: { outline: check },
	close: { outline: close },
	more: { outline: moreVert },
	refresh: { outline: refresh },
	history: { outline: history },
	lock: { outline: lock },
	logout: { outline: logout },
	info: { outline: info },
	warning: { outline: warning },
	error: { outline: error },
	expandMore: { outline: keyboardArrowDown },
	chevronRight: { outline: chevronRight },
	search: { outline: search },
	filters: { outline: tune },
	sort: { outline: sort },
	edit: { outline: edit },
	archive: { outline: archive },
	unarchive: { outline: unarchive },
	arrowBack: { outline: arrowBack },
	arrowForward: { outline: arrowForward },
	play: { outline: playArrowFill },
	stop: { outline: stopCircle },
	darkMode: { outline: darkMode },
	lightMode: { outline: lightMode },
	cloudOff: { outline: cloudOff },
	done: { outline: checkCircleFill },
	// Точки.
	point: { outline: storefront },
	switchPoint: { outline: unfoldMore },
	pointUnavailable: { outline: locationOff },
	pointDisabled: { outline: block },
	employee: { outline: badge },
	personAdd: { outline: personAdd },
	personRemove: { outline: personRemove },
	transfer: { outline: swapHoriz },
	// Поставщики и их ключи.
	key: { outline: key },
	copy: { outline: contentCopy },
	// Исходы приёмки.
	toStock: { outline: moveToInbox },
	rejected: { outline: doNotDisturbOn },
	quarantine: { outline: hourglassTop },
	// События поставки и изменения склада.
	registered: { outline: inbox },
	approved: { outline: taskAlt },
	quarantineResolved: { outline: hourglassBottom },
	corrected: { outline: editNote },
	stockIn: { outline: addCircle },
	stockOut: { outline: doNotDisturbOn },
	// Движения по продукту на складе.
	consumed: { outline: restaurant },
	writtenOff: { outline: deleteIcon },
	expiryWarning: { outline: schedule },
	expired: { outline: eventBusy },
	// Горячие предложения.
	pin: { outline: keep, filled: keepFill },
	unpin: { outline: keepOff },
	hide: { outline: visibilityOff },
	// Кухня.
	recipe: { outline: menuBook },
	// Заказы.
	manualOrder: { outline: editNote },
	hotDish: { outline: localFireDepartment },
	cancelOrder: { outline: cancel },
	migom: { outline: moped },
	// Пустые состояния.
	empty: { outline: inventory2 },
	notFound: { outline: searchOff },
	// Страницы входа.
	showPassword: { outline: visibility },
	hidePassword: { outline: visibilityOff },
	mail: { outline: mail },
} satisfies Record<string, IconEntry>;

export type IconName = keyof typeof icons;
