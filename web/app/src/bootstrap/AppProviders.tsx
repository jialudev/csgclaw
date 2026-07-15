import { QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { GlobalScrollbarController } from "@/bootstrap/GlobalScrollbarController";
import { queryClient } from "@/bootstrap/queryClient";
import { OverlayPortalProvider } from "@/components/ui/OverlayPortal";

export function AppProviders({ children }: { children: ReactNode }) {
  return (
    <OverlayPortalProvider>
      <QueryClientProvider client={queryClient}>
        <GlobalScrollbarController />
        {children}
      </QueryClientProvider>
    </OverlayPortalProvider>
  );
}
