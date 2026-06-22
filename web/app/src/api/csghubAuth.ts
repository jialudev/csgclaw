import { get, post } from "@/api/client";
import { ApiEndpoints } from "@/shared/constants/api";

export function fetchCSGHubAuthStatus(): Promise<unknown> {
  return get(ApiEndpoints.csghubAuthStatus);
}

export function beginCSGHubAuthLogin(returnURL = ""): Promise<unknown> {
  return post(ApiEndpoints.csghubAuthLogin, { return_url: returnURL });
}

export function logoutCSGHubAuth(): Promise<unknown> {
  return post(ApiEndpoints.csghubAuthLogout);
}
