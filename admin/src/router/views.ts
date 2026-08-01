import type { View } from "../types/admin";

export const viewFromHash = (): View => {
  const hash = window.location.hash.replace("#", "");
  if (/^users\/\d+$/.test(hash)) return "userDetail";
  switch (hash) {
    case "users":
      return "users";
    case "my-api-keys":
      return "myKeys";
    case "providers":
      return "providers";
    case "provider-keys":
      return "providers";
    case "models":
    case "hub":
      return "models";
    default:
      return "dashboard";
  }
};

export const userIDFromHash = (): number => {
  const match = window.location.hash.replace("#", "").match(/^users\/(\d+)$/);
  return match ? Number(match[1]) : 0;
};

export const hashForUser = (userID: number) => `users/${userID}`;

export const hashForView = (view: View) => {
  switch (view) {
    case "users":
    case "userDetail":
      return "users";
    case "myKeys":
      return "my-api-keys";
    case "providers":
      return "providers";
    case "models":
      return "models";
    default:
      return "dashboard";
  }
};
