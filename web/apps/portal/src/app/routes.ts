export type ViewID =
  | "dashboard"
  | "models"
  | "credits"
  | "usage"
  | "api-keys"
  | "tasks"
  | "onboarding"
  | "settings";

export const navItems: Array<{ id: ViewID; label: string }> = [
  { id: "dashboard", label: "Dashboard" },
  { id: "models", label: "Models" },
  { id: "credits", label: "Credits" },
  { id: "usage", label: "Usage" },
  { id: "api-keys", label: "API Keys" },
  { id: "tasks", label: "Tasks" },
  { id: "onboarding", label: "Onboarding" },
  { id: "settings", label: "Settings" }
];
