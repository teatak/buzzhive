export const storageKey = "buzzhive-admin-key";

export function request<T>(path: string, token: string, options: RequestInit = {}): Promise<T> {
  return fetch(path, {
    ...options,
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
      ...(options.headers ?? {}),
    },
  }).then(async (response) => {
    if (response.status === 401) throw new Error("unauthorized");
    if (!response.ok) throw new Error(await response.text());
    return response.json() as Promise<T>;
  });
}
