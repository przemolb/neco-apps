#!/bin/sh

. ./bin/env

# Create GCE instance
$GCLOUD compute instances delete ${INSTANCE_NAME} --zone ${ZONE} --quiet || true
$GCLOUD compute instances create ${INSTANCE_NAME} \
  --zone ${ZONE} \
  --machine-type ${MACHINE_TYPE} \
  --image vmx-enabled \
  --boot-disk-type ${DISK_TYPE} \
  --boot-disk-size ${BOOT_DISK_SIZE} \
  --local-ssd interface=scsi

# Run data center test
for i in $(seq 300); do
  if $GCLOUD compute ssh --zone=${ZONE} cybozu@${INSTANCE_NAME} --command=date 2>/dev/null; then
    break
  fi
  sleep 1
done

cat >run.sh <<EOF
#!/bin/sh -ex
# Run test
GOPATH=\$HOME/go
GO111MODULE=on
export GOPATH GO111MODULE
PATH=/usr/local/go/bin:\$GOPATH/bin:\$PATH
export PATH
git clone https://github.com/${CIRCLE_PROJECT_USERNAME}/${CIRCLE_PROJECT_REPONAME} \$HOME/go/src/github.com/${CIRCLE_PROJECT_USERNAME}/${CIRCLE_PROJECT_REPONAME}
cd \$HOME/go/src/github.com/${CIRCLE_PROJECT_USERNAME}/${CIRCLE_PROJECT_REPONAME}
git checkout -qf ${CIRCLE_SHA1}

cat >/etc/docker/daemon.json << EOF2
{
    "bip": "172.17.0.1/16"
}
EOF2
sudo systemctl restart docker.service

cd test
cp /home/cybozu/account.json ./
make setup
make -f Makefile.kindtest start
make -f Makefile.kindtest COMMIT_ID=${CIRCLE_SHA1} kindtest
EOF
chmod +x run.sh

# Clean old CI files
$GCLOUD compute scp --zone=${ZONE} run.sh .bash_profile account.json cybozu@${INSTANCE_NAME}:
$GCLOUD compute ssh --zone=${ZONE} cybozu@${INSTANCE_NAME} --command="sudo -H /home/cybozu/run.sh"

exit $?
