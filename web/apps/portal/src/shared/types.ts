import type { PortalBFFComponents } from "@token-gateway/api-client";

export type PortalSchemas = PortalBFFComponents["schemas"];
export type APIKey = PortalSchemas["APIKey"];
export type APIKeyCreateResponse = PortalSchemas["APIKeyCreateResponse"];
export type APIKeyRotateResponse = PortalSchemas["APIKeyRotateResponse"];
export type CreditsResponse = PortalSchemas["CreditsResponse"];
export type Dashboard = PortalSchemas["PortalDashboardResponse"];
export type ModelDetail = PortalSchemas["ModelDetailResponse"];
export type ModelList = PortalSchemas["ModelListResponse"];
export type ModelSchema = PortalSchemas["ModelSchemaResponse"];
export type Onboarding = PortalSchemas["OnboardingState"];
export type PlaygroundRunResult = PortalSchemas["PlaygroundRunResult"];
export type ProjectSettings = PortalSchemas["ProjectSettings"];
export type Session = PortalSchemas["PortalSessionResponse"];
export type TaskList = PortalSchemas["TaskListResponse"];
export type TaskObject = PortalSchemas["TaskObject"];
export type UsageResponse = PortalSchemas["UsageResponse"];
