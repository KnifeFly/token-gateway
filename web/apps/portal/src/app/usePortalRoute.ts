import { useEffect, useState } from "react";

import { navItems, type ViewID } from "./routes";

function viewFromLocation(): ViewID {
  const path = window.location.pathname;
  return navItems.find((item) => path.startsWith(item.route))?.id ?? "dashboard";
}

export function usePortalRoute() {
  const [activeView, setActiveView] = useState<ViewID>(() => viewFromLocation());

  useEffect(() => {
    function syncViewFromHistory() {
      setActiveView(viewFromLocation());
    }
    window.addEventListener("popstate", syncViewFromHistory);
    return () => {
      window.removeEventListener("popstate", syncViewFromHistory);
    };
  }, []);

  function navigateView(viewID: ViewID) {
    const item = navItems.find((navItem) => navItem.id === viewID) ?? navItems[0];
    setActiveView(item.id);
    window.history.pushState({}, "", item.route);
  }

  return { activeView, navigateView };
}
