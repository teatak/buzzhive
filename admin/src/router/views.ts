import type { View } from "../types/admin";

export const viewFromHash = (): View => {
  switch (window.location.hash.replace("#", "")) {
    case "users":
      return "users";
    case "my-api-keys":
      return "myKeys";
    case "google-accounts":
      return "accounts";
    case "runtime":
      return "runtime";
    default:
      return "dashboard";
  }
};

export const hashForView = (view: View) => {
  switch (view) {
    case "users":
      return "users";
    case "myKeys":
      return "my-api-keys";
    case "accounts":
      return "google-accounts";
    case "runtime":
      return "runtime";
    default:
      return "dashboard";
  }
};
