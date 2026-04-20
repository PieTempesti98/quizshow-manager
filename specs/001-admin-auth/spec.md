# Feature Specification: Admin Authentication

**Feature Branch**: `001-admin-auth`  
**Created**: 2026-04-20  
**Status**: Draft  
**Input**: US-A01 (Login) and US-A02 (Logout and Token Refresh) from docs/01-mvp-requirements.md

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Admin Login (Priority: P1)

An admin visits the login page and submits their email and password. On success, they are redirected to the management dashboard and gain access to all admin-only areas. On failure, they receive a generic error that does not reveal whether the email or password was wrong.

**Why this priority**: The entire admin panel is inaccessible without this flow. It is the entry point to all management functionality.

**Independent Test**: Can be fully tested by submitting valid and invalid credentials and observing the response — either successful dashboard access or a non-specific error message.

**Acceptance Scenarios**:

1. **Given** a valid admin email and correct password, **When** the admin submits the login form, **Then** they receive a short-lived access credential (15-minute TTL) and a long-lived refresh credential (7-day TTL), and are redirected to the dashboard.
2. **Given** a valid email but wrong password, **When** the admin submits the form, **Then** a generic "Invalid credentials" error is returned — no indication of which field is incorrect.
3. **Given** an unregistered email, **When** the admin submits the form, **Then** the same generic error is returned as for a wrong password.
4. **Given** an empty or malformed email field, **When** the admin submits the form, **Then** a validation error is returned before attempting to authenticate.
5. **Given** the admin is already authenticated, **When** they visit the login page, **Then** they are redirected to the dashboard.

---

### User Story 2 - Transparent Token Refresh (Priority: P1)

While the admin is working in the panel, their short-lived access credential expires. The system automatically obtains a new access credential using the long-lived refresh credential stored in an httpOnly cookie, without interrupting the admin's workflow or requiring them to log in again.

**Why this priority**: Without transparent refresh, admins would be forced to log in every 15 minutes, which is unacceptable for any real-world session.

**Independent Test**: Can be tested by waiting for the access token to expire (or manually expiring it), then making a protected request and observing that the request succeeds with a new access token issued transparently.

**Acceptance Scenarios**:

1. **Given** a valid refresh credential in the httpOnly cookie, **When** the access credential has expired, **Then** a new access credential is issued and the admin can continue working without interruption.
2. **Given** the refresh credential has itself expired, **When** a refresh is attempted, **Then** the admin is redirected to the login page.
3. **Given** the refresh credential has been revoked server-side (e.g., after logout on another device), **When** a refresh is attempted, **Then** the refresh is rejected and the admin is redirected to the login page.

---

### User Story 3 - Logout with Server-Side Invalidation (Priority: P2)

An admin clicks logout. Their session is fully terminated: the refresh credential is revoked in persistent storage so that no further access can be obtained from it, and the admin is redirected to the login page. The short-lived access credential expires naturally.

**Why this priority**: Logout correctness is a security requirement. Without server-side revocation, a stolen refresh credential would remain valid for up to 7 days.

**Independent Test**: Can be tested by logging out and then attempting to use the now-revoked refresh cookie to obtain a new access token — the attempt must fail with an unauthorized error.

**Acceptance Scenarios**:

1. **Given** an authenticated admin, **When** they call logout, **Then** the refresh credential is marked revoked in persistent storage, the `qz_refresh` httpOnly cookie is cleared, and the admin is redirected to the login page.
2. **Given** a logout has been performed, **When** the same refresh credential is used to request a new access token, **Then** the request is rejected with an unauthorized error.
3. **Given** the access credential is still within its 15-minute window, **When** the admin calls a protected endpoint after logout, **Then** the access credential remains valid until natural expiry — short-lived access credentials are not blacklisted server-side.

---

### User Story 4 - Protected Resource Access via Auth Middleware (Priority: P1)

All admin management endpoints (questions, sessions, categories, statistics) are guarded by an auth gate that validates the access credential on every request. The gate supports swapping the credential issuer (e.g., from a self-managed identity service to a corporate SSO provider) without requiring code changes — only an environment configuration update.

**Why this priority**: Without an auth gate, all management endpoints are publicly accessible, which is a critical security gap that blocks any further safe development.

**Independent Test**: Can be tested by calling a protected endpoint with no token, an expired token, a valid admin token, and a valid non-admin token — only the valid admin token should succeed.

**Acceptance Scenarios**:

1. **Given** a request with no Authorization header, **When** a protected endpoint is called, **Then** the server responds with 401 Unauthorized.
2. **Given** an expired access credential, **When** a protected endpoint is called, **Then** the server responds with 401 Unauthorized.
3. **Given** a valid access credential from the configured issuer, **When** a protected endpoint is called, **Then** the request is processed normally.
4. **Given** the issuer configuration is changed to a new value, **When** credentials from the new issuer are presented, **Then** they are accepted without any code change — only the environment configuration is updated.
5. **Given** a valid player or projection credential (wrong role), **When** an admin-only endpoint is called, **Then** the server responds with 403 Forbidden.

