"""Reproducible incremental MyPage scenarios from explicit assumptions."""
import argparse
import copy
import json
import math
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


def recaptcha(assessments):
    if assessments <= 10000:
        return 0.0
    return 8.0 + max(0, assessments - 100000) * 0.001


def premium_delta(a, new, upgrades):
    return a['effectiveNetCollectionRatio'] * (
        new * a['candidatePremiumJPY'] +
        upgrades * (a['candidatePremiumJPY'] - a['existingMemberJPY']))


def scenario(model, case, mode='monthly', region='asia-northeast2', static_cache=True, profile='armor-run'):
    a = model['assumptions']
    p = model['prices'][region]
    seconds = a['pollSecondsBefore'] if mode == 'before' else a['pollSecondsAfter']
    requests = case['dau'] * a['days'] * (1 + case['hours'] * 3600 / seconds)
    documents = case['dau'] * a['days'] * a['staticDocumentLoadsPerDAUDay']
    session_requests = documents * a.get('sessionCompletionsPerDocument', 0)
    h = a['cacheHitRatio']
    if not 0 <= h <= 1:
        raise ValueError('cache hit ratio must be 0..1')
    variable = a['monthlyLookupsAverage'] if mode == 'monthly' else max(1, a['rawSegmentsPerMiss'])
    reads = requests * (4 * h + (1 - h) * (10 + variable))
    index_reads = 0 if mode == 'monthly' else requests * (1 - h) * math.ceil(a['rawIndexEntriesPerMiss'] / 1000)
    closes = a['dailyBotClosesPerBotDAU'] * case['botDAU'] * a['days']
    writer_reads = closes * a['extraRollupReadsPerClose'] if mode == 'monthly' else 0
    writer_writes = closes * a['extraRollupWritesPerClose'] if mode == 'monthly' else 0
    details = case['dau'] * a['detailOperationsPerDAUMonth']
    detail_reads = a['detailPageSize'] + 1 + a['detailControlReadsEstimate']
    detail_index = math.ceil(a['detailIndexEntriesEstimate'] / 1000)
    reads += writer_reads + details * detail_reads + session_requests * a.get('sessionCompleteReads', 0)
    index_reads += details * detail_index
    writes = writer_writes + details * a['detailWritesPerOperation']
    # Firestore is per project/day and shared with the existing workload.
    paid_reads = max(0, (reads + index_reads) / a['days'] - a['availableFirestoreReadsPerProjectDay']) * a['days']
    paid_writes = max(0, writes / a['days'] - a['availableFirestoreWritesPerProjectDay']) * a['days']
    firestore = paid_reads * p['firestoreRead100k'] / 100000 + paid_writes * p['firestoreWrite100k'] / 100000
    firestore += a['newStorageGiB'] * p['firestoreStorageGiBMonth']
    if profile not in ('hosting-run', 'armor-run'):
        raise ValueError('unknown architecture profile')
    static_bytes = a['staticBytesPerDocumentAfterCacheAverage'] if static_cache else a['staticBytesPerDocumentBefore']
    # The proposed WAF architecture has a separate Run web service, no CDN.
    # A browser-cache hit avoids a transfer; it does not eliminate the HTML load.
    static_requests = documents * (a['staticRequestsPerDocumentAfterCache'] if static_cache else a['staticRequestsPerDocumentBefore'])
    web_seconds = static_requests * a['effectiveWebBilledSecondsPerRequest'] if profile == 'armor-run' else 0
    api_requests = requests + details + session_requests
    billed_seconds = api_requests * a['effectiveBilledSecondsPerRequest'] + web_seconds
    cpu = max(0, billed_seconds * a['allocatedVCPU'] * p['cpuSecond'] - a['availableRunCPUDiscountUSDPerBillingAccountMonth'])
    ram = max(0, billed_seconds * a['allocatedGiB'] * p['gibSecond'] - a['availableRunRAMDiscountUSDPerBillingAccountMonth'])
    public_requests = api_requests + (static_requests if profile == 'armor-run' else 0)
    run_requests = max(0, public_requests * p['requestsMillion'] / 1000000 - a['availableRunRequestDiscountUSDPerBillingAccountMonth'])
    fixed = (model['prices']['armorPolicyMonthly'] + a['rules'] * model['prices']['armorRuleMonthly'] + a['hoursPerMonth'] * model['prices']['globalForwardingHour']) if profile == 'armor-run' else 0
    api_bytes = requests * a['apiResponseBytes'] + details * a['detailResponseBytesAverage'] + session_requests * a.get('sessionCompleteResponseBytes', 0)
    api_gib = api_bytes / 2**30
    armor = public_requests * model['prices']['armorGlobalRequestsMillion'] / 1000000 if profile == 'armor-run' else 0
    static_total = documents * static_bytes
    inbound_gib = (api_requests * a.get('apiRequestBytesAverage', a['requestBytesAverage']) + (static_requests * a['requestBytesAverage'] if profile == 'armor-run' else 0)) / 2**30
    outbound_gib = api_gib + (static_total / 2**30 if profile == 'armor-run' else 0)
    network = ((inbound_gib + outbound_gib) * p['lbGiBEachDirection'] + outbound_gib * p['japanEgressGiB']) if profile == 'armor-run' else 0
    # Hosting rewrites transfer API responses too; Run->Hosting is not charged
    # as a second public internet transfer in this separate current profile.
    hosting_gb = (static_total + api_bytes) / 10**9
    hosting = max(0, hosting_gb - a['availableHostingGBPerProjectMonth']) * model['prices']['hostingTransferGB'] if profile == 'hosting-run' else 0
    if profile == 'hosting-run':
        hosting += a['newHostingStorageGB'] * model['prices']['hostingStorageGBMonth']
    assessments = case['dau'] * a['days'] * case['hours'] * a['assessmentsPerAppCheckHour']
    app_check = recaptcha(a['existingOrgAssessments'] + assessments) - recaptcha(a['existingOrgAssessments'])
    log_gib = public_requests * a['responseLogBytesPerRequest'] / 2**30
    logging = (max(0, a['existingProjectLoggingGiB'] + log_gib - 50) - max(0, a['existingProjectLoggingGiB'] - 50)) * model['prices']['loggingGiB']
    production = fixed + firestore + cpu + ram + run_requests + armor + network + hosting + app_check + logging + a['otherIncrementalMonthlyUSD']
    dev_fixed = a['devInfrastructureFixedUSD'] if profile == 'armor-run' else 0
    total = production + dev_fixed + a['devVariableMonthlyUSD']
    yen = total * a['usdJpy'] * a['cloudTaxMultiplier']
    contribution = premium_delta(a, case['newPremium'], case['upgrades'])
    unit_new = a['candidatePremiumJPY'] * a['effectiveNetCollectionRatio']
    unit_upgrade = (a['candidatePremiumJPY'] - a['existingMemberJPY']) * a['effectiveNetCollectionRatio']
    return dict(name=case['name'], profile=profile, mode=mode, requests=requests, sessionRequests=session_requests, allAPIRequests=api_requests, webRequests=static_requests, reads=reads, indexReads=index_reads, extraWrites=writes,
                productionUSD=production, devFixedUSD=dev_fixed, totalUSD=total, totalJPY=yen,
                firestoreUSD=firestore, runUSD=cpu + ram + run_requests, hostingUSD=hosting, appCheckUSD=app_check,
                armorUSD=armor, networkUSD=network, loggingUSD=logging,
                premiumContributionJPY=contribution, projectedProfitJPY=a['baselineProfitJPY'] - yen + contribution,
                newPremiumToMaintainBase=math.ceil(yen / unit_new), upgradesToMaintainBase=math.ceil(yen / unit_upgrade),
                averageVariableUSDPerMyPageDAU=(firestore + cpu + ram + run_requests + armor + network + hosting + app_check + logging) / max(1, case['dau']),
                historyFirestoreOnlyUSDPerDAU=a['detailOperationsPerDAUMonth'] * ((detail_reads + detail_index) * p['firestoreRead100k'] / 100000 + a['detailWritesPerOperation'] * p['firestoreWrite100k'] / 100000))


