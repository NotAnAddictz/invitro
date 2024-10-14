import uuid
from time import sleep, perf_counter

import airflow_client.client
from airflow_client.client.api import config_api, dag_api, dag_run_api
from airflow_client.client.model.dag_run import DAGRun

from airflow_client.client.api import x_com_api
from airflow_client.client.model.x_com_collection import XComCollection
from airflow_client.client.model.error import Error

import json
from contextlib import suppress
# The client must use the authentication and authorization parameters
# in accordance with the API server security policy.
# Examples for each auth method are provided below, use the example that
# satisfies your auth use case.
#
# In case of the basic authentication below, make sure that Airflow is
# configured also with the basic_auth as backend additionally to regular session backend needed
# by the UI. In the `[api]` section of your `airflow.cfg` set:
#
# auth_backend = airflow.api.auth.backend.session,airflow.api.auth.backend.basic_auth
#
# Make sure that your user/name are configured properly - using the user/password that has admin
# privileges in Airflow

# Configure HTTP basic authorization: Basic

# Make sure in the [core] section, the  `load_examples` config is set to True in your airflow.cfg
# or AIRFLOW__CORE__LOAD_EXAMPLES environment variable set to True

# Enter a context with an instance of the API client

def get_dag_list(api_client):
    errors = False

    print('Getting DAG list')
    dag_api_instance = dag_api.DAGApi(api_client)
    try:
        api_response = dag_api_instance.get_dags()
        print(api_response)
    except airflow_client.client.OpenApiException as e:
        print("Exception when calling DagAPI->get_dags: %s\n" % e)
        errors = True
    else:
        print('Getting DAG list successful')


def trigger_dag(api_client, DAG_ID):

    print('Triggering a DAG run: ')
    dag_run_api_instance = dag_run_api.DAGRunApi(api_client)
    try:
        # Create a DAGRun object (no dag_id should be specified because it is read-only property of DAGRun)
        # dag_run id is generated randomly to allow multiple executions of the script
        dag_run = DAGRun(
            dag_run_id='manual_' + uuid.uuid4().hex,
        )
        api_response = dag_run_api_instance.post_dag_run(DAG_ID, dag_run)
        print(f"    dag_id: {api_response['dag_id']}")
        print(f"dag_run_id: {api_response['dag_run_id']}")
        return api_response
    except airflow_client.client.exceptions.OpenApiException as e:
        print("Exception when calling DAGRunAPI->post_dag_run: %s\n" % e)
        errors = True

def get_xcom_values(api_client, dag_id, run_id, task_id):
    extract_id, count_id, sum_id, avg_id = task_id
    api_instance = x_com_api.XComApi(api_client)
    while True:
        try: 
            xcom_return_value_extract = api_instance.get_xcom_entry(dag_id, run_id, extract_id, 'return_value')
            xcom_return_value_count = api_instance.get_xcom_entry(dag_id, run_id, count_id, 'return_value')
            xcom_return_value_sum = api_instance.get_xcom_entry(dag_id, run_id, sum_id, 'return_value')
            xcom_return_value_average = api_instance.get_xcom_entry(dag_id, run_id, avg_id, 'return_value')
        except Exception as e:
            print(f"waiting for result: {e.status}", end="\r")
            sleep(0.1)
        else: 
            got_result_time = perf_counter()
            print(f"\n\nExecution Result: ")
            print(f"  extract: \n    dag_id: {xcom_return_value_extract['dag_id']}\n    task_id: {xcom_return_value_extract['task_id']}\n    value: {xcom_return_value_extract['value']}")
            print(f"  count: \n    dag_id: {xcom_return_value_count['dag_id']}\n    task_id: {xcom_return_value_count['task_id']}\n    value: {xcom_return_value_count['value']}")
            print(f"  sum: \n    dag_id: {xcom_return_value_sum['dag_id']}\n    task_id: {xcom_return_value_sum['task_id']}\n    value: {xcom_return_value_sum['value']}")
            print(f"  average: \n    dag_id: {xcom_return_value_average['dag_id']}\n    task_id: {xcom_return_value_average['task_id']}\n    value: {xcom_return_value_average['value']}\n")
            
            return got_result_time

def main():
    configuration = airflow_client.client.Configuration(
    host="http://localhost:8080/api/v1",
    username='admin',
    password='admin', 
    )
    dag_id = "compute_avg_distributed"
    api_client = airflow_client.client.ApiClient(configuration)
    start_trigger = perf_counter()
    response = trigger_dag(api_client, dag_id)
    finish_trigger = perf_counter()
    
    run_id = response['dag_run_id']
    extract_id = 'extract'
    count_id = 'compute_count'
    sum_id = 'compute_sum'
    avg_id = 'do_avg'
    
    got_result_time = get_xcom_values(api_client, dag_id, run_id, [extract_id, count_id, sum_id, avg_id])
    
    print(f"End-to-end Latency: {round(got_result_time - finish_trigger,2)} second")

    
if __name__ == "__main__":
    main()