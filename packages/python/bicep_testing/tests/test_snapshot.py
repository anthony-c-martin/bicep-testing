from pathlib import Path

from anthonycmartin.bicep_testing import BicepTestSession, SnapshotMetadata


def test_snapshot_matches_reference_behavior() -> None:
    fixture = Path(__file__).parents[4] / "samples" / "infra" / "main.bicepparam"
    metadata = SnapshotMetadata(
        tenant_id="00000000-0000-0000-0000-000000000000",
        subscription_id="00000000-0000-0000-0000-000000000000",
        resource_group="sample-rg",
        location="eastus",
        deployment_name="sample-deployment",
    )

    with BicepTestSession.create("0.43.1") as session:
        snapshot = session.snapshot(fixture, metadata)

    assert snapshot.diagnostics == ()
    assert len(snapshot.predicted_resources) == 3
    assert {resource.name for resource in snapshot.predicted_resources} == {
        "sampleprimary",
        "samplebackup",
        "samplekv",
    }