def validate(model):
    a = model['assumptions']
    for value in a.values():
        if type(value) not in (int, float) or not math.isfinite(value) or value < 0:
            raise ValueError('assumptions must be finite nonnegative numbers')
    for key in ('days', 'pollSecondsBefore', 'pollSecondsAfter', 'usdJpy', 'cloudTaxMultiplier', 'allocatedVCPU', 'allocatedGiB'):
        if a[key] <= 0:
            raise ValueError('positive assumption required: ' + key)
    if not 0 < a['effectiveNetCollectionRatio'] <= 1 or not 0 <= a['cacheHitRatio'] <= 1 or a['candidatePremiumJPY'] <= a['existingMemberJPY']:
        raise ValueError('invalid membership/cache assumptions')
    if not 1 <= a['monthlyLookupsAverage'] <= 2 or len(model['scenarios']) < 3:
        raise ValueError('monthly lookup bound / three scenarios required')
    if not 1 <= a['detailPageSize'] <= 100 or int(a['detailPageSize']) != a['detailPageSize']:
        raise ValueError('detail page size must be an integer in 1..100')
    for case in model['scenarios']:
        for key in ('dau', 'hours', 'botDAU', 'newPremium', 'upgrades'):
            if type(case[key]) not in (int, float) or not math.isfinite(case[key]) or case[key] < 0:
                raise ValueError('invalid scenario')


