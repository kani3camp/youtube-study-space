# MyPage incremental cost scenarios

Official prices checked 2026-10-24 JST. Scenario prices use Production Osaka (asia-northeast2); controlled DEV measurements use Jakarta (asia-southeast2). Scenario values below are estimates, not measured invoices.

See [assumptions and exclusions](cost-notes.md). Main table: proposed Armor + global LB + Run Web/API, no CDN. Current Hosting + Run is a separate profile below.

| Scale | 60s/raw USD | 5m/raw USD | 5m/month no-cache USD | +static cache USD | JPY cost | Profit JPY | New members to cover | Upgrades to cover |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| small | 71.89 | 71.06 | 71.08 | 70.75 | 10613 | 22187 | 22 | 38 |
| standard | 87.78 | 76.82 | 75.95 | 72.69 | 10904 | 32326 | 23 | 39 |
| growth | 581.78 | 417.41 | 401.73 | 369.17 | 55376 | 78924 | 115 | 198 |

## DEV direct-reader measurements (2026-10-24)

Jakarta Firestore via local SDK. These are controlled fixtures, not the population average, HTTP BFF, invoice reads, or Cloud Run CPU. See [trial and limits](dev-cost-measurement-20261024.md).

| Fixture raw docs | Raw returned | Analyze document scans | Analyze index scans | Analyze read operations | Monthly returned / lookups | Raw / monthly SDK ms |
|---|---:|---:|---:|---:|---:|---:|
| 0 | 0 | 0 | 0 | 1 | 1 / 1 | 769 / 762 |
| 10 | 10 | 10 | 10 | 11 | 1 / 1 | 783 / 810 |
| 100 | 100 | 100 | 100 | 101 | 1 / 1 | 803 / 747 |
| 500 | 500 | 500 | 500 | 501 | 1 / 1 | 988 / 758 |
| 1000 | 1000 | 1000 | 1000 | 1001 | 1 / 1 | 1173 / 772 |
| 1001 | 1001 | 1001 | 1001 | 1003 | 1 / 1 | 1064 / 748 |
| 2 (day boundary + month rebuild) | 1 | 1 | 1 | 2 | 1 / 1 | 775 / 753 |

Raw 1001 returns HISTORY_LIMIT_EXCEEDED; it is not an exact graph baseline. At 0 docs, unknown coverage stays unavailable for the registered fixture. The separate authenticated unregistered case displays a normal zero. In the boundary fixture, the month-crossing row was outside the recent seven-day window: its rebuild was checked, but a live recent-window month crossing and two-month lookup were not measured.

For the 1000-doc fixture, adding the source-inventory estimate of 10 fixed read equivalents to Analyze gives 1011 versus 11 with one month lookup (about 98.9% fewer). This is a derived Firestore cache-miss comparison, not measured HTTP billing. Writer overhead and other services are included separately in the scenarios. Default raw average 20, average monthly lookups 1.2, cache ratio and billing time remain assumptions.


## Authenticated DEV cohort and Run allocation

Same owned 180-row fixture: raw and monthly showed recorded today/week and lifetime 3 minutes; SDK comparison was 180 seconds. 6 overview GETs (including post-cleanup zero) and 3 separate session-completion POSTs were observed in the allocation window. Full response bodies and internal cache-hit counters were not collected.

Run billed allocation: 1.1 instance seconds / 9 requests, 1 vCPU and 0.5 GiB. This is not per-request process CPU. Raw requests: 205.6 / 35.0 / 32.9 ms; monthly: 140.6 / 32.0 ms. These few samples are not percentiles or population latency.

API request messages averaged 3303 bytes and responses 2092.4 bytes for the five nonempty overview requests (Cloud Logging sizes include headers). The model uses the measured API inbound reference separately from static request bytes, retains an 8192-byte response budget, and adds one source-estimated session completion per document with four Firestore read equivalents. Initial OAuth/new-account writes, retries and organization billing remain separate.


## Architecture and component separation (after monthly + browser cache)

