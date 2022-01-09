# sonic_exporter
Prometheus exporter for the SONiC NOS.

## Installation using `sonic-package-manager`

Installation using `sonic-package-manager` requires SONiC 202106 or later.

```shell
# Fetch the latest version
version=$(curl -s https://api.github.com/repos/kamelnetworks/sonic_exporter/releases | jq '.[0].name' -r)

sudo sonic-package-manager install \
  --from-repository "ghcr.io/kamelnetworks/sonic_exporter:${version}" \
  --enable
```
