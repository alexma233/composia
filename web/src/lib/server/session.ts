import { createHmac, timingSafeEqual } from "node:crypto";
import { verify } from "argon2";

import { env } from "$env/dynamic/private";

const sessionCookieName = "composia_session";
const sessionDurationMs = 1000 * 60 * 60 * 24 * 14;

export type SessionUser = {
  name: string;
};

type SessionPayload = {
  name: string;
  expiresAt: number;
};

export function authConfig() {
  const username = env.WEB_LOGIN_USERNAME?.trim();
  const passwordHash = env.WEB_LOGIN_PASSWORD_HASH?.trim();
  const sessionSecret = env.WEB_SESSION_SECRET?.trim();

  if (!username || !passwordHash || !sessionSecret) {
    return {
      ready: false as const,
      reason:
        "Set WEB_LOGIN_USERNAME, WEB_LOGIN_PASSWORD_HASH, and WEB_SESSION_SECRET in the web server environment.",
    };
  }

  return {
    ready: true as const,
    username,
    passwordHash,
    sessionSecret,
  };
}

export function sessionCookie() {
  return sessionCookieName;
}

export async function authenticate(
  username: string,
  password: string,
): Promise<SessionUser | null> {
  const config = authConfig();
  if (!config.ready) {
    throw new Error(config.reason);
  }

  if (!secureEqual(username.trim(), config.username)) {
    return null;
  }

  const passwordMatches = await verify(config.passwordHash, password);
  if (!passwordMatches) {
    return null;
  }

  return { name: config.username };
}

export function createSessionToken(user: SessionUser) {
  const config = requireAuthConfig();
  const payload: SessionPayload = {
    name: user.name,
    expiresAt: Date.now() + sessionDurationMs,
  };
  const encodedPayload = encodePayload(payload);
  const signature = sign(encodedPayload, config.sessionSecret);
  return `${encodedPayload}.${signature}`;
}

export function readSessionToken(
  token: string | undefined,
): SessionUser | null {
  if (!token) {
    return null;
  }

  const config = authConfig();
  if (!config.ready) {
    return null;
  }

  const separatorIndex = token.lastIndexOf(".");
  if (separatorIndex <= 0 || separatorIndex === token.length - 1) {
    return null;
  }

  const encodedPayload = token.slice(0, separatorIndex);
  const signature = token.slice(separatorIndex + 1);
  const expectedSignature = sign(encodedPayload, config.sessionSecret);
  if (!secureEqual(signature, expectedSignature)) {
    return null;
  }

  const payload = decodePayload(encodedPayload);
  if (
    !payload ||
    payload.expiresAt <= Date.now() ||
    !secureEqual(payload.name, config.username)
  ) {
    return null;
  }

  return { name: payload.name };
}

function requireAuthConfig() {
  const config = authConfig();
  if (!config.ready) {
    throw new Error(config.reason);
  }
  return config;
}

function encodePayload(payload: SessionPayload) {
  return Buffer.from(JSON.stringify(payload), "utf8").toString("base64url");
}

function decodePayload(value: string): SessionPayload | null {
  try {
    const parsed = JSON.parse(
      Buffer.from(value, "base64url").toString("utf8"),
    ) as Partial<SessionPayload>;
    if (
      typeof parsed.name !== "string" ||
      typeof parsed.expiresAt !== "number"
    ) {
      return null;
    }
    return {
      name: parsed.name,
      expiresAt: parsed.expiresAt,
    };
  } catch {
    return null;
  }
}

function sign(value: string, secret: string) {
  return createHmac("sha256", secret).update(value).digest("base64url");
}

function secureEqual(left: string, right: string) {
  const leftBuffer = Buffer.from(left);
  const rightBuffer = Buffer.from(right);
  if (leftBuffer.length !== rightBuffer.length) {
    return false;
  }
  return timingSafeEqual(leftBuffer, rightBuffer);
}
