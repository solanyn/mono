import boto3
import s3fs

from datalake.config import DatalakeConfig


def get_s3fs(config: DatalakeConfig | None = None) -> s3fs.S3FileSystem:
    config = config or DatalakeConfig()
    return s3fs.S3FileSystem(
        key=config.access_key,
        secret=config.secret_key,
        endpoint_url=config.endpoint,
        client_kwargs={"region_name": config.region},
    )


def get_boto3_client(config: DatalakeConfig | None = None):
    config = config or DatalakeConfig()
    return boto3.client(
        "s3",
        endpoint_url=config.endpoint,
        aws_access_key_id=config.access_key,
        aws_secret_access_key=config.secret_key,
        region_name=config.region,
    )
