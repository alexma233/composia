import { timingSafeEqual } from "node:crypto";

type Argon2Name = "Argon2d" | "Argon2i" | "Argon2id";

interface Argon2SubtleCrypto {
  importKey(
    format: "raw-secret",
    keyData: BufferSource,
    algorithm: Argon2Name,
    extractable: false,
    keyUsages: ["deriveBits"],
  ): Promise<CryptoKey>;
  deriveBits(
    algorithm: {
      name: Argon2Name;
      memory: number;
      passes: number;
      parallelism: number;
      nonce: BufferSource;
    },
    baseKey: CryptoKey,
    length: number,
  ): Promise<ArrayBuffer>;
}

export async function verifyArgon2(
  encodedHash: string,
  password: string,
): Promise<boolean> {
  const { name, memory, passes, parallelism, salt, expected } =
    parseArgon2Hash(encodedHash);
  const subtle = crypto.subtle as unknown as Argon2SubtleCrypto;
  const key = await subtle.importKey(
    "raw-secret",
    new TextEncoder().encode(password),
    name,
    false,
    ["deriveBits"],
  );
  const derived = new Uint8Array(
    await subtle.deriveBits(
      { name, memory, passes, parallelism, nonce: salt },
      key,
      expected.byteLength * 8,
    ),
  );

  return timingSafeEqual(derived, expected);
}

function parseArgon2Hash(encodedHash: string) {
  const [, variant, version, encodedParams, encodedSalt, encodedDigest, extra] =
    encodedHash.split("$");
  const name = argon2Names[variant];
  if (
    extra !== undefined ||
    !name ||
    version !== "v=19" ||
    !encodedParams ||
    !encodedSalt ||
    !encodedDigest
  ) {
    throw new Error("WEB_LOGIN_PASSWORD_HASH must be a valid Argon2 PHC hash.");
  }

  const params = Object.fromEntries(
    encodedParams.split(",").map((entry) => entry.split("=", 2)),
  );
  const memory = positiveInteger(params.m);
  const passes = positiveInteger(params.t);
  const parallelism = positiveInteger(params.p);
  if (!memory || !passes || !parallelism) {
    throw new Error("WEB_LOGIN_PASSWORD_HASH has invalid Argon2 parameters.");
  }

  return {
    name,
    memory,
    passes,
    parallelism,
    salt: decodeBase64(encodedSalt),
    expected: decodeBase64(encodedDigest),
  };
}

const argon2Names: Record<string, Argon2Name> = {
  argon2d: "Argon2d",
  argon2i: "Argon2i",
  argon2id: "Argon2id",
};

function positiveInteger(value: string | undefined) {
  const parsed = Number(value);
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : null;
}

function decodeBase64(value: string) {
  try {
    return Uint8Array.from(atob(value), (character) => character.charCodeAt(0));
  } catch {
    throw new Error("WEB_LOGIN_PASSWORD_HASH contains invalid base64 data.");
  }
}
