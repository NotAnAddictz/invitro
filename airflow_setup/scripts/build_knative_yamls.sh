docker build -f ./airflow_setup/workflows/knative_yaml_builder/Dockerfile ./airflow_setup/workflows -t yaml-builder:latest
tmpdir="$(mktemp -d)"
chmod 777 "$tmpdir"
docker run -it -v "$(pwd)"/airflow_setup/workflows/image/airflow-dags:/dag_import -v "$tmpdir":/output -v "$(pwd)"/airflow_setup/workflows/knative_yaml_builder/knative_service_template.yaml:/knative_service_template.yaml yaml-builder:latest
mkdir -p ./airflow_setup/workflows/knative_yamls
for dag_dir in "$tmpdir"/*; do
	cp -r "$dag_dir" ./airflow_setup/workflows/knative_yamls/
done
