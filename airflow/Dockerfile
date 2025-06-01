ARG AIRFLOW_TAG

FROM apache/airflow:$AIRFLOW_TAG

COPY airflow/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
