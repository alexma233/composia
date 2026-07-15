export const decryptedSecretResponseHeaders = {
  "Cache-Control": "private, no-store",
};

type SetHeaders = (headers: Record<string, string>) => void;

export function decryptedSecretResponseInit(enabled: boolean) {
  return enabled ? { headers: decryptedSecretResponseHeaders } : undefined;
}

export function setDecryptedSecretResponseHeaders(
  setHeaders: SetHeaders,
  enabled: boolean,
) {
  if (enabled) {
    setHeaders(decryptedSecretResponseHeaders);
  }
}
