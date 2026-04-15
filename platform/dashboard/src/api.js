// Small fetch helper. All API calls return parsed JSON or throw.

const BASE = '';

export async function api(path, opts = {}) {
  const res = await fetch(BASE + path, {
    headers: { 'content-type': 'application/json', ...(opts.headers || {}) },
    ...opts,
  });
  const text = await res.text();
  let body;
  try { body = text ? JSON.parse(text) : null; } catch { body = text; }
  if (!res.ok) {
    const msg = (body && body.error) || `HTTP ${res.status}`;
    throw new Error(msg);
  }
  return body;
}

export const apiGet = (path) => api(path);
export const apiPost = (path, data) => api(path, { method: 'POST', body: JSON.stringify(data) });
