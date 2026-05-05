# E2E Test Run #1 — 20 Backlinks Live Smoke Test

**Date:** [YYYY-MM-DD]  
**Time (UTC):** [HH:MM]  
**Operator:** [Name]  
**Environment:** local | production  
**Branch:** [git branch]

---

## Setup Phase

| Item | Value | Status |
|------|-------|--------|
| WP sites connected | _/20 | [ ] Complete |
| Sites status | all "connected" | [ ] Verified |
| User wallet credits | ___ | [ ] ≥50 credits |
| Campaign created | [Campaign ID] | [ ] OK |
| Campaign status | running | [ ] OK |

**Start time:** __:__ UTC  
**End time:** __:__ UTC  
**Duration:** __ min

---

## Content Generation Phase

| Metric | Value | Target | Status |
|--------|-------|--------|--------|
| Jobs enqueued | _/20 | 20 | [ ] Pass |
| AI generation started | __:__ UTC | - | [ ] OK |
| AI generation completed | __:__ UTC | <5 min | [ ] OK |
| Jobs with content_ready | _/20 | 20 | [ ] Pass |
| AI cost (USD) | $__ | <$0.50 | [ ] Pass |

**Issues:** [ ] None [ ] [Describe]

---

## Publishing Phase

| Metric | Value | Target | Status |
|--------|-------|--------|--------|
| Publishing started | __:__ UTC | - | [ ] OK |
| Publishing completed | __:__ UTC | - | [ ] OK |
| Jobs succeeded | _/20 | ≥18 | [ ] Pass |
| Jobs failed | _/20 | ≤2 | [ ] Pass |
| Jobs in_progress (stragglers) | _ | 0 | [ ] OK |
| Avg publish time per job | _ sec | <60 | [ ] OK |
| Max publish time (slowest job) | _ sec | <120 | [ ] OK |

**Failed jobs breakdown:**

| Job ID | WP Site | Status | Error Code | Error Message |
|--------|---------|--------|------------|---------------|
| [id] | [site] | failed | [code] | [msg] |
| (add rows as needed) | | | | |

**Issues:** [ ] None [ ] [Describe]

---

## Verification Phase

| Metric | Value | Target | Status |
|--------|-------|--------|--------|
| Verification started | __:__ UTC | - | [ ] OK |
| Verification completed | __:__ UTC | - | [ ] OK |
| Jobs verified | _/20 | ≥18 | [ ] Pass |
| Avg verify time per job | _ sec | <30 | [ ] OK |
| Unverified jobs | _ | ≤2 | [ ] OK |

**Unverified jobs (if any):**

| Job ID | Result URL | Reason |
|--------|-----------|--------|
| [id] | [url] | [reason] |
| (add rows as needed) | | |

**Manual spot-check URLs (tested in browser):**

- [ ] [URL 1] — Status: __ (accessible/broken/DNS lag)
- [ ] [URL 2] — Status: __
- [ ] [URL 3] — Status: __

**Issues:** [ ] None [ ] [Describe]

---

## Overall Metrics

| Category | Metric | Value | Target | Status |
|----------|--------|-------|--------|--------|
| Success | Success rate | _%  | ≥90% | [ ] Pass |
| Success | Verification rate | _%  | ≥90% | [ ] Pass |
| Cost | AI cost (total USD) | $__ | <$1.00 | [ ] Pass |
| Time | Total E2E time | __ min | <30 | [ ] Pass |
| Time | Setup time | __ min | - | - |
| Time | Content gen time | __ min | <5 | - |
| Time | Publish time | __ min | <15 | - |
| Time | Verify time | __ min | <10 | - |
| Throughput | Avg jobs/min (publish) | _ | >2 | [ ] OK |
| Data | No duplicate posts | [ ] Verified | - | [ ] OK |
| Data | No orphan jobs | [ ] Verified | - | [ ] OK |
| Wallet | Credits debited correctly | _ credits | ~45 | [ ] OK |

**Actual total time:** [Setup + Content + Publish + Verify]

---

## Pass/Fail Decision

**All criteria met?** [ ] YES ✓ [ ] NO ✗

