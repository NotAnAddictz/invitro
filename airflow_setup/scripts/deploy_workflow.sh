#!/bin/bash

if [[ $# != 1 ]]; then
	echo Usage: deploy_workflow_yamls.sh '<dag_id>'
	echo Applies all yamls in airflow_setup/workflows/dag_id/*.yaml
	exit 1
fi

for yaml_file in airflow_setup/workflows/knative_yamls/"$1"/*.yaml; do
	kn service apply -f "$yaml_file" -n airflow &
done
wait
