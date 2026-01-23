# Production SSO Audit Report

**Date:** 2026-01-22
**Auditor:** Claude Code (Automated Playwright Tests)
**SSO Provider:** Janua (auth.madfam.io)
**Test Credentials:** `admin@madfam.io`

---

## Executive Summary

| Platform | Landing | SSO Redirect | Login Form | Auth Result | Dashboard | Status |
|----------|---------|--------------|------------|-------------|-----------|--------|
| **Janua** | N/A | N/A | N/A | N/A | N/A | **Provider** |
| **Dhanam** | ✅ 200 | ✅ auth.madfam.io | ✅ Credentials filled | ❌ INTERNAL_ERROR | ❌ Not reached | **FAILED** |
| **Enclii** | ✅ 200 | ✅ auth.madfam.io | ✅ Credentials filled | ❌ Auth failed | ❌ Not reached | **FAILED** |

### Critical Findings

**Both relying parties fail to complete SSO authentication due to Janua backend errors.**

---

## Platform 1: Dhanam (app.dhan.am)

### Test Flow

1. **Navigate to app.dhan.am** ✅
   - HTTP Status: 200
   - Landing page with "Sign in with Janua SSO" button visible

2. **Click SSO button** ✅
   - Successfully redirected to Janua login
   - URL: `auth.madfam.io/api/v1/auth/login?...&client_name=Dhanam+Ledger`

3. **Enter credentials** ✅
   - Email: `admin@madfam.io`
   - Password: Filled successfully
   - Submit clicked

4. **OAuth Authorize** ❌ **FAILED**
   - After login, stuck on `/api/v1/oauth/authorize` endpoint
   - Error returned:
   ```json
   {
     "error": {
       "code": "INTERNAL_ERROR",
       "message": "An unexpected error occurred",
       "request_id": 135672331152912,
       "timestamp": 1769129493.5957334
     }
   }
   ```

### Evidence
- Screenshot: `dhanam-01-landing.png` - Login page with SSO options
- Screenshot: `dhanam-02-sso-redirect.png` - Janua SSO login form
- Screenshot: `dhanam-03-post-login.png` - INTERNAL_ERROR response
- Screenshot: `dhanam-ERROR-detected.png` - Error confirmation

---

## Platform 2: Enclii (app.enclii.dev)

### Test Flow

1. **Navigate to app.enclii.dev** ✅
   - HTTP Status: 200
   - Initial "Checking session..." loading state (~15 seconds)
   - Login page renders with "Sign in with Janua SSO" button

2. **Click SSO button** ✅
   - Successfully redirected to Janua login
   - URL: `auth.madfam.io/api/v1/auth/login?...&client_name=Enclii+Platform`

3. **Enter credentials** ✅
   - Email: `admin@madfam.io`
   - Password: Filled successfully
   - Submit clicked

4. **OAuth Callback** ❌ **FAILED**
   - Janua issues auth code successfully
   - Callback URL: `api.enclii.dev/v1/auth/callback?code=...&state=...`
   - Enclii API returns error:
   ```json
   {
     "error": "Authentication failed",
     "hint": "Could not complete OIDC authentication. Please try again."
   }
   ```

### Evidence
- Screenshot: `enclii-01-landing.png` - Login page with SSO button
- Screenshot: `enclii-02-sso-redirect.png` - Janua SSO login form
- Screenshot: `enclii-03-post-login.png` - Authentication failed error

---

## Janua SSO Provider (auth.madfam.io)

### OIDC Configuration ✅
```
Issuer: https://auth.madfam.io
Authorization Endpoint: /api/v1/oauth/authorize
Token Endpoint: /api/v1/oauth/token
JWKS URI: /.well-known/jwks.json
```

### Client Registrations
| Client | Client ID | Redirect URI |
|--------|-----------|--------------|
| Dhanam Ledger | `jnc_uE2zp9ume_Fd6jMl1elL6wqjiECM711t` | `https://app.dhan.am/auth/callback` |
| Enclii Platform | `jnc_RqeHy54KYGjVr8yQiBeUncMhnQFhS2NA` | `https://api.enclii.dev/v1/auth/callback` |

### Issues Identified

1. **Dhanam - INTERNAL_ERROR on authorize**
   - Login succeeds (user authenticated to Janua)
   - OAuth authorize endpoint fails with 500 internal error
   - Possible causes: Session handling, database issue, code generation failure

2. **Enclii - Auth code works but callback fails**
   - Login succeeds
   - Auth code issued successfully
   - Enclii's callback handler fails to exchange code for token
   - Possible causes: Token endpoint error, client secret mismatch, callback processing bug

---

## Root Cause Analysis

### Janua Backend (Priority 1)
The INTERNAL_ERROR on Dhanam suggests a backend issue in Janua's OAuth authorize flow:
- User authentication succeeds (login form works)
- Authorization code generation/session binding fails
- This affects all relying parties using PKCE flow

### Enclii Callback Handler (Priority 2)
Enclii receives a valid auth code but fails to complete authentication:
- The `/v1/auth/callback` endpoint exists and receives the code
- Token exchange or session creation fails
- Check Enclii API logs for token endpoint errors

---

## Recommendations

### Immediate Actions (P0)

1. **Check Janua server logs** for INTERNAL_ERROR around timestamp 1769129493
2. **Verify database connectivity** for Janua's authorization code table
3. **Check Enclii API logs** for callback processing errors
4. **Test token endpoint directly**:
   ```bash
   curl -X POST https://auth.madfam.io/api/v1/oauth/token \
     -d "grant_type=authorization_code&code=...&redirect_uri=..."
   ```

### Short-term Fixes

1. Add better error handling in Janua's authorize endpoint
2. Add retry logic in Enclii's callback handler
3. Implement health checks for OAuth endpoints

### Monitoring

1. Add alerting for 500 errors on `/api/v1/oauth/authorize`
2. Monitor auth callback success/failure rates
3. Track token exchange latency and errors

---

## Test Environment

- **Browser:** Chromium (Playwright)
- **Test Framework:** Playwright Test v1.57.0
- **Test Mode:** Headless
- **Test Date:** 2026-01-22
- **Test Files:** `tests/sso-audit/sso-production-audit.spec.ts`

---

## Appendix: Screenshot Inventory

| File | Description |
|------|-------------|
| `dhanam-01-landing.png` | Dhanam login page with SSO options |
| `dhanam-02-sso-redirect.png` | Janua SSO form for Dhanam |
| `dhanam-03-post-login.png` | INTERNAL_ERROR response |
| `dhanam-sso-credentials-filled.png` | Credentials entered |
| `dhanam-ERROR-detected.png` | Error confirmation |
| `enclii-01-landing.png` | Enclii login page |
| `enclii-02-sso-redirect.png` | Janua SSO form for Enclii |
| `enclii-03-post-login.png` | Authentication failed error |

---

**Report Generated:** 2026-01-22T00:50:00Z
**Status:** AUDIT COMPLETE - CRITICAL ISSUES FOUND