**Summary:**
- Success count: _/20 (threshold: ≥18) **[PASS/FAIL]**
- Verified count: _/20 (threshold: ≥18) **[PASS/FAIL]**
- AI cost: $_ (threshold: <$1.00) **[PASS/FAIL]**
- Total time: __ min (threshold: <30 min) **[PASS/FAIL]**

---

## Observations & UX Feedback

### What Went Well
- [ ] Site connection was smooth
- [ ] Content generation was fast
- [ ] Publishing was reliable
- [ ] UI feedback was clear
- [ ] Error messages were helpful
- [Other:]

### Issues Found

#### Issue #1: [Title]
**Severity:** Critical | High | Medium | Low  
**Description:** [What happened?]  
**Steps to reproduce:**  
1. [Step 1]
2. [Step 2]  

**Expected:** [What should happen]  
**Actual:** [What did happen]  

**Root cause:** [Best guess]  

**Suggested fix:** [Solution]  

#### Issue #2: [Title]
[Repeat format]

### Performance Bottlenecks
- [ ] Site connection was slow (>10 sec per site)
- [ ] Content generation was bottlenecked (rate limited, API slow)
- [ ] Publishing was slow (worker max concurrency too low)
- [ ] Verification was slow (too many retries, DNS lag)
- [Other:]

### UX Pain Points
- [ ] Campaign form was confusing
- [ ] Progress feedback was unclear
- [ ] Error messages were vague
- [ ] Site connection errors weren't user-friendly
- [Other:]

---

## Follow-Up Actions

### Bugs to File
- [ ] [Issue 1]
- [ ] [Issue 2]

### Improvements to Consider
1. [Enhancement 1]
2. [Enhancement 2]

### For Next Run
- [ ] Document any new failure modes
- [ ] Test retry logic with 1–2 deliberate failures
- [ ] Measure rate-limit behavior under load

---

## Artifact Links

- **Campaign ID:** [uuid]
- **Campaign URL:** http://localhost:3000/campaigns/[uuid] or https://sbf-web.fly.dev/campaigns/[uuid]
- **API logs:** [link to log collection, if applicable]
- **Screenshots:** [folder link, if any]

---

## Sign-Off

**Operator Name:** ___________________  
**Date Signed:** ___________________  
**Result:** ✓ PASS | ✗ FAIL  

**Next Steps:**
- [ ] All pass criteria met → **SHIP IT** ✓
- [ ] Minor issues found → Document + file low-priority bugs
- [ ] Major issues found → Debug + rerun before shipping
- [ ] Blockers found → Escalate to team lead

---

## Appendix: Raw Data Export

### SQL Export

```sql
-- Campaign summary
SELECT * FROM campaigns WHERE id = '[CAMPAIGN_ID]';

-- Jobs summary
SELECT 
  COUNT(*) as total,
  SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success_count,
  SUM(CASE WHEN verified = true THEN 1 ELSE 0 END) as verified_count,
  SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed_count
FROM jobs
WHERE campaign_id = '[CAMPAIGN_ID]';

-- Detailed job listing
SELECT 
  id, status, verified, error_code, error_message,
  result_url, retry_count,
  created_at, dispatched_at, completed_at, verified_at
FROM jobs
WHERE campaign_id = '[CAMPAIGN_ID]'
ORDER BY created_at ASC;

-- Wallet ledger
SELECT * FROM ledger
WHERE user_id = '[USER_ID]'
  AND created_at > NOW() - INTERVAL '2 hours'
ORDER BY created_at DESC;
```

### API Export

```bash
# Export campaign summary
curl -s "http://localhost:8080/api/v1/campaigns/[CAMPAIGN_ID]" \
  -H "Authorization: Bearer [AUTH_TOKEN]" | jq '.' > campaign-summary.json

# Export all jobs
curl -s "http://localhost:8080/api/v1/campaigns/[CAMPAIGN_ID]/jobs?limit=100" \
  -H "Authorization: Bearer [AUTH_TOKEN]" | jq '.' > jobs-full.json

# Save timestamp
date -u +"%Y-%m-%dT%H:%M:%SZ" > test-timestamp.txt
```

---

## Notes

[Free-form field for additional observations, context, or concerns]

