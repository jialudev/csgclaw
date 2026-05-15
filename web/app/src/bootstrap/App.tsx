import { QueryClientProvider } from "@tanstack/react-query";
import { WorkspacePage } from "@/pages/WorkspacePage/WorkspacePage";
import { queryClient } from "@/bootstrap/queryClient";

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <WorkspacePage />
    </QueryClientProvider>
  );
}
