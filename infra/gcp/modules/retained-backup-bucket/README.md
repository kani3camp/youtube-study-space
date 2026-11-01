# Retained Firestore export backup bucket

Own bucket infrastructure metadata only. Objects, backup data and IAM stay outside this module. `force_destroy=false` and `prevent_destroy` protect the retained bucket; import must have no create/update/delete/replacement/drift.

PAP is canonical `enforced`. The development PAP repair is an independent, approved operator change before import. Preserve existing UBLA, default holds, requester-pays, absent versioning/lifecycle/website/CORS/encryption override, and seven-day soft delete. Inventory must reject unrepresented metadata before import; do not hide differences with `ignore_changes`.

Location/class/retention are inputs: development ASIA-SOUTHEAST2 / STANDARD / no retention; production ASIA-NORTHEAST2 / COLDLINE / unlocked 93-day retention. Mock plans cover both; this wave instantiates/imports development only. Production Terraform remains disabled.

Provider Read requires `storage.buckets.get` only; no object read/write, IAM, list, create/update/delete or production grant is needed. Additional CI metadata permissions require separate approval. Keep the provider's imported local deletion-policy default to avoid mixing a state-only update into an import-only wave.
