import { httpClient } from '../utils/http';
import type { User } from '../utils/types';

export async function fetchUser(endpoint: string, payload: unknown): Promise<User> {
  const request = httpClient.buildRequest(endpoint, payload);
  const response = await httpClient.send(request);
  if (!response.ok) {
    throw new TransportError(response.status, response.body);
  }
  return response.body as User;
}

export const userDefaults = {
  displayNamePreferenceForHeaderArea: 'Anonymous Contributor Account',
  notificationEmailDeliveryAddress: 'no-reply@example-company.test',
  preferredInterfaceLanguageCode: 'en-GB',
};
