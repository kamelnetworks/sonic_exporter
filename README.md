# sonic_exporter
Prometheus exporter for the SONiC NOS.

The exporter acts as two exporters in one:

 * Data plane: Exposes metrics based on the SONiC State Database
 * Control plane: Implements [`node_exporter`](https://github.com/prometheus/node_exporter/) tuned to work well with SONiC.

## Installation using `sonic-package-manager`

Installation using `sonic-package-manager` requires SONiC 202106 or later.

```shell
# Fetch the latest version
version=$(curl -s https://api.github.com/repos/kamelnetworks/sonic_exporter/releases | jq '.[0].name' -r)

sudo sonic-package-manager install \
  --from-repository "ghcr.io/kamelnetworks/sonic_exporter:${version}" \
  --enable
```

**NOTE**: Due to https://github.com/sonic-net/sonic-buildimage/issues/14805 the above does
not work if you want to download the container in the management VRF. For that case we
release container tarballs that can be imported instead.

```
# Fetch the latest version
version=$(curl -s https://api.github.com/repos/kamelnetworks/sonic_exporter/releases | jq '.[0].name' -r)
# TODO UPDATE
sudo sonic-package-manager install \
  --from-repository "ghcr.io/kamelnetworks/sonic_exporter:${version}" \
  --enable
```

## Configuration

Configuration is done using the SONiC click system.

Example:
```
admin@sonic:~$ sudo config sonic-exporter
Usage: config sonic-exporter [OPTIONS] COMMAND [ARGS]...

  Configure Prometheus exporter for SONiC

Options:
  -h, -?, --help  Show this message and exit.

Commands:
  port  Set the port that the exporter is listening to.
  vrf   Set the VRF that the exporter is listening inside.
admin@sonic:~$ sudo config sonic-exporter port 1234
admin@sonic:~$ sudo config sonic-exporter vrf VrfTest
```
