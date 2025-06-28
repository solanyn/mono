ARG AIRFLOW_TAG

FROM apache/airflow:$AIRFLOW_TAG

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
