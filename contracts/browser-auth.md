# Browser authentication contract v2

This document is normative for the Management browser client and its
same-origin browser API.

## Application ownership

- Flutter starts without waiting for runtime configuration or authentication
  network requests.
- Flutter owns startup, login, expired-session, forbidden and retry UI.
- JavaScript bootstrap code may initialize the Flutter engine only. It must not
  fetch authentication resources, render provider controls or gate `runApp`.

## Flow

1. Flutter loads `/runtime-config.json`.
2. Flutter requests `GET /api/v1/auth/me` with browser credentials.
3. `200` enters the authenticated application. `401` causes Flutter to request
   `GET /api/v1/auth/providers`. `403` shows the global forbidden state.
4. The provider response contains only stable `id` and display `name` fields.
5. Flutter navigates the document to
   `/api/v1/auth/login?provider=<id>&return_to=<validated-relative-admin-uri>`.
6. Management validates `return_to`, starts OIDC and stores the destination in
   the server-side one-time login transaction.
7. Management consumes `/api/v1/auth/callback`, creates the server-side
   session, writes the session cookie and redirects to the stored destination.
8. `POST /api/v1/auth/logout` invalidates the session, clears the cookie and
   returns `204`, including when no session remains.

The callback remains a Management API route. Flutter never processes the web
OIDC `code` or `state`.

## Redirects

- `return_to` is a root-relative URI under the configured admin root.
- Scheme, authority, fragment, control characters, backslashes, protocol
  relative URLs, API paths and auth UI paths are rejected.
- The browser validates the route before login. Management validates it again
  before storing and before redirecting.
- A successful callback redirects to the stored destination or the configured
  admin root.
- A failed callback redirects to `/login?auth_error=<code>`, where `code` is
  one of `access_denied`, `provider_error`, `invalid_callback` or
  `session_creation_failed`.
- Raw provider errors, tokens, OIDC `code`, OIDC `state` and session
  identifiers never appear in the final UI URL.

## Security and transport

- Browser application and API use the same HTTPS origin.
- Requests include browser credentials.
- The production cookie is `__Host-endlessnet_session`, with `Path=/`,
  `HttpOnly`, `Secure` and `SameSite=Lax`, and without `Domain`.
- Cookie-mutating requests require the exact Management `Origin`.
- Access tokens, refresh tokens and session identifiers never enter URLs,
  frontend storage, JavaScript globals, logs or build artifacts.
- CSP remains strict and all Flutter runtime assets are local.
- `/api/v1/*` is never served by the SPA fallback.

## Failure behavior

- Missing or expired sessions return `401` and clear an invalid browser cookie.
- An authenticated identity without global Management access returns `403`.
- Feature-level `403` does not invalidate the session.
- Identity unavailability returns a controlled `503`.
- Responses include `X-Request-ID`; clients may surface it for diagnostics.
