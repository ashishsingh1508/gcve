from argparse import ArgumentParser
from json import load


def get_hostnames(src, cluster_id):
    hostnames = []
    for dcObj in src.get("vcObjs").get("dcObjs"):
        for clObj in dcObj.get("clObjs"):
            if clObj.get("moid") == cluster_id:
                for hostObj in clObj.get("hostObjs"):
                    hostnames.append(hostObj.get("name"))
                return hostnames


def get_credentials(src, hostnames):
    credentials = []
    for host in src.get("hosts"):
        for hostname in hostnames:
            if host.get("internal_hostname") == hostname.split(".", 1)[0]:
                credentials.append(
                    {
                        "hostname": hostname,
                        "ip_address": host.get("mgmt").get("nat_ip_address"),
                        "username": host.get("esxi_username"),
                        "password": host.get("esxi_password"),
                    }
                )
    return credentials


def generate_commands(command_template, credentials):
    commands = []
    for credential in credentials:
        commands.append(command_template.format(**credential))
    return commands


if __name__ == "__main__":
    parser = ArgumentParser(
        prog="RemoteCommandGenerator",
        description="Generate shell script with remote commands for mummy",
    )

    parser.add_argument("--src", type=str, help="Source file path", required=True)
    parser.add_argument("--cluster_id", type=str, help="Cluster ID", required=True)
    parser.add_argument("--command_template", type=str, help="Command template file path", required=True)
    parser.add_argument("--output", type=str, help="Output file path", default="mummy_remote_commands.sh")

    args = parser.parse_args()

    with open(args.src, "r") as f:
        src = load(f)

    hostnames = get_hostnames(src, args.cluster_id)
    credentials = get_credentials(src, hostnames)

    with open(args.command_template, "r") as f:
        command_template = f.read()

    commands = generate_commands(command_template, credentials)

    with open(args.output, "w") as f:
        f.write("\n\n\n".join(commands))