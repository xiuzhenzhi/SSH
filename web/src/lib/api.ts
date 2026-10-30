export class HTTPError extends Error {
  status: number;
  constructor(status: number, message: string) { super(message); this.status = status; }
}

// One deadline covers both response headers and body; caller cancellation still works.
export async function requestJSON(path: string, method = 'GET', body?: unknown, signal?: AbortSignal, timeout = method === 'GET' ? 10000 : 30000) {
  const controller = new AbortController();
  const cancel = () => controller.abort();
  if (signal?.aborted) cancel();
  else signal?.addEventListener('abort', cancel, {once: true});
  const timer = setTimeout(cancel, timeout);
  try {
    const response = await fetch('/api/v1' + path, {
      method, signal: controller.signal, credentials: 'same-origin',
      headers: {'Content-Type': 'application/json'},
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    const data = await response.json().catch((e) => {
      if (controller.signal.aborted) throw e;
      throw new HTTPError(response.status, `主控返回异常响应（HTTP ${response.status}）`);
    });
    if (!response.ok) throw new HTTPError(response.status, data?.error || `HTTP ${response.status}`);
    if (!data || typeof data !== 'object' || !('data' in data)) throw new Error('主控返回的数据格式异常，请刷新后重试');
    return data.data;
  } catch (e) {
    if (controller.signal.aborted && !signal?.aborted) throw new Error('连接超时，请检查网络或主控服务后重试');
    throw e;
  } finally {
    clearTimeout(timer);
    signal?.removeEventListener('abort', cancel);
  }
}
