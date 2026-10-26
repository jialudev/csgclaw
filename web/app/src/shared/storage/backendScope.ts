// A hosted UI may connect to different private instances at the same origin.
// Keep their business caches separate; visual preferences remain shared.
export function backendStorageKey(key: string): string {
  const connection = globalThis.CSGCLAW_CONNECTION;
  if (!connection) return key;
  const scope = connection.storage_scope || connection.api_base;
  return `${key}.backend.${encodeURIComponent(scope)}`;
}
