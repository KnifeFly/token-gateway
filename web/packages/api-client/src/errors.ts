export class ConsoleAPIError extends Error {
  readonly status: number;
  readonly code: string;
  readonly requestID?: string;

  constructor(status: number, code: string, message: string, requestID?: string) {
    super(message);
    this.name = "ConsoleAPIError";
    this.status = status;
    this.code = code;
    this.requestID = requestID;
  }
}
