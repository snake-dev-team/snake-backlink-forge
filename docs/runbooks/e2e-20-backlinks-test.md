# E2E Smoke Test: 20 Backlinks Live Run

**Purpose:** Validate full SBF backlink workflow against 20 real WordPress sites.
**Expected duration:** 30–45 minutes (including setup + execution + verification)
**Success criteria:** ≥18/20 jobs succeed, ≥18/20 verified, AI cost <$1, total time <30min

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Environment Setup](#environment-setup)
3. [Connect 20 WordPress Sites](#connect-20-wordpress-sites)
4. [Create Campaign](#create-campaign)
5. [Trigger AI Content Generation](#trigger-ai-content-generation)
6. [Monitor Worker Publishing](#monitor-worker-publishing)
7. [Verification Phase](#verification-phase)
8. [Pass Criteria Validation](#pass-criteria-validation)
9. [Troubleshooting](#troubleshooting)
10. [Metrics Capture](#metrics-capture)

---

## Prerequisites

Before starting, ensure you have:

### User Requirements
- **20 WordPress sites** with App Passwords generated for each
- **Site credentials:** URL + App Username + App Password (password-manager recommended)
- **Money site URL:** A test affiliate URL (e.g., `https://test-affiliate.example.com`)
- **~50+ standard credits** in wallet (20 jobs @ ~2–3 credits per job + 10 buffer)
- **ANTHROPIC_API_KEY** configured (Fly.io secrets or local `.env`)

### System Requirements
- **Local dev:** Go 1.26+, PostgreSQL 15+ running locally
- **Fly.io prod:** Fly CLI authenticated, secrets configured
- **Network:** Outbound HTTPS to all 20 WP sites + Anthropic API
- **Time block:** Uninterrupted 45 minutes (worker polling can be interrupted but is suboptimal)

### WordPress Site Setup Checklist

For each of your 20 WP sites:

- [ ] Site runs WordPress 6.0+ with REST API enabled (default: enabled)
- [ ] User account created with "Editor" or "Administrator" role
- [ ] App Password generated: **Settings → App Passwords** (WP 5.6+)
  - App name: "SBF Backlink Forge"
  - Select only: **Posts** (create/read/update) + **Categories** (read)
  - Copy resulting password to password manager
- [ ] Note the **base URL** (e.g., `https://example.com` — no trailing slash)
- [ ] Test credentials locally: `curl -u "username:apppassword" https://example.com/wp-json/wp/v2/users/me`
  - Expected: 200 OK with user object

---

## Environment Setup

### Local Development

1. **Start the API server** (if not already running):
   ```bash
   cd E:/tool_backlink/services/api
   go run ./cmd/api/main.go
   # Expected: "Starting server on :8080"
   ```

2. **Start the landing app** (if not already running):
   ```bash
   cd E:/tool_backlink
   pnpm dev
   # Expected: "ready on http://localhost:3000"
   ```

3. **Verify API is healthy:**
   ```bash
   curl http://localhost:8080/health
   # Expected: {"status":"ok"}
   ```

4. **Verify database is up:**
   ```bash
   curl http://localhost:8080/ready
   # Expected: {"status":"ready"} or {"status":"not_ready"} with DB details
   ```

### Fly.io Production

1. **Ensure secrets are set:**
   ```bash
   flyctl secrets list
   # Should include: DATABASE_URL, REDIS_URL, ANTHROPIC_API_KEY
   ```

2. **Verify deployment:**
   ```bash
   curl https://snake-backlink-api.fly.dev/health
   # Expected: {"status":"ok"}
   ```

3. **Optional: Stream logs during test:**
   ```bash
   flyctl logs -a sbf-api
   ```

---

## Connect 20 WordPress Sites

### Via Web UI (Recommended for UX validation)

1. **Log in to landing app:**
   - Navigate to `http://localhost:3000/login` (local) or `https://sbf-web.fly.dev/login` (prod)
   - Use your API key to log in

2. **Navigate to /sites:**
   - Click "Sites" in the navigation bar
   - Expected: empty list or existing sites

3. **Add sites one by one** (or use bulk import if available):
   - Click "Add Site"
   - Fill form:
     - **Base URL:** `https://example-1.com`
     - **App Username:** (from WP credentials)
     - **App Password:** (from WP credentials)
     - **Label:** "Test Site 1" (optional but recommended)
   - Click "Connect"
   - Expected: status turns **green ✓ Connected** within 5s
   - If fails: check error message → see [Troubleshooting](#troubleshooting)

4. **Repeat for all 20 sites**
   - Total time: ~30 seconds per site (25 min for 20)
   - Expected final count: 20 sites, all status "Connected"

### Via API (Faster, if UI not ready)

```bash
#!/bin/bash
# bulk-add-sites.sh — add sites via API

API_URL="http://localhost:8080"
AUTH_TOKEN="your-api-key"  # Get from Telegram bot

# Site list (CSV format: base_url,app_username,app_password,label)
cat > sites.csv <<'EOF'
https://site-1.example.com,app_user_1,password_1,Test Site 1
https://site-2.example.com,app_user_2,password_2,Test Site 2
...
https://site-20.example.com,app_user_20,password_20,Test Site 20
EOF

while IFS=, read -r base_url app_user app_pass label; do
  curl -X POST "$API_URL/api/v1/wp-sites" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"base_url\": \"$base_url\",
      \"app_username\": \"$app_user\",
      \"app_password\": \"$app_pass\",
      \"label\": \"$label\"
    }"
  echo "Added: $label"
  sleep 1  # Rate limit to 1 req/sec
done < sites.csv
```

**Verify all 20 sites are connected:**
```bash
curl -H "Authorization: Bearer $AUTH_TOKEN" \
  http://localhost:8080/api/v1/wp-sites | jq '.[] | select(.status == "connected") | length'
# Expected: 20
```

---

## Create Campaign

### Via Web UI

1. **Navigate to /campaigns:**
   - Click "Campaigns" in the navigation bar
   - Click "New Campaign"

2. **Fill campaign form:**
   ```
   Campaign Name:        "E2E Test — 20 Backlinks [YYYYMMDD]"
   Topic:                "forex broker review vietnam"
   Money Site URL:       "https://test-affiliate.example.com"
   
   Anchor Texts (3 variants):
     - "broker uy tín" (weight 50, type: naked)
     - "sàn forex tốt nhất" (weight 30, type: exact)
     - "review broker" (weight 20, type: generic)
   
   Pool:                 "standard"
   Source Mode:          "custom"
   Selected Sites:       [select all 20]
   Quantity per Site:    20
   Tone:                 "storytelling"
   Daily Limit:          50
   Ethical Mode:         ON
   Start Now:            ✓ (CHECKED)
   ```

3. **Click "Create & Start"**
   - Expected: campaign created, redirects to campaign detail page
   - Status: "Running"
   - Job count: 20 queued

### Via API (If form not ready)

```bash
curl -X POST http://localhost:8080/api/v1/campaigns \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "E2E Test — 20 Backlinks",
    "money_site_url": "https://test-affiliate.example.com",
    "niche_keywords": ["forex", "broker"],
    "anchor_texts": [
      {"text": "broker uy tín", "weight": 50, "type": "naked"},
      {"text": "sàn forex tốt nhất", "weight": 30, "type": "exact"},
      {"text": "review broker", "weight": 20, "type": "generic"}
    ],
    "pool": "standard",
    "source_mode": "custom",
    "site_ids": ["<uuid-1>", "<uuid-2>", ..., "<uuid-20>"],
    "quantity": 20,
    "tone_preference": "storytelling",
    "daily_limit": 50,
    "ethical_mode": true,
    "start_now": true,
    "credits_allocated": 100
  }'
```

**Note the campaign UUID** (first 8 chars sufficient): `$CAMPAIGN_ID`

---

## Trigger AI Content Generation

### Via Web UI

1. **On campaign detail page:**
   - Scroll to "Content Generation" section
   - Click "Generate Content"
   - A progress bar appears: "5/20 generated"... "20/20 generated"
   - Expected duration: 3–5 minutes (depends on API rate limits)

2. **Monitor progress:**
   - Watch job cards update: status changes from "queued" → "content_ready"
   - Each row shows word count for generated content

### Via API

```bash
curl -X POST http://localhost:8080/api/v1/campaigns/$CAMPAIGN_ID/generate-content \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"force": false}'
```

Then poll for completion:

```bash
# Poll every 5 sec until all jobs have content_ready
for i in {1..60}; do
  curl -s http://localhost:8080/api/v1/campaigns/$CAMPAIGN_ID/jobs \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    | jq -r '.[] | select(.status != "content_ready") | length'
  
  remaining=$(curl -s http://localhost:8080/api/v1/campaigns/$CAMPAIGN_ID/jobs \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    | jq 'map(select(.status != "content_ready")) | length')
  
  echo "Jobs ready: $((20 - remaining))/20"
  
  if [ "$remaining" -eq 0 ]; then
    echo "Content generation complete!"
    break
  fi
  
  sleep 5
done
```

---

## Monitor Worker Publishing

### Background: Architecture Decision

**This build uses the embedded server worker** (Phase 7.05) rather than legacy browser extension.
- Worker starts automatically when API boots
- Polls every 10 seconds for `content_ready` jobs
- Posts to WordPress REST API `/wp-json/wp/v2/posts`
- Max 5 concurrent publishes
- Graceful backoff on transient failures

### Via Web UI

1. **On campaign detail page:**
   - Watch job cards update in real-time
   - Status flow: `content_ready` → `in_progress` → `success` or `failed`
   - Each successful job shows the published post URL

2. **Monitor progress:**
   ```
   Total:     20
   Content:   20/20 ✓
   Publishing: 5/20 in progress, 8/20 success, 0/20 failed
   ```

### Via API & SQL

**Check worker progress:**

```bash
# List all jobs for campaign with status breakdown
curl -s http://localhost:8080/api/v1/campaigns/$CAMPAIGN_ID/jobs \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  | jq 'group_by(.status) | map({status: .[0].status, count: length})'

# Expected output (streaming over 5-10 min):
# [
#   {"status": "success", "count": 5},
#   {"status": "in_progress", "count": 2},
#   {"status": "content_ready", "count": 13}
# ]
# (eventually)
# [
#   {"status": "success", "count": 20}
# ]
```

**Raw SQL query (local dev):**

```sql
-- Connect to local PostgreSQL
psql $DATABASE_URL

-- Monitor job status distribution
SELECT 
  status, 
  COUNT(*) as count,
  AVG(EXTRACT(EPOCH FROM (completed_at - dispatched_at)))::INT as avg_duration_sec
FROM jobs
WHERE campaign_id = $CAMPAIGN_ID
  AND user_id = $USER_ID
GROUP BY status;

-- View failed jobs (if any)
SELECT 
  id, 
  status, 
  error_code, 
  error_message, 
  target_url_snapshot,
  retry_count
FROM jobs
WHERE campaign_id = $CAMPAIGN_ID
  AND status IN ('failed', 'dlq')
ORDER BY created_at DESC;

-- Expected: no rows (ideal), or 1–2 transient failures
```

### Expected Timeline

| Time | Status | Notes |
|------|--------|-------|
| T+0m | 20 content_ready | AI generation complete |
| T+1m | 5 in_progress, 15 queued | Worker claiming jobs |
| T+3m | 10 success, 10 in_progress | Midway through |
| T+5m | 18 success, 2 failed | Near completion |
| T+7m | 20 success (or 18 success + 2 retried) | All published |

---

## Verification Phase

After all 20 jobs reach **success** status, the automatic verifier kicks in.

### Architecture Decision

The verifier:
- Runs asynchronously in the background
- HTTP HEAD checks each published post URL (confirm 200 OK)
- Parses HTML to verify anchor link is present
- Updates `verified=true` flag on job
- Retries every 5 min for unverified jobs (DNS propagation lag)

### Manual Spot Check

While verifier runs, manually check 3 random published URLs in browser:

```bash
# Grab 3 random success URLs
curl -s http://localhost:8080/api/v1/campaigns/$CAMPAIGN_ID/jobs \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  | jq -r '.[] | select(.status == "success") | .result_url' \
  | shuf -n 3
```

For each URL:
1. Open in browser
2. Verify post is live + readable
3. Check for anchor link (Ctrl+F for "broker uy tín" or "sàn forex tốt nhất")
4. Click anchor link → should navigate to money site

### Monitor Verification Progress

```bash
# Poll job verification status
curl -s http://localhost:8080/api/v1/campaigns/$CAMPAIGN_ID/jobs \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  | jq -r '.[] | select(.status == "success") | [.id[0:8], .verified] | @csv' \
  | column -t -s,

# Expected: verified column alternates between true/false as verifier processes
```

**Expected timeline:**
- T+5m: 0 verified (verifier hasn't started)
- T+8m: 5 verified
- T+12m: 15 verified
- T+20m: 20 verified ✓

---

## Pass Criteria Validation

After ~20 minutes, check success metrics:

```bash
# ─── Fetch final metrics ───
curl -s http://localhost:8080/api/v1/campaigns/$CAMPAIGN_ID/summary \
  -H "Authorization: Bearer $AUTH_TOKEN" | jq '.'

# Expected response structure:
# {
#   "campaign_id": "...",
#   "total_jobs": 20,
#   "success_count": 19,          # Goal: ≥18
#   "verified_count": 19,          # Goal: ≥18
#   "failed_count": 1,
#   "avg_time_to_publish_sec": 42,
#   "avg_time_to_verify_sec": 78,
#   "total_ai_cost_usd": 0.89,     # Goal: <$1.00
#   "total_duration_sec": 1242     # Goal: <1800 (30 min)
# }
```

### Checklist

- [ ] **Success count ≥18/20** ✓
- [ ] **Verified count ≥18/20** ✓
- [ ] **AI cost <$1.00** ✓
- [ ] **Total time <30 min** ✓
- [ ] **No data corruption** (verify via SQL: no orphan jobs, no duplicate posts)
- [ ] **Wallet debited correctly** (~45 credits for 20 jobs @ standard pool)

**Success:** All checks pass → **SHIP IT** ✓

---

## Troubleshooting

### 1. Site Connection Fails: "wp_invalid_credentials"

**Symptom:** After filling site form + clicking "Connect", error: `wp_invalid_credentials`

**Causes:**
- App password is incorrect
- App user doesn't have Editor role
- WP site is down

**Fix:**
1. Verify App Password in WP admin: **Settings → App Passwords**
   - Copy exact password (no extra spaces)
   - Paste into form
2. Verify user role: **Users** → select user → check role is "Editor+" (not Subscriber)
3. Test credentials locally:
   ```bash
   curl -u "appuser:apppassword" https://site-1.example.com/wp-json/wp/v2/users/me
   # Expected: 200 OK with user JSON
   ```
4. Retry connect in UI

---

### 2. Campaign Creation Succeeds, But No Jobs Enqueued

**Symptom:** Campaign created with status "Running", but job list is empty

**Causes:**
- No sites were selected in form
- Insufficient credits (budget depleted)
- DB transaction failed silently

**Fix:**
1. Verify sites were linked:
   ```bash
   curl -s http://localhost:8080/api/v1/campaigns/$CAMPAIGN_ID \
     -H "Authorization: Bearer $AUTH_TOKEN" | jq '.linked_site_count'
   # Expected: 20 (not 0)
   ```
2. Check wallet balance:
   ```bash
   curl -s http://localhost:8080/api/v1/wallet \
     -H "Authorization: Bearer $AUTH_TOKEN" | jq '.balance_credits'
   # Expected: ≥45 (for 20 jobs)
   ```
3. Manually re-enqueue:
   ```bash
   curl -X POST http://localhost:8080/api/v1/campaigns/$CAMPAIGN_ID/enqueue \
     -H "Authorization: Bearer $AUTH_TOKEN" \
     -d '{"count": 20}'
   ```

---

### 3. Content Generation Stalls at "5/20"

**Symptom:** Content generation starts but gets stuck at 5/20 for >10 min

**Causes:**
- ANTHROPIC_API_KEY not set or invalid
- Rate limit hit (Anthropic free tier: 3 RPM)
- Network timeout to Anthropic

**Fix:**
1. Verify API key is set:
   ```bash
   # Local dev:
   grep ANTHROPIC_API_KEY E:/tool_backlink/.env
   
   # Prod (Fly):
   flyctl secrets list | grep ANTHROPIC
   ```
2. Check logs:
   ```bash
   # Local:
   go run ./cmd/api/main.go 2>&1 | grep -i "anthropic\|rate"
   
   # Prod:
   flyctl logs -a sbf-api | grep -i "anthropic\|rate"
   ```
3. If rate-limited: wait 60s + retry
4. If key invalid: regenerate in Anthropic console → update env

---

### 4. Jobs Stuck in "in_progress"

**Symptom:** Jobs transition to `in_progress` but never reach `success` after 5 min

**Causes:**
- Worker crashed or paused
- WordPress site is down or blocking requests
- Lease timeout (worker didn't release lock)

**Fix:**
1. Check if worker is running:
   ```bash
   # Logs should show "worker started" + periodic "polling"
   # Local: check terminal running API
   # Prod: flyctl logs -a sbf-api | grep -i worker
   ```
2. Verify WP sites are still reachable:
   ```bash
   curl -u "appuser:apppassword" https://site-1.example.com/wp-json/wp/v2/posts \
     -H "Content-Type: application/json" \
     -d '{"title": "Test", "status": "draft"}'
   # Expected: 201 Created
   ```
3. Check for lease locks:
   ```sql
   SELECT id, lease_holder, lease_until, status FROM jobs
   WHERE campaign_id = $CAMPAIGN_ID AND status = 'in_progress'
   ORDER BY lease_until;
   
   -- If lease_until is stale (>5 min ago):
   UPDATE jobs SET lease_holder = NULL, lease_until = NULL, status = 'content_ready'
   WHERE campaign_id = $CAMPAIGN_ID AND status = 'in_progress' AND lease_until < NOW();
   ```

---

### 5. Verification Never Completes ("verified" stays NULL)

**Symptom:** All jobs reach `success`, but `verified` flag remains `NULL` after 20 min

**Causes:**
- Verifier didn't start or crashed
- Published URLs are inaccessible (DNS propagation lag, firewall)
- HTML doesn't contain expected anchor text

**Fix:**
1. Check verifier logs:
   ```bash
   # Logs should show "verifier started" + "verifying job..."
   ```
2. Manually test verification of one URL:
   ```bash
   # Grab a success URL
   JOB_URL=$(curl -s http://localhost:8080/api/v1/campaigns/$CAMPAIGN_ID/jobs \
     -H "Authorization: Bearer $AUTH_TOKEN" \
     | jq -r '.[0].result_url')
   
   # Fetch the page
   curl -s "$JOB_URL" | grep -i "broker uy tín"
   # Expected: found in HTML
   ```
3. If URL is unreachable: wait 2 min (DNS) + retry
4. If anchor is missing: check content generation produced valid HTML

---

### 6. Worker Hangs on Shutdown

**Symptom:** When stopping API, worker takes >10 sec to shut down

**Expected behavior:** Graceful drain (in-flight jobs complete, then exit)

**Workaround:** Allow 15 sec for shutdown, then force-kill if needed.

---

## Metrics Capture

### Template: Record Results

Copy this template to `plans/260505-0916-real-e2e-20-backlinks/test-results/run-1.md`:

```markdown
# E2E Test Run #1 — 20 Backlinks

**Date:** 2026-05-05  
**Operator:** [Your name]  
**Environment:** [local | production]

## Setup Phase
- **Start time:** HH:MM UTC
- **Sites connected:** 20/20 ✓
- **Campaign created:** [Campaign ID]
- **Jobs enqueued:** 20/20

## Content Generation
- **Start time:** HH:MM UTC
- **End time:** HH:MM UTC
- **Duration:** X min
- **Status:** Complete ✓ | Failed ✗

## Publishing Phase
- **Start time:** HH:MM UTC
- **End time:** HH:MM UTC
- **Duration:** X min
- **Success count:** 20/20
- **Failed count:** 0/20
- **Avg time per job:** X sec

## Verification Phase
- **Start time:** HH:MM UTC
- **End time:** HH:MM UTC
- **Duration:** X min
- **Verified count:** 20/20

## Metrics
| Metric | Value | Target | Status |
|--------|-------|--------|--------|
| Success rate | 100% | ≥90% | ✓ |
| Verification rate | 100% | ≥90% | ✓ |
| AI cost (USD) | $0.XX | <$1.00 | ✓ |
| Total time (min) | XX | <30 | ✓ |
| Worker throughput (jobs/min) | X | >2 | ✓ |

## Issues Found
- [ ] None
- [ ] Minor (site flakiness, etc.)
- [ ] Major (feature broken, critical bug)

**Description:** [if any]

## UX Notes
- [Observe any pain points, confusing UI flows, etc.]

## Next Iteration
- [Suggested improvements]
```

### Key Metrics to Log

1. **Timestamps** (HH:MM UTC):
   - Campaign created
   - Content generation started/completed
   - Publishing started/completed
   - All verifications complete

2. **Counts**:
   - Total jobs: 20
   - Success: ?/20
   - Failed: ?/20
   - Verified: ?/20
   - Retried: ?

3. **Performance**:
   - Avg time: site connect to creation (sec)
   - Avg time: content generation per job (sec)
   - Avg time: publish per job (sec)
   - Avg time: verify per job (sec)

4. **Cost**:
   - AI tokens used
   - Cost in USD (from Anthropic dashboard)
   - Credits debited from wallet

5. **Errors** (if any):
   - Error code
   - Count
   - Root cause

---

## Appendix: SQL Queries

### Campaign Summary

```sql
SELECT 
  c.id,
  c.name,
  c.status,
  COUNT(j.id) as job_count,
  SUM(CASE WHEN j.status = 'success' THEN 1 ELSE 0 END) as success_count,
  SUM(CASE WHEN j.verified = true THEN 1 ELSE 0 END) as verified_count,
  SUM(CASE WHEN j.status = 'failed' THEN 1 ELSE 0 END) as failed_count,
  AVG(EXTRACT(EPOCH FROM (j.completed_at - j.dispatched_at)))::INT as avg_publish_sec,
  c.created_at
FROM campaigns c
LEFT JOIN jobs j ON c.id = j.campaign_id
WHERE c.name LIKE '%E2E Test%'
GROUP BY c.id
ORDER BY c.created_at DESC
LIMIT 1;
```

### Job Details

```sql
SELECT 
  id,
  status,
  error_code,
  error_message,
  result_url,
  verified,
  retry_count,
  EXTRACT(EPOCH FROM (completed_at - dispatched_at))::INT as publish_duration_sec,
  EXTRACT(EPOCH FROM (verified_at - completed_at))::INT as verify_duration_sec
FROM jobs
WHERE campaign_id = $1
ORDER BY status DESC, created_at ASC;
```

### Credits Ledger

```sql
SELECT 
  event_type,
  pool,
  SUM(delta_credits) as total_delta,
  COUNT(*) as transaction_count,
  MAX(created_at) as last_event
FROM ledger
WHERE user_id = $1
  AND created_at > NOW() - INTERVAL '1 hour'
GROUP BY event_type, pool
ORDER BY event_type;
```

---

## Sign-Off

**Operator:** ___________  
**Date:** ___________  
**Result:** ✓ PASS | ✗ FAIL  
**Next Steps:** ___________
