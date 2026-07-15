import { createContext, useContext } from "react";

export type OverlayPortalContainer = Element | DocumentFragment | null;

export const OverlayPortalContainerContext = createContext<OverlayPortalContainer>(null);

export function useOverlayPortalContainer(container?: OverlayPortalContainer) {
  const defaultContainer = useContext(OverlayPortalContainerContext);
  return container === undefined ? defaultContainer : container;
}
