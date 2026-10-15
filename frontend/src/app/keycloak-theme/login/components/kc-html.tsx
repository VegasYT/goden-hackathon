import { kcSanitize } from 'keycloakify/lib/kcSanitize';

/** Текст Keycloak с разметкой: перед вставкой его очищает `kcSanitize`. */
export function KcHtml({ html }: { html: string }) {
	return <span dangerouslySetInnerHTML={{ __html: kcSanitize(html) }} />;
}
