#!/bin/bash
export PATH=$PWD/cmds:$PATH

# Clean up
rm -f lsm.db daemon.log daemon.pid
rm -rf /tmp/mock_systemctl

echo ">>> Building..."
go build -o lsm main.go

echo ">>> Adding Service (nginx)..."
./lsm add --name "nginx" \
  --restart "systemctl restart nginx" \
  --check "! systemctl is-failed nginx" \
  --status "systemctl is-active nginx" \
  --schedule "@every 10s"

echo ">>> Starting Daemon..."
./lsm daemon > daemon.log 2>&1 &
echo $! > daemon.pid
DAEMON_PID=$(cat daemon.pid)

echo ">>> Test 1: Manual Stop (Should NOT restart)"
systemctl start nginx
sleep 2 # Let monitor see it's active
systemctl stop nginx
echo "Stopped nginx. Waiting 15s to ensure NO restart..."
sleep 15
if systemctl is-active nginx; then
  echo "FAIL: Service restarted automatically after manual stop!"
else
  echo "PASS: Service remained stopped."
fi

echo ">>> Test 2: Crash (Should restart)"
# First start it
systemctl start nginx
sleep 2
# Now simulate crash
systemctl fail nginx
echo "Crashed nginx. Waiting 15s for restart..."
sleep 15
if systemctl is-active nginx; then
  echo "PASS: Service restarted after crash."
else
  echo "FAIL: Service did not restart after crash."
fi

echo ">>> Test 3: Scheduler Safe Restart"
# Case A: Service is Stopped. Schedule runs. Should NOT restart.
systemctl stop nginx
echo "Stopped nginx. Waiting 15s for schedule (10s) to trigger..."
sleep 15
if systemctl is-active nginx; then
  echo "FAIL: Scheduler restarted a stopped service!"
else
  echo "PASS: Scheduler respected stopped state."
fi

# Case B: Service is Running. Schedule runs. Should restart.
systemctl start nginx
# We need to see if it actually restarts. Mock restart logs to stdout check?
# Mock 'restart' prints "Restarted nginx".
# We can grep daemon log?
echo "Running nginx. Waiting 15s for schedule to trigger restart..."
sleep 15
# Check daemon log for "Triggered scheduled restart" AND "Successfully restarted"
if grep -q "Triggered scheduled restart for nginx" daemon.log; then
    echo "PASS: Scheduler triggered restart."
else
    echo "FAIL: Scheduler did not trigger restart."
fi

echo ">>> Cleanup"
kill $DAEMON_PID
rm daemon.pid
