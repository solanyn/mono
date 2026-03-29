import os
from dataclasses import dataclass


@dataclass(frozen=True)
class DatalakeConfig:
    endpoint: str = os.environ.get(
        "S3_ENDPOINT", "http://garage.storage.svc.cluster.local:3900"
    )
    access_key: str = os.environ.get("S3_ACCESS_KEY", "")
    secret_key: str = os.environ.get("S3_SECRET_KEY", "")
    region: str = os.environ.get("S3_REGION", "us-east-1")
