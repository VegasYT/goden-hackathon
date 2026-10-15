import Keycloak from 'keycloak-js';

import { keycloakConfig } from '@shared/config';

export const keycloak = new Keycloak(keycloakConfig);