---

### Edge Cases

- What happens when the `qz_refresh` cookie is present but the token record does not exist in persistent storage (e.g., database was reset)? → Refresh must be rejected with 401; the admin must log in again.
- What happens when two concurrent refresh requests arrive with the same refresh credential? → Only one should succeed; the second must be rejected to prevent credential replay.
- What happens when an admin account is soft-deleted while a session is active? → The existing access credential remains valid until its natural TTL expiry; the next refresh attempt must be rejected since the admin record is inactive.
- What happens when the `JWT_ISSUER` environment variable is not set at startup? → The server must refuse to start or enforce a safe restrictive default; it must never silently accept tokens from any issuer.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST allow an admin to authenticate by submitting their registered email address and password.
- **FR-002**: The system MUST validate that the email field is non-empty and in a valid email format, and that the password field is non-empty, before attempting authentication.
- **FR-003**: On successful authentication, the system MUST issue a short-lived access credential with a 15-minute TTL, returned in the response body, and a long-lived refresh credential with a 7-day TTL, delivered as an httpOnly cookie named `qz_refresh`.
- **FR-004**: The system MUST store a server-side record of each issued refresh credential, including its expiry time, to enable revocation checks.
- **FR-005**: On failed authentication (unrecognised email or wrong password), the system MUST return a generic error without indicating which field is incorrect.
- **FR-006**: The system MUST provide a token refresh endpoint that accepts the `qz_refresh` httpOnly cookie and returns a new access credential if the refresh credential is valid and not revoked.
- **FR-007**: The system MUST provide a logout endpoint that marks the current refresh credential as revoked in persistent storage and clears the `qz_refresh` httpOnly cookie.
- **FR-008**: All admin management endpoints MUST be protected by an auth middleware that validates the access credential on every request.
- **FR-009**: The auth middleware MUST read the accepted credential issuer from an environment variable so that the issuer can be changed without modifying application code.
- **FR-010**: The auth middleware MUST reject credentials whose issuer does not match the configured value.
- **FR-011**: The auth middleware MUST enforce role-based access: only credentials carrying the `admin` role are accepted on admin endpoints; `player` and `projection` roles must be rejected with 403 Forbidden.
- **FR-012**: The system MUST reject refresh attempts using a revoked or expired refresh credential with a 401 Unauthorized response.
- **FR-013**: Short-lived access credentials MUST expire naturally at their TTL; the system MUST NOT maintain a server-side blacklist for access credentials.

### Key Entities

- **Admin Account**: An operator identity with a unique email address and a securely stored password credential. Supports soft deletion to preserve historical data while disabling access.
- **Refresh Token Record**: A server-side entry linking a long-lived refresh credential to an admin account, with an expiry timestamp and an optional revocation timestamp. Only a one-way hash of the credential is persisted — the raw value is never stored.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An admin with valid credentials can reach the dashboard in under 2 seconds from submitting the login form under normal network conditions.
- **SC-002**: 100% of protected admin endpoints return 401 or 403 for unauthenticated or insufficiently-privileged requests.
- **SC-003**: A revoked refresh credential cannot be used to obtain a new access credential — the rejection occurs in the same request cycle with no delay.
- **SC-004**: Changing the credential issuer requires zero code changes — only an environment variable update and server restart.
- **SC-005**: The login error message provides no information that would allow an attacker to enumerate valid admin email addresses (verified by testing both unregistered email and wrong password paths and confirming identical error responses).
- **SC-006**: Issued access credentials expire within 15 minutes and refresh credentials expire within 7 days — verifiable by inspecting the credential payload.

## Assumptions

- There is exactly one admin account in the MVP. The data model supports multiple admins but no admin self-registration or invite flow is in scope for this feature.
- A database seed or bootstrap process is responsible for creating the initial admin account; this feature does not include an admin account creation endpoint or UI.
- The `qz_refresh` cookie is scoped to the `/api/v1/auth/` path to limit its exposure surface across all endpoints.
- The `JWT_ISSUER` environment variable must be set before the server starts; its absence is treated as a configuration error.
- Short-lived access credentials are transmitted via the `Authorization: Bearer` header by the client; the server never reads them from cookies.
- Token rotation on refresh (issuing a new refresh credential and invalidating the old one on each refresh call) is out of scope for MVP — the same refresh credential remains valid until explicit logout or natural expiry.
- The one-way hash algorithm used to store refresh token records in persistent storage is fixed at implementation time and is not user-configurable.
