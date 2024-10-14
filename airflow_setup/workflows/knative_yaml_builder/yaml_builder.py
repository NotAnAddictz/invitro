import os
import sys

from airflow.models.dagbag import DagBag

dagbag = DagBag("/dag_import")

with open("/knative_service_template.yaml") as f:
    yaml_template = f.read()

sys.path.append("/dag_import")

for dag_id, dag in dagbag.dags.items():
    os.mkdir(f"/output/{dag_id}")
    for task_id in dag.task_ids:
        escaped_service_name = f'airflow-worker-{dag_id.replace("_", "-")}-{task_id.replace("_", "-")}'
        instantiated_template = yaml_template \
            .replace("__SERVICE_NAME__", escaped_service_name) \
            .replace("__DAGID__", dag_id) \
            .replace("__TASKID__", task_id)
        with open(f"/output/{dag_id}/{dag_id}-{task_id}.yaml", "w") as f:
            f.write(instantiated_template)
