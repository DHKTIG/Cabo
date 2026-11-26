const apiToken = new URLSearchParams(location.hash.slice(1)).get('token') || '';
if (apiToken) history.replaceState(null, '', location.pathname + location.search);

export async function api(path, body) {
  const headers = { 'Content-Type': 'application/json' };
  if (apiToken) headers.Authorization = `Bearer ${apiToken}`;
  const res = await fetch(path, {
    method: body === undefined ? 'GET' : 'POST',
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const data = await res.json();
  if (data && data.error && typeof data.error === 'object') {
    return { ...data, error: data.error.message, code: data.error.code };
  }
  return data;
}
