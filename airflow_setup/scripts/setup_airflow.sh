#!/bin/bash

# increase max open files
ulimit -n 1000000

# install helm
curl https://baltocdn.com/helm/signing.asc | gpg --dearmor | sudo tee /usr/share/keyrings/helm.gpg > /dev/null
sudo apt-get install apt-transport-https --yes
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/helm.gpg] https://baltocdn.com/helm/stable/debian/ all main" | sudo tee /etc/apt/sources.list.d/helm-stable-debian.list
sudo apt-get update
sudo apt-get install helm
helm repo add apache-airflow https://airflow.apache.org

# setup volumes
kubectl create namespace airflow
sudo mkdir -p /mnt/data{0..19}
sudo chmod 777 /mnt/data*
kubectl -n airflow apply -f airflow_setup/configs/volumes.yaml

# deploy airflow
helm upgrade -f airflow_setup/configs/values.yaml airflow ./airflow_setup/chart --install --namespace airflow

# provide scheduler access to kubernetes admin interface, this is required to discover knative services
scheduler="$(kubectl -n airflow get pods | grep scheduler | awk '{print $1}')"
kubectl -n airflow exec $scheduler -- mkdir /home/airflow/.kube
kubectl -n airflow cp ~/.kube/config "$scheduler":/home/airflow/.kube/config

while [[ ! $(kubectl -n airflow get pods | grep webserver.*1/1.*Running) ]]; do sleep 1; done