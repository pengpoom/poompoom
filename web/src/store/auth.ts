"use client";

import localforage from "localforage";

const LEGACY_AUTH_KEY_STORAGE_KEY = "imagestudio_auth_key";
export const AUTH_ROLE_STORAGE_KEY = "imagestudio_auth_role";
export const AUTH_USERNAME_STORAGE_KEY = "imagestudio_auth_username";
export const AUTH_AVATAR_URL_STORAGE_KEY = "imagestudio_auth_avatar_url";
export const AUTH_STATE_CHANGED_EVENT = "image-studio:auth-state-changed";

export type AuthRole = "user" | "admin";

let intentionalLogoutUntil = 0;

const authStorage = localforage.createInstance({
  name: "imagestudio",
  storeName: "auth",
});

export async function getStoredAuthKey() {
  return "";
}

export async function setStoredAuthKey(_authKey: string) {
  if (typeof window === "undefined") {
    return;
  }
  await authStorage.removeItem(LEGACY_AUTH_KEY_STORAGE_KEY);
}

export async function getStoredAuthRole(): Promise<AuthRole | null> {
  if (typeof window === "undefined") {
    return null;
  }
  const value = await authStorage.getItem<string>(AUTH_ROLE_STORAGE_KEY);
  return value === "admin" || value === "user" ? value : null;
}

export async function setStoredAuthRole(role: AuthRole | null) {
  if (typeof window === "undefined") {
    return;
  }
  if (!role) {
    await authStorage.removeItem(AUTH_ROLE_STORAGE_KEY);
    return;
  }
  intentionalLogoutUntil = 0;
  await authStorage.setItem(AUTH_ROLE_STORAGE_KEY, role);
  notifyAuthStateChanged();
}

export async function getStoredAuthUsername() {
  if (typeof window === "undefined") {
    return "";
  }
  const value = await authStorage.getItem<string>(AUTH_USERNAME_STORAGE_KEY);
  return String(value || "").trim();
}

export async function setStoredAuthUsername(username: string | null) {
  if (typeof window === "undefined") {
    return;
  }
  const normalizedUsername = String(username || "").trim();
  if (!normalizedUsername) {
    await authStorage.removeItem(AUTH_USERNAME_STORAGE_KEY);
    return;
  }
  await authStorage.setItem(AUTH_USERNAME_STORAGE_KEY, normalizedUsername);
}

export async function getStoredAuthAvatarUrl() {
  if (typeof window === "undefined") {
    return "";
  }
  const value = await authStorage.getItem<string>(AUTH_AVATAR_URL_STORAGE_KEY);
  return String(value || "").trim();
}

export async function setStoredAuthAvatarUrl(avatarUrl: string | null) {
  if (typeof window === "undefined") {
    return;
  }
  const normalizedAvatarUrl = String(avatarUrl || "").trim();
  if (!normalizedAvatarUrl) {
    await authStorage.removeItem(AUTH_AVATAR_URL_STORAGE_KEY);
    notifyAuthStateChanged();
    return;
  }
  await authStorage.setItem(AUTH_AVATAR_URL_STORAGE_KEY, normalizedAvatarUrl);
  notifyAuthStateChanged();
}

export async function clearStoredAuthKey() {
  if (typeof window === "undefined") {
    return;
  }
  await authStorage.removeItem(LEGACY_AUTH_KEY_STORAGE_KEY);
  await authStorage.removeItem(AUTH_ROLE_STORAGE_KEY);
  await authStorage.removeItem(AUTH_USERNAME_STORAGE_KEY);
  await authStorage.removeItem(AUTH_AVATAR_URL_STORAGE_KEY);
  notifyAuthStateChanged();
}

export function beginIntentionalLogout() {
  intentionalLogoutUntil = Date.now() + 15000;
}

export function finishIntentionalLogout() {
  intentionalLogoutUntil = Math.max(intentionalLogoutUntil, Date.now() + 10000);
}

export function isIntentionalLogoutInProgress() {
  return Date.now() < intentionalLogoutUntil;
}

export function isAuthErrorDuringIntentionalLogout(error: unknown) {
  if (!isIntentionalLogoutInProgress() || typeof error !== "object" || error === null) {
    return false;
  }
  const status = (error as { status?: unknown }).status;
  const message = String((error as { message?: unknown }).message || "");
  return Number(status) === 401 || /authorization is invalid/i.test(message);
}

function notifyAuthStateChanged() {
  if (typeof window === "undefined") {
    return;
  }
  window.dispatchEvent(new CustomEvent(AUTH_STATE_CHANGED_EVENT));
}
