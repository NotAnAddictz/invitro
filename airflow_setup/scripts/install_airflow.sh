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
sudo airflow_setup/scripts/build_knative_yamls.sh
sudo airflow_setup/scripts/update_images.sh
sudo airflow_setup/scripts/setup_airflow.sh