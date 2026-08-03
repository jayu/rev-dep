import { httpClient } from '../utils/http';
import type { Team } from '../utils/types';

// Copy-pasted from userApi with only the cast changed - literal mode should
// NOT match this, structural mode should.
export async function fetchTeam(endpoint: string, payload: unknown): Promise<Team> {
  const req = httpClient.buildRequest(endpoint, payload);
  const res = await httpClient.send(req);
  if (!res.ok) {
    throw new TransportError(res.status, res.body);
  }
  return res.body as Team;
}

export const teamDefaults = {
  displayNamePreferenceForHeaderArea: 'Anonymous Contributor Account',
  notificationEmailDeliveryAddress: 'no-reply@example-company.test',
  preferredInterfaceLanguageCode: 'en-GB',
};
