import { render, type RenderOptions } from '@testing-library/react';
import type { ReactElement } from 'react';

import { ThemeRoot } from '@shared/ui/theme';

/** Рендер компонента в той же теме, что и приложение. */
export function renderWithTheme(ui: ReactElement, options?: RenderOptions) {
	return render(ui, { wrapper: ThemeRoot, ...options });
}
