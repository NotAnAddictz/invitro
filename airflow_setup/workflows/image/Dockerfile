FROM apache/airflow:2.8.0
STOPSIGNAL SIGINT
RUN pip install grpcio grpcio-tools
COPY airflow-dags /opt/airflow/dags
COPY airflow /home/airflow/.local/lib/python3.8/site-packages/airflow
EXPOSE 50000
ENTRYPOINT ["python", "/home/airflow/.local/lib/python3.8/site-packages/airflow/grpc/remote_xcom/grpc_worker_entrypoint.py", "--port", "50000"]
