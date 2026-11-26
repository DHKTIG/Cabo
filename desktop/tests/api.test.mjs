import assert from 'node:assert/strict';
import test from 'node:test';

globalThis.location = { hash: '#token=test-token', pathname: '/', search: '' };
let replaced = '';
globalThis.history = {
  replaceState: (_state, _title, url) => {
    replaced = url;
  },
};
let request;
globalThis.fetch = async (url, options) => {
  request = { url, options };
  return {
    json: async () => ({
      ok: false,
      error: { code: 'unauthorized', message: '访问令牌无效', retryable: false },
      requestId: 'request-1',
    }),
  };
};

const { api } = await import('../../internal/server/static/api.mjs');

test('fragment token is removed and sent as a bearer header', async () => {
  const result = await api('/api/state');
  assert.equal(replaced, '/');
  assert.equal(request.url, '/api/state');
  assert.equal(request.options.method, 'GET');
  assert.equal(request.options.headers.Authorization, 'Bearer test-token');
  assert.equal(result.ok, false);
  assert.equal(result.code, 'unauthorized');
  assert.equal(result.error, '访问令牌无效');
  assert.equal(result.requestId, 'request-1');
});
