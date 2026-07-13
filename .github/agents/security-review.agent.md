---
name: security-review
description: Evidence-based, read-only security review of the Echo v5 app, Firebase integration, separate Go function, container, and CI.
---

# Task: REST API Security Review

Perform an evidence-based security review of this Echo v5 playground using the OWASP API Security risks as a guide.
This is a read-only audit; do not modify code, dependencies, generated files, or repository configuration unless
explicitly requested. Verify behavior before reporting it and distinguish application defects from controls that belong
at a Cloud Run, IAM, WAF, load-balancer, or organization-policy boundary.

## Required File Reads

Before analysis, read these files:
1. `cmd/server/application.go` and `cmd/server/config.go` - Application setup, middleware, timeouts, IP trust, and configuration
2. `internal/platform/middleware/cors.go` - CORS configuration
3. `internal/platform/middleware/security.go` - Security headers middleware
4. `internal/platform/auth/middleware.go` - Authentication middleware and failure logging
5. `internal/platform/audit/audit.go` - Request-scoped audit logging
6. `internal/platform/respond/respond.go` - Error handling and panic recovery
7. `internal/platform/request/json.go` and `internal/platform/request/query.go` - Strict body and query contracts
8. All files in `internal/http/v1/` - Endpoint definitions
9. `internal/service/profile/firestore.go` - Data access and audit-event calls
10. `go.mod` - echo-observability and other security-relevant dependency versions
11. `functions/function.go`, `functions/cmd/server/main.go`, and `functions/go.mod` - independent function contract
12. `Dockerfile`, `.dockerignore`, `firebase.json`, `firestore.rules`, and `.github/workflows/*.yml` - artifact and CI controls

Inspect the `github.com/janisto/echo-observability` version selected by `go.mod` when validating request ID handling,
trace correlation, logger field safety, and access-log behavior. Do not assume the removed local middleware contract.

## Security Review Checklist

### 1. Authentication & Authorization
- [ ] All protected endpoints require authentication
- [ ] Authorization checks verify user permissions before resource access
- [ ] Token validation handles all error cases (expired, revoked, invalid)
- [ ] `WWW-Authenticate: Bearer` header included in 401 responses
- [ ] No sensitive operations allowed without verified identity
- [ ] Successful verification cannot install a nil or blank-UID identity

### 2. Input Validation & Data Sanitization
- [ ] All inputs validated via go-playground/validator struct tags with strict types
- [ ] Path parameters have proper type constraints
- [ ] Query parameters validated with validate tags
- [ ] Closed query contracts reject unknown keys and bound cursor length
- [ ] Request body limits enforced
- [ ] No raw string interpolation in database queries (prevent injection)

### 3. Security Headers (OWASP Recommended)
Verify these headers are set in security middleware:
```http
Cache-Control: no-store
Content-Security-Policy: default-src 'none'; frame-ancestors 'none'
Cross-Origin-Opener-Policy: same-origin
Cross-Origin-Resource-Policy: same-origin
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Referrer-Policy: strict-origin-when-cross-origin
```

Content type is response-specific. HSTS belongs at the TLS-terminating Cloud Run or proxy edge; do not add it to
plain local HTTP responses.

### 4. Error Handling & Information Leakage
- [ ] Error responses use RFC 9457 Problem Details format
- [ ] Error responses use generic messages (e.g., "Unauthorized" not internal details)
- [ ] Stack traces never exposed in production
- [ ] Internal exception details logged but not returned to clients
- [ ] 404 vs 403 responses don't leak resource existence
- [ ] Validation errors don't expose internal field names or structure

### 5. Logging & Monitoring
- [ ] Expected 401 responses rely on the correlated access log rather than duplicate warning logs
- [ ] Authentication dependency failures are logged without tokens or PII
- [ ] Sensitive data (tokens, passwords, PII) never logged
- [ ] Security events use appropriate log levels (WARN/ERROR)
- [ ] Request correlation IDs present for traceability (X-Request-ID)
- [ ] Suspicious patterns (brute force, scanning) would be detectable

