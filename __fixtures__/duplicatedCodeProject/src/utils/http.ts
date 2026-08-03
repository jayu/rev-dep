export const httpClient = {
  buildRequest(endpoint: string, payload: unknown) {
    return { endpoint, payload, headers: { 'content-type': 'application/json' } };
  },
  async send(request: unknown) {
    return { ok: true, status: 200, body: request };
  },
};

// A regex containing braces and a slash, which must not desync the scanner.
export const versionPattern = /v[0-9]{1,3}\/(alpha|beta)/g;

export const templateBlock = `
  .banner { color: red; }
  ${'interpolated'}
`;
