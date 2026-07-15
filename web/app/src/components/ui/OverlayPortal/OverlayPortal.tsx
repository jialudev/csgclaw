import { useState } from "react";
import type { ReactNode } from "react";
import { OverlayPortalContainerContext } from "./OverlayPortalContext";
import type { OverlayPortalContainer } from "./OverlayPortalContext";

export type OverlayPortalProviderProps = {
  children: ReactNode;
  container?: OverlayPortalContainer;
};

export function OverlayPortalProvider({ children, container }: OverlayPortalProviderProps) {
  const [defaultContainer, setDefaultContainer] = useState<HTMLDivElement | null>(null);
  const usesDefaultContainer = container === undefined;

  return (
    <OverlayPortalContainerContext.Provider value={usesDefaultContainer ? defaultContainer : container}>
      {children}
      {usesDefaultContainer ? <div data-csg-overlay-root="" ref={setDefaultContainer} /> : null}
    </OverlayPortalContainerContext.Provider>
  );
}
