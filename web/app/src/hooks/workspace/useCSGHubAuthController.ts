import { useCallback, useEffect, useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { beginCSGHubAuthLogin, fetchCSGHubAuthStatus, logoutCSGHubAuth } from "@/api/csghubAuth";
import { errorMessage } from "@/api/client";
import {
  emptyCSGHubAuthStatus,
  isCSGHubAuthenticated,
  normalizeCSGHubAuthStatus,
  normalizeCSGHubLoginResponse,
} from "@/models/csghubAuth";
import type { CSGHubAuthStatus } from "@/models/csghubAuth";
import type { TranslateFn } from "@/models/conversations";
import { workspaceQueryKeys } from "./workspaceQueries";

const LOGIN_POLL_INTERVAL_MS = 2000;
const LOGIN_POLL_TIMEOUT_MS = 120000;

export type CSGHubAuthController = {
  csghubAuthBusy: boolean;
  csghubAuthError: string;
  csghubAuthPending: boolean;
  csghubAuthStatus: CSGHubAuthStatus;
  loginCSGHub: () => Promise<void>;
  logoutCSGHub: () => Promise<void>;
};

export function useCSGHubAuthController(t: TranslateFn): CSGHubAuthController {
  const queryClient = useQueryClient();
  const [busyAction, setBusyAction] = useState<"login" | "logout" | "">("");
  const [authError, setAuthError] = useState("");
  const [loginPending, setLoginPending] = useState(false);

  const statusQuery = useQuery({
    queryKey: workspaceQueryKeys.csghubAuthStatus(),
    queryFn: fetchNormalizedCSGHubAuthStatus,
    retry: 0,
  });

  const status = useMemo(() => statusQuery.data ?? emptyCSGHubAuthStatus(), [statusQuery.data]);

  const setStatus = useCallback(
    (next: CSGHubAuthStatus) => {
      queryClient.setQueryData(workspaceQueryKeys.csghubAuthStatus(), next);
    },
    [queryClient],
  );

  const refreshStatus = useCallback(async () => {
    return queryClient.fetchQuery({
      queryKey: workspaceQueryKeys.csghubAuthStatus(),
      queryFn: fetchNormalizedCSGHubAuthStatus,
      retry: 0,
    });
  }, [queryClient]);

  const loginCSGHub = useCallback(async () => {
    if (busyAction) {
      return;
    }
    setBusyAction("login");
    setAuthError("");
    try {
      const login = normalizeCSGHubLoginResponse(await beginCSGHubAuthLogin(window.location.href));
      if (!login.login_url) {
        throw new Error(t("csghubLoginURLMissing"));
      }
      setLoginPending(true);
      window.location.assign(login.login_url);
    } catch (err) {
      setLoginPending(false);
      setAuthError(errorMessage(err, t("csghubLoginFailed")));
    } finally {
      setBusyAction("");
    }
  }, [busyAction, t]);

  const logoutCSGHub = useCallback(async () => {
    if (busyAction) {
      return;
    }
    setBusyAction("logout");
    setAuthError("");
    try {
      const next = normalizeCSGHubAuthStatus(await logoutCSGHubAuth());
      setStatus(next);
      setLoginPending(false);
    } catch (err) {
      setAuthError(errorMessage(err, t("csghubLogoutFailed")));
    } finally {
      setBusyAction("");
    }
  }, [busyAction, setStatus, t]);

  useEffect(() => {
    if (!loginPending) {
      return undefined;
    }
    const startedAt = Date.now();
    let cancelled = false;

    async function pollStatus() {
      try {
        const next = await refreshStatus();
        if (cancelled) {
          return;
        }
        if (isCSGHubAuthenticated(next)) {
          setLoginPending(false);
          setAuthError("");
          return;
        }
      } catch (_) {
        // Keep polling until timeout; transient callback timing is expected.
      }
      if (!cancelled && Date.now() - startedAt >= LOGIN_POLL_TIMEOUT_MS) {
        setLoginPending(false);
        setAuthError(t("csghubLoginTimedOut"));
      }
    }

    const firstPoll = window.setTimeout(pollStatus, 1000);
    const interval = window.setInterval(pollStatus, LOGIN_POLL_INTERVAL_MS);
    return () => {
      cancelled = true;
      window.clearTimeout(firstPoll);
      window.clearInterval(interval);
    };
  }, [loginPending, refreshStatus, t]);

  return {
    csghubAuthBusy: Boolean(busyAction),
    csghubAuthError: authError || (statusQuery.isError ? errorMessage(statusQuery.error, t("csghubStatusFailed")) : ""),
    csghubAuthPending: loginPending,
    csghubAuthStatus: status,
    loginCSGHub,
    logoutCSGHub,
  };
}

async function fetchNormalizedCSGHubAuthStatus(): Promise<CSGHubAuthStatus> {
  return normalizeCSGHubAuthStatus(await fetchCSGHubAuthStatus());
}
