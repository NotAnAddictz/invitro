#!/usr/bin/zsh

function cleanup {
	rm -rf airflow_setup/workflows/image/airflow
}

trap cleanup EXIT

cleanup
cp -r airflow airflow_setup/workflows/image/airflow
(cd ./airflow_setup/workflows/image && docker build . -t airflow-worker:latest) || (echo Failed to build airflow-worker:latest && exit 1)
echo Built airflow-worker:latest
docker build . -t airflow:latest || (echo Failed to build airflow:latest && exit 1)
echo Built airflow:latest

docker tag airflow-worker:latest notanaddict/airflow-worker:latest
docker push notanaddict/airflow-worker:latest
docker tag airflow:latest notanaddict/airflow:latest
docker push notanaddict/airflow:latest