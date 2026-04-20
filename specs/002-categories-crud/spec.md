# Feature Specification: Categories CRUD

**Feature Branch**: `002-categories-crud`  
**Created**: 2026-04-21  
**Status**: Draft  
**Input**: User description: "Implement categories CRUD (US-Q04): list all categories, create a category, rename a category, soft-delete a category."

## User Scenarios & Testing *(mandatory)*

### User Story 1 — List and Create Categories (Priority: P1)

As an admin, I want to view all existing categories and create new ones so that I can organize the question bank before adding questions.

**Why this priority**: Without the ability to list and create categories, no other category operations are useful. This is the foundational flow.

**Independent Test**: Can be fully tested by authenticating as admin, calling the list endpoint (expecting an empty or populated list), creating a category, and verifying it appears in the subsequent list with a `question_count` of 0.

**Acceptance Scenarios**:

1. **Given** I am authenticated as admin, **When** I request the list of categories, **Then** I receive all non-deleted categories, each with name, slug, question count, and creation date.
2. **Given** I am authenticated as admin, **When** I create a category with name "Scienze naturali", **Then** the category is created with slug "scienze-naturali", `question_count` 0, and the full category object is returned.
3. **Given** I am authenticated as admin, **When** I attempt to create a category whose name already exists among non-deleted categories, **Then** the request is rejected with a conflict error.
4. **Given** I am not authenticated, **When** I attempt to list or create categories, **Then** the request is rejected with an unauthorized error.

---

### User Story 2 — Rename a Category (Priority: P2)

As an admin, I want to rename an existing category so that I can correct naming mistakes or reorganize the question bank.

**Why this priority**: Renaming is a common maintenance operation and does not depend on deletion; it can be implemented and tested independently.

**Independent Test**: Can be fully tested by creating a category, renaming it to a new unique name, and verifying the updated name and auto-regenerated slug are returned.

**Acceptance Scenarios**:

1. **Given** a category "Storia" exists, **When** I rename it to "Storia Moderna", **Then** the category is updated with the new name and slug "storia-moderna".
2. **Given** categories "Storia" and "Arte" both exist, **When** I attempt to rename "Storia" to "Arte", **Then** the request is rejected with a conflict error.
3. **Given** a category does not exist or has been soft-deleted, **When** I attempt to rename it, **Then** the request is rejected with a not-found error.

---

### User Story 3 — Soft-Delete a Category (Priority: P3)

As an admin, I want to delete a category that is no longer needed so that the question bank stays clean and organized.

**Why this priority**: Deletion is the most constrained operation (blocked by active questions) and is least critical for initial setup.

**Independent Test**: Can be fully tested by creating a category with no questions, deleting it, and verifying it no longer appears in the list. Also verifiable by creating a category with at least one active question and confirming deletion is blocked with a count of blocking questions.

**Acceptance Scenarios**:

1. **Given** a category with no active (non-deleted) questions, **When** I delete it, **Then** the category is soft-deleted and no longer appears in the category list.
2. **Given** a category with 5 active questions, **When** I attempt to delete it, **Then** the request is rejected with a `CATEGORY_HAS_QUESTIONS` error that includes the count (5) of blocking questions.
3. **Given** a category has already been soft-deleted, **When** I attempt to delete it again, **Then** the request is rejected with a not-found error.

---

### Edge Cases

- What happens when a category name is at the maximum length (50 characters)? It must be accepted; names longer than 50 characters must be rejected with a validation error.
- What happens when two names would produce the same slug after normalization (e.g., "Arte Musica" and "Arte-Musica")? Slug uniqueness is enforced; the second creation is rejected with a conflict error.
- What happens when a category is referenced by questions that have themselves been soft-deleted? Only questions where `deleted_at` is null count as blocking; soft-deleted questions do not block category deletion.
- What happens if the category ID in a rename or delete request is malformed (not a valid UUID)? The request is rejected with a validation error before hitting the data store.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST return a list of all non-deleted categories, each including name, slug, question count (count of non-deleted questions in that category), and creation timestamp.
- **FR-002**: The system MUST allow an authenticated admin to create a category by providing only a name; the slug MUST be auto-generated by lowercasing the name and replacing spaces with hyphens.
- **FR-003**: Category names MUST be unique among non-deleted categories; attempting to create or rename to a conflicting name MUST return a `CONFLICT` error.
- **FR-004**: Category slugs MUST be unique among non-deleted categories; a slug collision on creation or rename MUST return a `CONFLICT` error.
- **FR-005**: Category names MUST NOT exceed 50 characters; requests exceeding this limit MUST be rejected with a `VALIDATION_ERROR`.
- **FR-006**: The system MUST allow an authenticated admin to rename a non-deleted category; renaming regenerates the slug from the new name.
- **FR-007**: The system MUST allow an authenticated admin to soft-delete a non-deleted category; the category MUST no longer appear in the list immediately after deletion.
- **FR-008**: The system MUST block deletion of a category that has one or more non-deleted questions, returning a `CATEGORY_HAS_QUESTIONS` error with the count of blocking questions.
- **FR-009**: All four category endpoints MUST require a valid admin JWT; unauthenticated or insufficiently privileged requests MUST be rejected with `UNAUTHORIZED` or `FORBIDDEN`.
- **FR-010**: The `question_count` field in list and create responses MUST reflect only non-deleted questions at the time of the request.

### Key Entities

- **Category**: A named grouping for questions. Has a display name (max 50 chars), an auto-generated URL-safe slug, timestamps for creation and soft-deletion. Carries a derived `question_count` in API responses.
- **Question** (referenced): Each question belongs to one category. A question is "active" (blocking for category deletion) if it has not been soft-deleted.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An admin can list, create, rename, and delete categories without errors on all valid inputs.
- **SC-002**: Requests without a valid admin session are rejected 100% of the time across all four endpoints.
- **SC-003**: Attempts to delete a category with active questions are rejected 100% of the time, and the error always includes the correct blocking question count.
- **SC-004**: Attempts to create or rename a category with a duplicate name are rejected 100% of the time with a clear conflict indication.
- **SC-005**: Soft-deleted categories never appear in the category list response.
- **SC-006**: All four category endpoints respond within standard web application latency for typical question bank sizes (up to a few thousand questions across tens of categories).

## Assumptions

- The `categories` table and its partial unique indexes (`categories_name_unique`, `categories_slug_unique`) are already defined in migration 001 and no new migration is required for this feature.
- The `RequireAdmin` middleware delivered in `001-admin-auth` is already implemented and will be reused without modification.
- Slug generation handles only basic ASCII-safe transformations for MVP (lowercase + spaces to hyphens); non-ASCII characters are passed through as-is without transliteration.
- The `question_count` is computed at query time rather than stored as a denormalized column; recomputing on every list or create request is acceptable at MVP scale.
- No pagination is required for the category list in MVP; the full list is returned in a single response (category count is expected to remain small).
- The slug uniqueness constraint covers only non-deleted rows (partial unique index); a new category may reuse the slug of a previously soft-deleted category.