def self_test():
    # Independent fixed fixture: editable --input data must not change expected
    # regression values (31-day months and unused shared credits are valid).
    a = dict(candidatePremiumJPY=690, existingMemberJPY=290, effectiveNetCollectionRatio=0.7,
             days=30, pollSecondsBefore=60, pollSecondsAfter=300, cacheHitRatio=0,
             monthlyLookupsAverage=1.2, rawSegmentsPerMiss=20, rawIndexEntriesPerMiss=1000,
             dailyBotClosesPerBotDAU=4, extraRollupReadsPerClose=5.05, extraRollupWritesPerClose=2.05,
             detailOperationsPerDAUMonth=0, detailPageSize=50, detailControlReadsEstimate=32,
             detailIndexEntriesEstimate=20000, detailResponseBytesAverage=32768, detailWritesPerOperation=2,
             availableFirestoreReadsPerProjectDay=0, availableFirestoreWritesPerProjectDay=0,
             newStorageGiB=0, effectiveBilledSecondsPerRequest=0.2, allocatedVCPU=1, allocatedGiB=0.5,
             availableRunCPUDiscountUSDPerBillingAccountMonth=0, availableRunRAMDiscountUSDPerBillingAccountMonth=0,
             availableRunRequestDiscountUSDPerBillingAccountMonth=0, rules=8, hoursPerMonth=730,
             apiResponseBytes=8192, requestBytesAverage=1024, staticDocumentLoadsPerDAUDay=1,
             staticBytesPerDocumentAfterCacheAverage=200000, staticBytesPerDocumentBefore=4500000,
             staticRequestsPerDocumentAfterCache=1.2, staticRequestsPerDocumentBefore=7,
             effectiveWebBilledSecondsPerRequest=0.05, availableHostingGBPerProjectMonth=0, newHostingStorageGB=0,
             assessmentsPerAppCheckHour=2, existingOrgAssessments=10000, responseLogBytesPerRequest=0,
             existingProjectLoggingGiB=50, otherIncrementalMonthlyUSD=0, devInfrastructureFixedUSD=31.25,
             devVariableMonthlyUSD=0, usdJpy=150, cloudTaxMultiplier=1, baselineProfitJPY=30000)
    p = dict(firestoreRead100k=0.038, firestoreWrite100k=0.115, firestoreStorageGiBMonth=0.195,
             cpuSecond=0.000024, gibSecond=0.0000025, requestsMillion=0.4, lbGiBEachDirection=0.012, japanEgressGiB=0.12)
    model = dict(assumptions=a, prices={'asia-northeast2': p, 'asia-southeast2': dict(p, cpuSecond=0.0000336, gibSecond=0.0000035),
                 'armorPolicyMonthly':5, 'armorRuleMonthly':1, 'globalForwardingHour':0.025, 'armorGlobalRequestsMillion':0.75,
                 'hostingTransferGB':0.15, 'hostingStorageGBMonth':0.026, 'loggingGiB':0.5})
    assert premium_delta(a, 0, 1) == 280, 'upgrade must contribute only the 400-yen difference times net ratio'
    assert math.isclose(premium_delta(a, 1, 0), 483), 'new membership differs from an upgrade'
    assert [recaptcha(n) for n in [10000, 10001, 100000, 100001]] == [0, 8, 8, 8.001]
    c = dict(name='fixture', dau=200, hours=2, botDAU=500, newPremium=0, upgrades=0)
    old, post = scenario(model, c, 'before'), scenario(model, c, 'monthly')
    assert old['requests'] == 726000 and post['requests'] == 150000
    assert old['appCheckUSD'] == post['appCheckUSD'], 'polling does not determine assessments'
    changed = copy.deepcopy(model); changed['assumptions']['rawSegmentsPerMiss'] = 1000
    assert scenario(changed, c, 'monthly')['reads'] == post['reads'], 'normal graphs do not depend on raw count'
    assert scenario(model, c, 'monthly', 'asia-southeast2')['runUSD'] > post['runUSD']
    changed = copy.deepcopy(model); changed['assumptions']['existingProjectLoggingGiB'] = 60
    assert scenario(changed, c)['loggingUSD'] == 0, 'existing logging expense is not new Web expense'
    changed['assumptions']['responseLogBytesPerRequest'] = 2**30 / 150000
    assert math.isclose(scenario(changed,c)['loggingUSD'], 0.5 * (1 + post['webRequests']/150000))
    hosted = scenario(model,c,profile='hosting-run')
    assert hosted['armorUSD'] == hosted['networkUSD'] == hosted['devFixedUSD'] == 0 and hosted['hostingUSD'] > 0
    assert post['hostingUSD'] == 0 and post['armorUSD'] > 0
    with_session = copy.deepcopy(model); with_session['assumptions'].update(sessionCompletionsPerDocument=1, sessionCompleteReads=4, sessionCompleteResponseBytes=344)
    auth = scenario(with_session, c)
    assert auth['sessionRequests'] == 6000 and auth['allAPIRequests'] == 156000
    assert scenario(with_session,c,'before')['sessionRequests'] == auth['sessionRequests'], 'poll frequency does not determine session completions'
    assert auth['reads'] - post['reads'] == 24000, 'session transaction reads are separate from overview authorization reads'
    assert math.isclose(auth['runUSD'] - post['runUSD'], 6000 * (0.2 * (0.000024 + 0.5 * 0.0000025) + 0.4 / 1000000))
    assert auth['appCheckUSD'] == post['appCheckUSD'], 'session requests can reuse App Check proof'
    larger_headers=copy.deepcopy(with_session); larger_headers['assumptions']['apiRequestBytesAverage']=3303
    assert math.isclose(scenario(larger_headers,c)['networkUSD']-auth['networkUSD'], 156000*(3303-1024)/2**30*0.012), 'API header size must not be applied to static requests'
    changed = copy.deepcopy(model); changed['assumptions']['detailOperationsPerDAUMonth'] = 1
    assert math.isclose(scenario(changed,c)['armorUSD'] - post['armorUSD'], 200 * 0.75 / 1000000)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--input', type=Path, default=ROOT/'docs/mypage/cost-model.json')
    parser.add_argument('--output', type=Path)
    parser.add_argument('--test', action='store_true')
    args = parser.parse_args()
    model = json.loads(args.input.read_text(encoding='utf-8'))
    validate(model)
    self_test()
    if args.test:
        print('Cost model invariants passed')
        return
    lines = ['# MyPage incremental cost scenarios', '',
             f"Official prices checked {model['verifiedAtJST']} JST. Scenario prices use Production Osaka (asia-northeast2); DEV price inputs use Jakarta (asia-southeast2). Scenario values below are estimates, not measured invoices.", '',
             'See assumptions and exclusions in `docs/mypage/cost-notes.md`. **Adopted target: Firebase Hosting + Cloud Run Go BFF (no managed WAF/LB).** The main Armor + global LB + Run Web/API table is a retained, **not adopted** cost-comparison scenario; current Hosting + Run is shown separately below.', '',
             '| Scale | 60s/raw USD | 5m/raw USD | 5m/month no-cache USD | +static cache USD | JPY cost | Profit JPY | New members to cover | Upgrades to cover |',
             '|---|---:|---:|---:|---:|---:|---:|---:|---:|']
    for case in model['scenarios']:
        before = scenario(model, case, 'before', static_cache=False)
        poll = scenario(model, case, 'poll', static_cache=False)
        monthly_no_cache = scenario(model, case, static_cache=False)
        after = scenario(model, case)
        lines.append(f"| {case['name']} | {before['totalUSD']:.2f} | {poll['totalUSD']:.2f} | {monthly_no_cache['totalUSD']:.2f} | {after['totalUSD']:.2f} | {after['totalJPY']:.0f} | {after['projectedProfitJPY']:.0f} | {after['newPremiumToMaintainBase']} | {after['upgradesToMaintainBase']} |")
    lines += ['', '## Architecture and component separation (after monthly + browser cache)', '',
              f"Total includes a separate DEV variable allowance of USD{model['assumptions']['devVariableMonthlyUSD']:.2f}/month in addition to the displayed DEV fixed column.", '',
              '| Profile / scale | Run USD | Firestore USD | Hosting USD | Armor variable USD | Network USD | App Check USD | Prod USD | DEV fixed USD | Total USD |',
              '|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|']
    for profile in ('hosting-run','armor-run'):
        for case in model['scenarios']:
            r = scenario(model,case,profile=profile)
            lines.append(f"| {profile}/{case['name']} | {r['runUSD']:.2f} | {r['firestoreUSD']:.2f} | {r['hostingUSD']:.2f} | {r['armorUSD']:.2f} | {r['networkUSD']:.2f} | {r['appCheckUSD']:.2f} | {r['productionUSD']:.2f} | {r['devFixedUSD']:.2f} | {r['totalUSD']:.2f} |")
    lines += ['', '## Per-person variable and additional detail cost (USD/month)', '',
              'DAU-normalized variable cost includes all-Bot writer overhead divided by MyPage DAU; it is not a marginal user bill. Details add Run, WAF, network, quota reads/writes. No new App Check assessment is assumed just for opening a detail page.', '',
              '| Scale | Average variable / MyPage DAU | +10 detail operations / person | +100 max-size operations / person |', '|---|---:|---:|---:|']
    for case in model['scenarios']:
        base = scenario(model,case)
        regular = copy.deepcopy(model); regular['assumptions']['detailOperationsPerDAUMonth'] = 10
        maximum = copy.deepcopy(model); maximum['assumptions'].update(detailOperationsPerDAUMonth=100, detailPageSize=100, detailResponseBytesAverage=131072)
        extra = (scenario(regular,case)['totalUSD'] - base['totalUSD']) / max(1,case['dau'])
        heavy = (scenario(maximum,case)['totalUSD'] - base['totalUSD']) / max(1,case['dau'])
        lines.append(f"| {case['name']} | {base['averageVariableUSDPerMyPageDAU']:.5f} | {extra:.5f} | {heavy:.5f} |")
    lines += ['', '## Sensitivity: standard scale after monthly model', '', '| Input | Value | Total USD | Cost JPY | Profit JPY |', '|---|---:|---:|---:|---:|']
    for key, values in {'usdJpy':[130,150,170], 'effectiveBilledSecondsPerRequest':[0.05,0.2,1], 'staticDocumentLoadsPerDAUDay':[0.5,1,3], 'assessmentsPerAppCheckHour':[0.5,2,4], 'staticBytesPerDocumentAfterCacheAverage':[200000,1500000,4500000]}.items():
        for value in values:
            changed = copy.deepcopy(model); changed['assumptions'][key] = value
            r = scenario(changed, model['scenarios'][1])
            lines.append(f"| {key} | {value} | {r['totalUSD']:.2f} | {r['totalJPY']:.0f} | {r['projectedProfitJPY']:.0f} |")
    for hours in (0.5,2,4,8):
        r = scenario(model,dict(model['scenarios'][1],hours=hours))
        lines.append(f"| visible hours/day | {hours} | {r['totalUSD']:.2f} | {r['totalJPY']:.0f} | {r['projectedProfitJPY']:.0f} |")
    lines += ['', 'Raw history sensitivity (growth, free quota exhausted; assumed index scans and hypothetical user distribution):', '', '| Segments/miss | 60s/raw USD | 5m/raw USD | 5m/month USD |', '|---|---:|---:|---:|']
    for n in [0,10,100,500,1000]:
        changed = copy.deepcopy(model); changed['assumptions']['rawSegmentsPerMiss'] = n
        c = model['scenarios'][2]
        lines.append(f"| {n} | {scenario(changed,c,'before',static_cache=False)['totalUSD']:.2f} | {scenario(changed,c,'poll',static_cache=False)['totalUSD']:.2f} | {scenario(changed,c)['totalUSD']:.2f} |")
    lines += ['', 'Membership sensitivity (standard scale; baseline profit is counted once):', '', '| New | Upgrades | Incremental net JPY | Profit JPY |', '|---:|---:|---:|---:|']
    for new, upgrades in [(0,0),(0,10),(0,40),(10,30),(50,0),(0,100)]:
        case = dict(model['scenarios'][1], newPremium=new, upgrades=upgrades)
        r = scenario(model,case)
        lines.append(f"| {new} | {upgrades} | {r['premiumContributionJPY']:.0f} | {r['projectedProfitJPY']:.0f} |")
    text = '\n'.join(lines)+'\n'
    if args.output:
        args.output.write_text(text,encoding='utf-8')
    else:
        print(text)


if __name__ == '__main__':
    main()
