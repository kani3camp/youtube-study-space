# GCP Terraform modules

共有できる責務が実際に生まれた時点でmodule化する。

Phase 1では空の抽象化を先に作らない。dev / prod rootの差分を確認し、WIF / IAM、BigQuery、Storage等で同一契約が確認できた単位だけをここへ切り出す。
