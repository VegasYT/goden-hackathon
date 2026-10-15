import type { I18n } from './i18n';
import type { KcContext } from './KcContext';
import type { Template } from './template';

/** Пропсы свёрстанной страницы входа для её `pageId`. */
export interface GodenPageProps<Context extends KcContext> {
	kcContext: Context;
	i18n: I18n;
	Template: typeof Template;
}
