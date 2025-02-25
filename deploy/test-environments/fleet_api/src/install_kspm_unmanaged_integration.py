#!/usr/bin/env python
"""
This script installs KSPM unmanaged integration.

The following steps are performed:
1. Create an agent policy.
2. Create a KSPM unmanaged integration.
3. Create a KSPM manifest to be deployed on a host.
"""
import sys
from pathlib import Path
from munch import Munch
import configuration_fleet as cnfg
from api.agent_policy_api import create_agent_policy
from api.package_policy_api import create_kspm_unmanaged_integration
from api.common_api import (
    get_enrollment_token,
    get_fleet_server_host,
    create_kubernetes_manifest,
    get_package_version,
    update_package_version,
)
from loguru import logger
from state_file_manager import state_manager, PolicyState, HostType
from package_policy import (
    load_data,
    version_compatible,
    generate_random_name,
    VERSION_MAP,
)


KSPM_UNMANAGED_EXPECTED_AGENTS = 2
INTEGRATION_NAME = "KSPM Self Managed"
PKG_DEFAULT_VERSION = VERSION_MAP.get("cis_k8s", "")
INTEGRATION_INPUT = {
    "name": generate_random_name("pkg-kspm"),
    "input_name": "cis_k8s",
    "posture": "kspm",
    "deployment": "self_managed",
}
AGENT_INPUT = {
    "name": generate_random_name("kspm-self-managed"),
}


if __name__ == "__main__":
    # pylint: disable=duplicate-code
    package_version = get_package_version(cfg=cnfg.elk_config)
    logger.info(f"Package version: {package_version}")
    if not version_compatible(
        current_version=package_version,
        required_version=PKG_DEFAULT_VERSION,
    ):
        logger.warning(f"{INTEGRATION_NAME} is not supported in version {package_version}")
        sys.exit(0)

    update_package_version(
        cfg=cnfg.elk_config,
        package_name="cloud_security_posture",
        package_version=package_version,
    )

    logger.info(f"Starting installation of {INTEGRATION_NAME} integration.")
    agent_data, package_data = load_data(
        cfg=cnfg.elk_config,
        agent_input=AGENT_INPUT,
        package_input=INTEGRATION_INPUT,
    )

    logger.info("Create agent policy")
    agent_policy_id = create_agent_policy(cfg=cnfg.elk_config, json_policy=agent_data)

    logger.info(f"Create {INTEGRATION_NAME} integration")
    package_policy_id = create_kspm_unmanaged_integration(
        cfg=cnfg.elk_config,
        pkg_policy=package_data,
        agent_policy_id=agent_policy_id,
    )

    state_manager.add_policy(
        PolicyState(
            agent_policy_id,
            package_policy_id,
            KSPM_UNMANAGED_EXPECTED_AGENTS,
            [],
            HostType.KUBERNETES.value,
            INTEGRATION_INPUT["name"],
        ),
    )

    manifest_params = Munch()
    manifest_params.enrollment_token = get_enrollment_token(
        cfg=cnfg.elk_config,
        policy_id=agent_policy_id,
    )

    manifest_params.fleet_url = get_fleet_server_host(cfg=cnfg.elk_config)
    manifest_params.yaml_path = Path(__file__).parent / "kspm_unmanaged.yaml"
    manifest_params.docker_image_override = cnfg.kspm_config.docker_image_override
    logger.info(f"Creating {INTEGRATION_NAME} manifest")
    create_kubernetes_manifest(cfg=cnfg.elk_config, params=manifest_params)
    logger.info(f"Installation of {INTEGRATION_NAME} is done")