### 6. Secrets & Configuration
- [ ] No hardcoded secrets, API keys, or credentials in code
- [ ] Secrets loaded via environment variables
- [ ] Service account files excluded from version control
- [ ] Debug mode disabled in production configuration
- [ ] Firebase offline and emulator modes are development-only; live mode rejects demo projects and emulator hosts
- [ ] Emulator authorities use strict `host:port` values without schemes, whitespace, or invalid ports

### 7. CORS & Origin Policy
- [ ] Wildcard CORS is documented as an intentional public-playground policy with credentials disabled
- [ ] Credentials allowed only from trusted origins
- [ ] Preflight requests handled correctly
- [ ] Methods and headers properly restricted
- [ ] Vary header includes Origin for proper caching

### 8. Rate Limiting & DoS Protection
- [ ] Rate limiting responsibility is documented for the intended deployment boundary; absence of an in-process limiter
      is not automatically a defect in this non-production example
- [ ] Request body size limits enforced
- [ ] Timeouts configured for external service calls
- [ ] Pagination limits on list endpoints (cursor-based pagination)

### 9. Insecure Direct Object References (IDOR)
- [ ] Users can only access resources they own
- [ ] Resource ownership verified before read/update/delete
- [ ] UUIDs or non-sequential IDs used where appropriate
- [ ] Bulk operations validate all resource access

### 10. Panic Recovery
- [ ] Panic recovery middleware is in place
- [ ] Panics are logged with stack traces (server-side only)
- [ ] Panics return proper Problem Details response to client
- [ ] Panics after response commit re-panic with `http.ErrAbortHandler` to abort partial responses
- [ ] No sensitive information leaked in panic responses

### 11. Content Negotiation Security
- [ ] Unsupported content types return 415 Unsupported Media Type
- [ ] Accept behavior follows the documented JSON fallback and never reflects an arbitrary media type
- [ ] A specific `q=0` exclusion overrides broader matching wildcards per RFC 9110
- [ ] Response content type matches negotiated type
- [ ] CBOR/JSON handling is secure

### 12. Dependency Security
- [ ] Dependencies reviewed for available updates without mutating the repository
- [ ] No known vulnerabilities in dependencies (`just vuln`)
- [ ] Minimal dependency footprint

### 13. Build, Function, and Supply Chain
- [ ] The runtime image is non-root and excludes repository-only or sensitive material
- [ ] OCI version and source revision metadata are not conflated
- [ ] Workflow permissions are least privilege and every action uses an exact release tag that Dependabot can update
- [ ] Read-only checkouts disable persisted credentials, workflows pass pinned `actionlint`, and emulator coverage is separate
- [ ] Ruleset-required `ci` and `lint` aggregate jobs fail unless every internal dependency succeeds
- [ ] Root and function modules receive independent build, test, lint, race, and vulnerability checks
- [ ] The function enforces its documented method, media type, body-size, and JSON contracts
- [ ] Deployment documentation does not claim Firebase CLI can deploy the Go function

## Output Format

Provide findings in this structure:

### Critical Issues
Issues requiring immediate attention (authentication bypass, data exposure, injection).

### High Priority
Significant security gaps (missing authorization, weak validation).

### Medium Priority
Best practice violations (logging gaps, incomplete headers).

### Low Priority / Recommendations
Enhancements for defense in depth.

### Security Strengths
Patterns implemented correctly that should be maintained.

Order findings by exploitability and impact, not checklist count. For each finding include:
- **Location**: File path and line number
- **Issue**: Clear description of the vulnerability
- **Risk**: Potential impact if exploited
- **Recommendation**: Specific remediation steps
- **Evidence**: The command, test, or code path that verifies the claim

If a category has no actionable finding, do not manufacture one. State the relevant residual risk separately when it
depends on an undeployed infrastructure boundary.

## Echo v5-Specific Considerations

- Check Echo middleware ordering (security-critical middleware should run early)
- Verify go-playground/validator tags are comprehensive
- Ensure Problem Details responses don't leak internal state
- Check that all routes use appropriate error helpers from respond package
- Verify CORS middleware configuration in Echo stack
- Check request ID propagation for audit trails
- Ensure structured logging redacts sensitive fields
