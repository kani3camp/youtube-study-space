# MyPage incremental cost scenarios

Official prices checked 2026-10-24 JST. Scenario prices use Production Osaka (asia-northeast2); controlled DEV measurements use Jakarta (asia-southeast2). Scenario values below are estimates, not measured invoices.

See [assumptions and exclusions](cost-notes.md). Main table: proposed Armor + global LB + Run Web/API, no CDN. Current Hosting + Run is a separate profile below.

| Scale | 60s/raw USD | 5m/raw USD | 5m/month no-cache USD | +static cache USD | JPY cost | Profit JPY | New members to cover | Upgrades to cover |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| small | 71.88 | 71.06 | 71.08 | 70.75 | 10612 | 22188 | 22 | 38 |
| standard | 87.71 | 76.77 | 75.90 | 72.64 | 10896 | 32334 | 23 | 39 |
| growth | 581.03 | 416.89 | 401.21 | 368.65 | 55297 | 79003 | 115 | 198 |

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
| 2 (JST boundaries) | 1 | 1 | 1 | 2 | 1 / 1 | 775 / 753 |

Raw 1001 returns HISTORY_LIMIT_EXCEEDED; it is not an exact graph baseline. At 0 docs, unknown coverage stays unavailable for the registered fixture. The separate authenticated unregistered case displays a normal zero.

For the 1000-doc fixture, adding the source-inventory estimate of 10 fixed read equivalents to Analyze gives 1011 versus 11 with one month lookup (about 98.9% fewer). This is a derived Firestore cache-miss comparison, not measured HTTP billing. Writer overhead and other services are included separately in the scenarios. Default raw average 20, average monthly lookups 1.2, cache ratio and billing time remain assumptions.


## Architecture and component separation (after monthly + browser cache)

| Profile / scale | Run USD | Firestore USD | Hosting USD | Armor variable USD | Network USD | App Check USD | Prod USD | DEV fixed USD | Total USD |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| hosting-run/small | 0.06 | 0.15 | 0.03 | 0.00 | 0.00 | 8.00 | 8.25 | 0.00 | 8.25 |
| hosting-run/standard | 0.82 | 0.89 | 0.36 | 0.00 | 0.00 | 8.00 | 10.08 | 0.00 | 10.08 |
| hosting-run/growth | 12.10 | 10.47 | 4.53 | 0.00 | 0.00 | 278.00 | 305.10 | 0.00 | 305.10 |
| armor-run/small | 0.06 | 0.15 | 0.00 | 0.01 | 0.03 | 8.00 | 39.50 | 31.25 | 70.75 |
| armor-run/standard | 0.83 | 0.89 | 0.00 | 0.12 | 0.30 | 8.00 | 41.39 | 31.25 | 72.64 |
| armor-run/growth | 12.22 | 10.47 | 0.00 | 1.72 | 3.74 | 278.00 | 337.40 | 31.25 | 368.65 |

## Per-person variable and additional detail cost (USD/month)

DAU-normalized variable cost includes all-Bot writer overhead divided by MyPage DAU; it is not a marginal user bill. Details add Run, WAF, network, quota reads/writes. No new App Check assessment is assumed just for opening a detail page.

| Scale | Average variable / MyPage DAU | +10 detail operations / person | +100 max-size operations / person |
|---|---:|---:|---:|
| small | 0.41250 | 0.00052 | 0.00828 |
| standard | 0.05071 | 0.00052 | 0.00828 |
| growth | 0.15307 | 0.00052 | 0.00828 |

## Sensitivity: standard scale after monthly model

| Input | Value | Total USD | Cost JPY | Profit JPY |
|---|---:|---:|---:|---:|
| usdJpy | 130 | 72.64 | 9444 | 33786 |
| usdJpy | 150 | 72.64 | 10896 | 32334 |
| usdJpy | 170 | 72.64 | 12349 | 30881 |
| effectiveBilledSecondsPerRequest | 0.05 | 72.07 | 10811 | 32419 |
| effectiveBilledSecondsPerRequest | 0.2 | 72.64 | 10896 | 32334 |
| effectiveBilledSecondsPerRequest | 1 | 75.67 | 11351 | 31879 |
| staticDocumentLoadsPerDAUDay | 0.5 | 72.56 | 10884 | 32346 |
| staticDocumentLoadsPerDAUDay | 1 | 72.64 | 10896 | 32334 |
| staticDocumentLoadsPerDAUDay | 3 | 72.97 | 10946 | 32284 |
| assessmentsPerAppCheckHour | 0.5 | 72.64 | 10896 | 32334 |
| assessmentsPerAppCheckHour | 2 | 72.64 | 10896 | 32334 |
| assessmentsPerAppCheckHour | 4 | 72.64 | 10896 | 32334 |
| staticBytesPerDocumentAfterCacheAverage | 200000 | 72.64 | 10896 | 32334 |
| staticBytesPerDocumentAfterCacheAverage | 1500000 | 73.60 | 11040 | 32190 |
| staticBytesPerDocumentAfterCacheAverage | 4500000 | 75.81 | 11372 | 31858 |
| visible hours/day | 0.5 | 71.40 | 10711 | 32519 |
| visible hours/day | 2 | 72.64 | 10896 | 32334 |
| visible hours/day | 4 | 74.30 | 11144 | 32086 |
| visible hours/day | 8 | 83.60 | 12540 | 30690 |

Raw history sensitivity (growth, free quota exhausted; measured index scans where available, hypothetical user distribution):

| Segments/miss | 60s/raw USD | 5m/raw USD | 5m/month USD |
|---|---:|---:|---:|
| 0 | 498.50 | 400.02 | 368.65 |
| 10 | 539.77 | 408.45 | 368.65 |
| 100 | 911.18 | 484.38 | 368.65 |
| 500 | 2561.90 | 821.82 | 368.65 |
| 1000 | 4625.30 | 1243.62 | 368.65 |

Membership sensitivity (standard scale; baseline profit is counted once):

| New | Upgrades | Incremental net JPY | Profit JPY |
|---:|---:|---:|---:|
| 0 | 0 | 0 | 19104 |
| 0 | 10 | 2800 | 21904 |
| 0 | 40 | 11200 | 30304 |
| 10 | 30 | 13230 | 32334 |
| 50 | 0 | 24150 | 43254 |
| 0 | 100 | 28000 | 47104 |
