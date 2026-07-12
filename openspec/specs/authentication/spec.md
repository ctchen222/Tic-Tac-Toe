# Authentication Specification

## Purpose

Define the current identity, credential, token, and WebSocket admission contract.

## Requirements

### Requirement: Account registration

The service SHALL register a user with a unique username, store only a bcrypt password hash, and return a signed JWT after successful creation.

#### Scenario: Register a new account

- **WHEN** a client submits a valid unused username and password to `POST /api/register`
- **THEN** the service persists the account and returns a JWT for the created user

#### Scenario: Reject a duplicate username

- **WHEN** a client registers a username that already exists
- **THEN** the service rejects the request without creating another account

### Requirement: Credential login

The service SHALL authenticate a registered user by comparing the supplied password with the stored bcrypt hash and SHALL return a JWT only when they match.

#### Scenario: Login with valid credentials

- **WHEN** a client submits the correct username and password to `POST /api/login`
- **THEN** the service returns a JWT identifying that user

#### Scenario: Reject invalid credentials

- **WHEN** a client submits an unknown username or incorrect password
- **THEN** the service rejects the login without revealing which credential was wrong

### Requirement: Guest identity

The service SHALL create a uniquely named persistent guest account and return a JWT when a client calls `POST /api/guest-login`.

#### Scenario: Create a guest session

- **WHEN** an unauthenticated client requests guest login
- **THEN** the service creates a guest account in SQLite and returns its JWT

### Requirement: Token lifetime and identity

The service SHALL issue signed JWTs containing the user identifier and a 24-hour expiration time.

#### Scenario: Use a valid token

- **WHEN** a token is parsed before its expiration and its signature is valid
- **THEN** the service resolves the user identifier from the token claims

#### Scenario: Reject an expired or invalid token

- **WHEN** a token is expired, malformed, or has an invalid signature
- **THEN** token validation fails and no user identity is accepted

### Requirement: Authenticated WebSocket admission

The WebSocket endpoint SHALL require a valid JWT in the `token` query parameter before upgrading the HTTP connection.

#### Scenario: Upgrade an authenticated connection

- **WHEN** a client connects to `GET /api/ws` with a valid token
- **THEN** the server upgrades the connection and registers or reconnects the identified player

#### Scenario: Reject a missing token

- **WHEN** a client connects to `GET /api/ws` without a token
- **THEN** the server responds with an unauthorized error and does not upgrade the connection

#### Scenario: Reject an invalid token

- **WHEN** a client connects to `GET /api/ws` with an invalid or expired token
- **THEN** the server responds with an unauthorized error and does not upgrade the connection
