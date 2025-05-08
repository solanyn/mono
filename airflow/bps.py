from airflow.decorators import dag, task
from airflow.utils.dates import days_ago
import re
from urllib.parse import urlparse
from urllib.request import urlopen
from bs4 import BeautifulSoup
import httpx
import os
import s3fs

NSW_PROPERTY_SALES_INFORMATION_URL = (
    "https://valuation.property.nsw.gov.au/embed/propertySalesInformation"
)


# Extracted into plain function so it's available at DAG parse time
def fetch_zip_file_links():
    headers = {
        "User-Agent": (
            "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
            "AppleWebKit/537.36 (KHTML, like Gecko) "
            "Chrome/120.0.0.0 Safari/537.36"
        ),
    }
    r = httpx.get(NSW_PROPERTY_SALES_INFORMATION_URL, headers=headers)
    soup = BeautifulSoup(r.text, "html.parser")
    return [
        link["href"]
        for link in soup.find_all(href=True)
        if re.search(r"\.zip$", link["href"], re.IGNORECASE)
    ]


@task
def download_link_to_s3(url: str):
    s3 = s3fs.S3FileSystem(
        key=os.getenv("AWS_ACCESS_KEY_ID"),
        secret=os.getenv("AWS_SECRET_ACCESS_KEY"),
        client_kwargs={"endpoint_url": os.getenv("S3_ENDPOINT_URL")},
    )
    path = urlparse(url).path.lstrip("/")
    uri = f"s3://bps/{path}"
    if s3.exists(uri):
        return f"Skipped {uri}"

    with urlopen(url) as fd:
        with s3.open(uri, "wb") as f:
            f.write(fd.read())
    return f"Uploaded {uri}"


@dag(
    dag_id="nsw_property_sales_dag_v2",
    schedule_interval="@daily",
    start_date=days_ago(1),
    catchup=False,
    tags=["property", "sales"],
)
def nsw_property_sales_dag():
    urls = fetch_zip_file_links()
    download_link_to_s3.expand(url=urls)  # task mapping


dag = nsw_property_sales_dag()