| Profile / scale | Run USD | Firestore USD | Hosting USD | Armor variable USD | Network USD | App Check USD | Prod USD | DEV fixed USD | Total USD |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| hosting-run/small | 0.07 | 0.15 | 0.03 | 0.00 | 0.00 | 8.00 | 8.25 | 0.00 | 8.25 |
| hosting-run/standard | 0.85 | 0.90 | 0.36 | 0.00 | 0.00 | 8.00 | 10.12 | 0.00 | 10.12 |
| hosting-run/growth | 12.43 | 10.57 | 4.53 | 0.00 | 0.00 | 278.00 | 305.52 | 0.00 | 305.52 |
| armor-run/small | 0.07 | 0.15 | 0.00 | 0.01 | 0.03 | 8.00 | 39.50 | 31.25 | 70.75 |
| armor-run/standard | 0.86 | 0.90 | 0.00 | 0.12 | 0.30 | 8.00 | 41.44 | 31.25 | 72.69 |
| armor-run/growth | 12.55 | 10.57 | 0.00 | 1.76 | 3.80 | 278.00 | 337.92 | 31.25 | 369.17 |

## Per-person variable and additional detail cost (USD/month)

DAU-normalized variable cost includes all-Bot writer overhead divided by MyPage DAU; it is not a marginal user bill. Details add Run, WAF, network, quota reads/writes. No new App Check assessment is assumed just for opening a detail page.

| Scale | Average variable / MyPage DAU | +10 detail operations / person | +100 max-size operations / person |
|---|---:|---:|---:|
| small | 0.41275 | 0.00052 | 0.00828 |
| standard | 0.05097 | 0.00052 | 0.00828 |
| growth | 0.15334 | 0.00052 | 0.00828 |

## Sensitivity: standard scale after monthly model

| Input | Value | Total USD | Cost JPY | Profit JPY |
|---|---:|---:|---:|---:|
| usdJpy | 130 | 72.69 | 9450 | 33780 |
| usdJpy | 150 | 72.69 | 10904 | 32326 |
| usdJpy | 170 | 72.69 | 12358 | 30872 |
| effectiveBilledSecondsPerRequest | 0.05 | 72.10 | 10815 | 32415 |
| effectiveBilledSecondsPerRequest | 0.2 | 72.69 | 10904 | 32326 |
| effectiveBilledSecondsPerRequest | 1 | 75.84 | 11377 | 31853 |
| staticDocumentLoadsPerDAUDay | 0.5 | 72.59 | 10888 | 32342 |
| staticDocumentLoadsPerDAUDay | 1 | 72.69 | 10904 | 32326 |
| staticDocumentLoadsPerDAUDay | 3 | 73.12 | 10968 | 32262 |
| assessmentsPerAppCheckHour | 0.5 | 72.69 | 10904 | 32326 |
| assessmentsPerAppCheckHour | 2 | 72.69 | 10904 | 32326 |
| assessmentsPerAppCheckHour | 4 | 72.69 | 10904 | 32326 |
| staticBytesPerDocumentAfterCacheAverage | 200000 | 72.69 | 10904 | 32326 |
| staticBytesPerDocumentAfterCacheAverage | 1500000 | 73.65 | 11048 | 32182 |
| staticBytesPerDocumentAfterCacheAverage | 4500000 | 75.87 | 11380 | 31850 |
| visible hours/day | 0.5 | 71.45 | 10718 | 32512 |
| visible hours/day | 2 | 72.69 | 10904 | 32326 |
| visible hours/day | 4 | 74.35 | 11152 | 32078 |
| visible hours/day | 8 | 83.66 | 12549 | 30681 |

Raw history sensitivity (growth, free quota exhausted; measured index scans where available, hypothetical user distribution):

| Segments/miss | 60s/raw USD | 5m/raw USD | 5m/month USD |
|---|---:|---:|---:|
| 0 | 499.24 | 400.54 | 369.17 |
| 10 | 540.51 | 408.98 | 369.17 |
| 100 | 911.92 | 484.90 | 369.17 |
| 500 | 2562.64 | 822.34 | 369.17 |
| 1000 | 4626.04 | 1244.14 | 369.17 |

Membership sensitivity (standard scale; baseline profit is counted once):

| New | Upgrades | Incremental net JPY | Profit JPY |
|---:|---:|---:|---:|
| 0 | 0 | 0 | 19096 |
| 0 | 10 | 2800 | 21896 |
| 0 | 40 | 11200 | 30296 |
| 10 | 30 | 13230 | 32326 |
| 50 | 0 | 24150 | 43246 |
| 0 | 100 | 28000 | 47096 |
