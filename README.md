# sonic_exporter
Prometheus exporter for the SONiC NOS.

## Installation using `sonic-package-manager`

Installation using `sonic-package-manager` requires SONiC 202106 or later.

```
sudo sonic-package-manager install --from-repository quay.io/kamelnetworks/sonic_exporter:latest
sudo config feature state sonic_exporter enabled
```
