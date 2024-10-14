# remove k8s and helm environment set up by previous installation
if pgrep -xf "kubectl port-forward svc/airflow-webserver 8080:8080 --namespace airflow" > /dev/null; then
    pgrep -xf "kubectl port-forward svc/airflow-webserver 8080:8080 --namespace airflow" | xargs kill -9
    echo "Airflow Webserver portforwarding cleared"
fi
helm uninstall -n airflow airflow
kn service delete --all -n airflow
kubectl delete namespace airflow
kubectl delete -f airflow_setup/configs/volumes.yaml
sudo rm -rf /mnt/data*/*

# update knative yamls, rebuild worker image and deploy airflow using helm
./airflow_setup/scripts/build_knative_yamls.sh
./airflow_setup/scripts/update_images.sh
./airflow_setup/scripts/setup_airflow.sh

./airflow_setup/scripts/deploy_workflow.sh compute_avg_distributed

# Get Logs
log_dir=./benchmark/"$(date +%s)"
mkdir -p "$log_dir"
kubectl port-forward svc/airflow-webserver 8080:8080 --namespace airflow 2>&1 >/dev/null &
sleep 3
python ./workflow-gateway/main.py 2>&1 >/dev/null
sleep 3
python ./workflow-gateway/main.py
sleep 3
scheduler="$(kubectl -n airflow get pods | grep scheduler | awk '{print $1}')"
kubectl -n airflow logs "$scheduler" scheduler | grep TIMING > "$log_dir"/log_timing.log
kubectl -n airflow logs "$scheduler" scheduler > "$log_dir"/log_scheduler.log
producer="$(kubectl -n airflow get pods | grep extract | awk '{print $1}')"
kubectl -n airflow logs "$producer" user-container > "$log_dir"/log_producer.log
consumer="$(kubectl -n airflow get pods | grep do-avg | awk '{print $1}')"
kubectl -n airflow logs "$consumer" user-container > "$log_dir"/log_consumer.log