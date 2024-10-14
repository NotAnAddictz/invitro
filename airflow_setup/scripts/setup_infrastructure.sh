git clone --depth=1 https://github.com/vhive-serverless/vhive.git
cd vhive
mkdir -p /tmp/vhive-logs

./scripts/install_go.sh; source /etc/profile
pushd scripts && go build -o setup_tool && popd

./scripts/setup_tool setup_node stock-only  # this might print errors, ignore them

sudo screen -dmS containerd containerd; sleep 5;
./scripts/setup_tool create_one_node_cluster stock-only
cd ..