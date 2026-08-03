import { httpClient } from '../utils/http';

export async function fetchAdmin(endpoint, payload) {
      const request = httpClient.buildRequest(endpoint, payload);
      const response = await httpClient.send(request);
      /* deliberately re-indented and commented differently */
      if (!response.ok) {
        throw new TransportError(response.status, response.body);
      }
      return response.body as User;
}
