"""Pure offline selection contract for import-fidelity tests.

No historical proof, deadline, private staging or execution authority.
"""
from prepare_runtime_ownership import prepare, require

WAVES = {"pool", "provider", "grant", "api", "noop"}
SELECTION_KEYS = {"pool", "provider", "grants", "apis"}

def selection(value, candidate):
    require(type(value) is dict and set(value) == SELECTION_KEYS)
    require(type(value["pool"]) is bool and type(value["provider"]) is bool)
    for key, available in (("grants", candidate["runtime_wif_inventory"]["grants"]),
                           ("apis", candidate["api_classification"])):
        keys = value[key]
        require(type(keys) is list and all(type(k) is str for k in keys))
        require(len(keys) == len(set(keys)) and set(keys) <= set(available))
    require(not value["provider"] or value["pool"])
    require(not value["grants"] or value["provider"])
    require(not value["apis"] or (value["provider"] and
            set(value["grants"]) == set(candidate["runtime_wif_inventory"]["grants"])))
    for service in value["apis"]:
        item = candidate["api_classification"][service]
        require(item["classification"] == "Own" and item["state"] == "ENABLED")
    return value


def validate_selection(packet, *, wave):
    require(wave in WAVES and type(packet) is dict)
    require(set(packet) == {"inventory", "adopted", "selected", "wave"} and packet["wave"] == wave)
    candidate = prepare(packet["inventory"])
    old, new = (selection(packet[k], candidate) for k in ("adopted", "selected"))
    # Preserve all adopted flags and keys; adoption adds one object, noop none.
    require(not old["pool"] or new["pool"])
    require(not old["provider"] or new["provider"])
    require(set(old["grants"]) <= set(new["grants"]) and set(old["apis"]) <= set(new["apis"]))
    delta = {"pool": int(new["pool"]) - int(old["pool"]),
             "provider": int(new["provider"]) - int(old["provider"]),
             "grant": len(set(new["grants"]) - set(old["grants"])),
             "api": len(set(new["apis"]) - set(old["apis"]))}
    require(delta == {key: int(key == wave) for key in delta})
    if wave == "pool":
        require(old == {"pool": False, "provider": False, "grants": [], "apis": []})
    elif wave == "provider":
        require(old["pool"] and not old["grants"] and not old["apis"])
    elif wave == "grant":
        require(old["provider"] and not old["apis"])
    elif wave == "api":
        require(old["provider"] and set(old["grants"]) == set(candidate["runtime_wif_inventory"]["grants"]))
    elif wave == "noop":
        # Every valid nonempty runtime selection includes the pool.
        require(old["pool"])
    candidate.update(own_runtime_wif_pool=new["pool"], own_runtime_wif_provider=new["provider"],
                     runtime_wif_grant_keys=sorted(new["grants"]), owned_api_keys=sorted(new["apis"]))
    return candidate
