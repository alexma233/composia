export const decryptedSecretResponseHeaders = {
  "Cache-Control": "private, no-store",
};

export function decryptedSecretResponseInit(enabled: boolean) {
  return enabled ? { headers: decryptedSecretResponseHeaders } : undefined;
}
