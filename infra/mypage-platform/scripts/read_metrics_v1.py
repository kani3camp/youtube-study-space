"""Exact DEV metrics toggle using Cloud Run's resourceVersion contract."""
from copy import deepcopy
import json
import time
import urllib.error
import urllib.request

import foundation_contract as c
import runtime_contract as r
import read_metrics_transition as m

URL = "https://asia-southeast2-run.googleapis.com/apis/serving.knative.dev/v1/namespaces/" + c.PROJECT + "/services/mypage-api"
OUTPUT_ANNOTATIONS = {"serving.knative.dev/lastModifier", "run.googleapis.com/operation-id"}


def request(token, body=None):
    import read_metrics_live as live
    c.need(type(token) is str and bool(token), "metrics-v1-credential")
    req = urllib.request.Request(URL, data=c.canonical(body) if body is not None else None,
                                 method="PUT" if body is not None else "GET",
                                 headers={"Authorization":"Bearer " + token, "Content-Type":"application/json"})
    try:
        with urllib.request.build_opener(live.NoRedirect()).open(req, timeout=30) as response:
            raw = response.read(1048577)
            c.need(len(raw) <= 1048576, "metrics-v1-response-size")
            return json.loads(raw)
    except urllib.error.HTTPError as error:
        c.need(False, "metrics-v1-conflict" if error.code in (409, 412) else "metrics-v1-api-failed")
    except (ValueError, OSError):
        c.need(False, "metrics-v1-api-failed")


def identity(service, before):
    import read_metrics_live as live
    meta = service.get("metadata", {})
    c.need(service.get("apiVersion") == "serving.knative.dev/v1" and service.get("kind") == "Service"
           and meta.get("name") == "mypage-api" and meta.get("namespace") in {c.PROJECT, before.get("projectNumber", "")}
           and meta.get("uid") == before.get("uid")
           and str(meta.get("generation")) == str(before.get("generation"))
           and type(meta.get("resourceVersion")) is str and bool(meta["resourceVersion"]), "metrics-v1-identity")
    status = service.get("status", {})
    c.need(status.get("latestReadyRevisionName") == live.ready(before)
           and status.get("latestCreatedRevisionName") == live.ready(before)
           and any(x.get("type") == "Ready" and x.get("status") == "True" for x in status.get("conditions", [])), "metrics-v1-ready")


def configuration(service):
    meta = service.get("metadata", {})
    return {"spec":deepcopy(service.get("spec")), "labels":deepcopy(meta.get("labels", {})),
            "annotations":{k:v for k,v in meta.get("annotations", {}).items() if k not in OUTPUT_ANNOTATIONS}}


def conditional_update(token, before, target):
    import read_metrics_live as live
    original = request(token)
    # The namespace may be a project number. It is explicitly present in the
    # server's already-admitted MYPAGE_PROJECT_NUMBER env, never client input.
    config = before | {"projectNumber":next(x["value"] for x in before["template"]["containers"][0]["env"] if x["name"] == "MYPAGE_PROJECT_NUMBER")}
    identity(original, config)
    current = live.request(token, live.SERVICE_URL)
    c.need(live.ready(current) == live.ready(before) and current["uid"] == before["uid"] and
           current["generation"] == before["generation"] and current["etag"] == before["etag"] and
           live.api_configuration(current) == live.api_configuration(before), "metrics-v1-stale-snapshot")
    payload = deepcopy(original)
    payload.pop("status", None)
    container = r.one(payload["spec"]["template"]["spec"]["containers"], "metrics-v1-container")
    old_env = container.get("env")
    c.need(type(old_env) is list and all(type(x) is dict and type(x.get("name")) is str for x in old_env)
           and len({x["name"] for x in old_env}) == len(old_env), "metrics-v1-env")
    old_by_name = {x["name"]: x for x in old_env if x["name"] != m.METRICS_ENV}
    wanted = m.container(target)["env"]
    c.need(set(old_by_name) == {x["name"] for x in wanted if x["name"] != m.METRICS_ENV}, "metrics-v1-env")
    container["env"] = [({"name":m.METRICS_ENV, "value":"true"} if x["name"] == m.METRICS_ENV else deepcopy(old_by_name[x["name"]])) for x in wanted]
    expected = configuration(payload)
    result = request(token, payload)  # Original resourceVersion; one attempt.
    c.need(result.get("metadata", {}).get("uid") == before["uid"] and
           str(result["metadata"].get("generation")) == str(int(before["generation"]) + 1), "metrics-v1-result")
    deadline = time.monotonic() + 300
    while True:
        after = live.request(token, live.SERVICE_URL)
        if not after.get("reconciling", False):
            live.validate_result(after, before, target)
            break
        c.need(time.monotonic() < deadline, "metrics-v1-timeout")
        time.sleep(2)
    final = request(token)
    c.need(configuration(final) == expected, "metrics-v1-result-config")
    identity(final, after | {"projectNumber":config["projectNumber"]})
    confirm = live.request(token, live.SERVICE_URL)
    c.need(confirm["etag"] == after["etag"] and live.ready(confirm) == live.ready(after), "metrics-v1-superseded")
    return confirm
