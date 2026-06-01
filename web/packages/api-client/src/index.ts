export {
  ConsoleAPIError,
  createConsoleClient,
  defaultConsoleBaseURL,
  type ConsoleClient,
  type ConsoleClientOptions,
  type ConsoleRequestInit
} from "./client";

export type {
  components as AdminBFFComponents,
  operations as AdminBFFOperations,
  paths as AdminBFFPaths
} from "./generated/admin-bff";
export type {
  components as PortalBFFComponents,
  operations as PortalBFFOperations,
  paths as PortalBFFPaths
} from "./generated/portal-bff";
