import type { View } from "../types/admin";

export const viewFromHash = (): View => {
  switch (window.location.hash.replace("#", "")) {
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

export const hashForView = (view: View) => {
  switch (view) {
    case "users":
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
