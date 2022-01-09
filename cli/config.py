import click
import utilities_common.cli as clicommon


@click.group()
def sonic_exporter():
    """Configure Prometheus exporter for SONiC"""
    pass


@sonic_exporter.command('port')
@click.argument('port', required=True, type=int)
@clicommon.pass_db
def set_port(db, port):
    """Set the port that the exporter is listening to."""
    db.cfgdb.mod_entry('SONIC_EXPORTER', 'default', {'port': port})


@sonic_exporter.command('vrf')
@click.argument('vrf', required=True)
@clicommon.pass_db
def set_vrf(db, vrf):
    """Set the VRF that the exporter is listening inside.

    If VRF is set to 'none' the default VRF is used.
    """
    ctx = click.get_current_context()
    if vrf != 'none' and vrf not in db.cfgdb.get_table('VRF'):
        ctx.fail('VRF {} does not exist'.format(vrf))
    if vrf == 'none':
        db.cfgdb.mod_entry('SONIC_EXPORTER', 'default', {'vrf': ''})
    else:
        db.cfgdb.mod_entry('SONIC_EXPORTER', 'default', {'vrf': vrf})


def register(cli):
    cli.add_command(sonic_exporter)
