service=minectaft-util

systemctl is-enabled $service
if [ $? -eq 0 ]; then
  systemctl stop $service
fi

go build

install -Dm755 $service /usr/local/bin/$service

if [ -d /usr/lib/systemd/system/ ]; then
  unit_dir=/usr/lib/systemd/system
else
  unit_dir=/etc/systemd/system
fi

install -Dm644 systemd/$service.service $unit_dir/$service.service

touch /tmp/grafana_cpu.png
chown nobody:nobody /tmp/grafana_cpu.png

systemctl daemon-reload
systemctl enable $service.service
systemctl start $service.service