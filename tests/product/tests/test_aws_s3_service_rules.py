"""
CIS AWS S3 Service rules verification.
This module verifies correctness of retrieved findings by manipulating audit actions
"""
from datetime import datetime, timedelta
from functools import partial
import pytest
from commonlib.utils import get_ES_evaluation, res_identifier

from product.tests.data.aws import aws_s3_test_cases as aws_s3_tc
from product.tests.parameters import register_params, Parameters
from .data.constants import RES_NAME


@pytest.mark.aws_s3_service_rules
def test_aws_s3_service_rules(
    cspm_client,
    cloudbeat_agent,
    rule_tag,
    case_identifier,
    expected,
):
    """
    This data driven test verifies rules and findings return by cloudbeat agent.
    In order to add new cases @pytest.mark.parameterize section shall be updated.
    Setup and teardown actions are defined in data method.
    This test creates verifies that cloudbeat returns correct finding.
    @param rule_tag: Name of rule to be verified.
    @param case_identifier: Resource unique identifier
    @param expected: Result to be found in finding evaluation field.
    @return: None - Test Pass / Fail result is generated.
    """
    # pylint: disable=duplicate-code
    s3_identifier = partial(res_identifier, RES_NAME, case_identifier)

    evaluation = get_ES_evaluation(
        elastic_client=cspm_client,
        timeout=cloudbeat_agent.aws_findings_timeout,
        rule_tag=rule_tag,
        exec_timestamp=datetime.utcnow() - timedelta(minutes=30),
        resource_identifier=s3_identifier,
    )

    assert evaluation is not None, f"No evaluation for rule {rule_tag} could be found"
    assert evaluation == expected, f"Rule {rule_tag} verification failed," f"expected: {expected}, got: {evaluation}"


register_params(
    test_aws_s3_service_rules,
    Parameters(
        ("rule_tag", "case_identifier", "expected"),
        [*aws_s3_tc.cis_aws_s3_cases.values()],
        ids=[*aws_s3_tc.cis_aws_s3_cases.keys()],
    ),
)
