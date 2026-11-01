# Explicit API ownership preparation

全 enabled services を import しない。private inventory / caller evidence で `Own` と分類し、
Terraform resource の明示 dependency とレビューされた既存 `ENABLED` service だけを選ぶ。
`service_keys=[]` が既定値。`classification` は理由・dependency address を含む private input。
`Platform/External` / `Investigate` / `Do not own` / 未確認 service は選択時に拒否する。

`disable_on_destroy=false` / `disable_dependent_services=false` / `prevent_destroy=true`。
pinned provider の local `deletion_policy` default は import 時に変えない。
これらは API enable の許可ではない。missing/disabled API を Terraform の create で補わず、
import-only plan が create/update/delete/replace を1件でも示せば STOP。

Google provider の Read は project GET と enabled-services list を使用する。
API1件の ownership でも service-level GET-only の CI permission では成立しない。
[実行前 runbook と pinned source](../../docs/runtime-wif-api-ownership.md) を参照。
