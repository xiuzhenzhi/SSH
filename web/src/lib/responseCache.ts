export function createResponseCache(maxEntries = 40, now = Date.now) {
  const entries = new Map<string, {data: unknown; time: number}>();
  let epoch = 0;
  return {
    get epoch() { return epoch; },
    get<T = any>(key: string, maxAge = 15000): T | undefined {
      const entry = entries.get(key);
      return entry && now() - entry.time <= maxAge ? entry.data as T : undefined;
    },
    set(key: string, data: unknown, expectedEpoch = epoch) {
      if (expectedEpoch !== epoch) return;
      entries.delete(key);
      entries.set(key, {data, time: now()});
      if (entries.size > maxEntries) entries.delete(entries.keys().next().value!);
    },
    clear() { epoch++; entries.clear(); },
  };
}
