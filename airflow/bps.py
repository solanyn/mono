from airflow.decorators import (
    task,
    dag,
)
from airflow.utils.dates import days_ago
import re
import s3fs
from urllib.parse import urlparse
from urllib.request import urlopen
from bs4 import BeautifulSoup
import httpx
import os


NSW_PROPERTY_SALES_INFORMATION_URL = (
    "https://valuation.property.nsw.gov.au/embed/propertySalesInformation"
)


@task(task_id="get_links")
def get_links(**kwargs):
    headers = {
        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
    }
    r = httpx.get(NSW_PROPERTY_SALES_INFORMATION_URL, headers=headers)
    soup = BeautifulSoup(r.text, features="html.parser")
    zip_files = [
        link["href"]
        for link in soup.find_all(href=True)
        if re.search(r"\.zip$", link["href"], re.IGNORECASE)
    ]
    kwargs["ti"].xcom_push(key="zip_files", value=zip_files)


@task(task_id="download_link_to_s3")
def download_link_to_s3(url, **kwargs):
    s3 = s3fs.S3FileSystem(
        key=os.getenv("AWS_ACCESS_KEY_ID"),
        secret=os.getenv("AWS_SECRET_ACCESS_KEY"),
        client_kwargs={"endpoint_url": os.getenv("S3_ENDPOINT_URL")},
    )

    path = urlparse(url).path.lstrip("/")
    uri = f"s3://bps/{path}"
    if s3.exists(uri):
        return

    with urlopen(url) as fd:
        with s3.open(uri, "wb") as f:
            f.write(fd.read())


@dag(
    dag_id="nsw_property_sales_dag",
    schedule_interval="@daily",
    start_date=days_ago(1),
    catchup=False,
)
def dag():
    get_links_task = get_links()

    @task
    def generate_download_tasks(**kwargs):
        ti = kwargs["ti"]
        urls = ti.xcom_pull(key="zip_files", task_ids="get_links")
        tasks = []
        for i, url in enumerate(urls):
            task = download_link_to_s3.override(task_id=f"download_link_to_s3_{i}")(url)
            get_links_task >> task
            tasks.append(task)
        return tasks

    generate_download_tasks_task = generate_download_tasks()

    get_links_task >> generate_download_tasks_task
