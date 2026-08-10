from pathlib import Path

from anthonycmartin.bicep_testing import BicepTestSession, SnapshotMetadata


def test_snapshot_matches_reference_behavior() -> None:
    fixture = Path(__file__).parents[4] / "samples" / "infra" / "main.bicepparam"
    metadata = SnapshotMetadata(
        tenant_id="ddbe463a-0554-485d-b589-0b17d60cd38b",
        subscription_id="28c9069e-23e8-47d2-b640-00d2e0f09616",
        resource_group="sample-rg",
        location="eastus",
        deployment_name="sample-deployment",
    )

    with BicepTestSession.create("0.46.1") as session:
        snapshot = session.snapshot(fixture, metadata)

    assert snapshot.diagnostics == ()
    assert len(snapshot.predicted_resources) == 3
    assert {resource.name for resource in snapshot.predicted_resources} == {
        "sampleprimary",
        "samplebackup",
        "samplekv",
    }