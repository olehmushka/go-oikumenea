// Neutral (non server-only) error type so both server and client components can reference it.

export class ApiError extends Error {
  constructor(
    public status: number,
    public body: unknown,
  ) {
    super(
      typeof body === "object" && body && "errorName" in body
        ? String((body as { errorName?: string }).errorName)
        : `API error ${status}`,
    );
    this.name = "ApiError";
  }
}
